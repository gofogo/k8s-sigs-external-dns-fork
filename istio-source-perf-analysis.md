# Istio Source Performance Analysis

Analysis of `source/istio_gateway.go` and `source/istio_virtualservice.go`.

## Findings

### 1. VirtualService: Gateway lookup and target fetch repeated per host (high impact)

**Files:** `source/istio_virtualservice.go`

`endpointsFromVirtualService` calls `targetsFromVirtualService(ctx, vService, host)` once per host in
`vService.Spec.Hosts`. Inside that function, for each gateway reference:

- `getGateway` → `gatewayInformer.Lister().Gateways(ns).Get(name)` (cache lookup)
- `targetsFromGateway` → `EndpointTargetsFromServices` → `svcInformer.Lister().Services(ns).List(labels.Everything())` (full cache scan)

Neither call depends on the host — only `virtualServiceBindsToGateway` does. With 5 hosts × 3
gateways this is **15 redundant lookups instead of 3**.

The same problem affects `endpointsFromTemplate`, which also calls `targetsFromVirtualService` per
templated hostname.

**Fix:** extract a `fetchGatewayTargets` helper that resolves each gateway reference exactly once per
VirtualService call, returning the gateway objects and their targets as parallel slices. Slim down
`targetsFromVirtualService` to accept that pre-fetched data:

```go
func (sc *virtualServiceSource) fetchGatewayTargets(
    ctx context.Context,
    vService *networkingv1.VirtualService,
) ([]*networkingv1.Gateway, []endpoint.Targets, error) {
    gateways := make([]*networkingv1.Gateway, 0, len(vService.Spec.Gateways))
    targets := make([]endpoint.Targets, 0, len(vService.Spec.Gateways))
    for _, gatewayStr := range vService.Spec.Gateways {
        gw, err := sc.getGateway(ctx, gatewayStr, vService)
        if err != nil {
            return nil, nil, err
        }
        if gw == nil {
            continue
        }
        tgs, err := sc.targetsFromGateway(gw)
        if err != nil {
            return nil, nil, err
        }
        gateways = append(gateways, gw)
        targets = append(targets, tgs)
    }
    return gateways, targets, nil
}

func targetsFromVirtualService(
    gateways []*networkingv1.Gateway,
    gwTargets []endpoint.Targets,
    vService *networkingv1.VirtualService,
    vsHost string,
) []string {
    var targets []string
    for i, gw := range gateways {
        if !virtualServiceBindsToGateway(vService, gw, vsHost) {
            continue
        }
        for _, t := range gwTargets[i] {
            if !slices.Contains(targets, t) {
                targets = append(targets, t)
            }
        }
    }
    return targets
}
```

`endpointsFromVirtualService` calls `fetchGatewayTargets` once at the top (skipped when annotation
targets are present) and passes the result to every `targetsFromVirtualService` call.

---

### 2. Gateway: `targetsFromGateway` called twice when FQDN template fires (medium impact)

**File:** `source/istio_gateway.go`

In `Endpoints`, `endpointsFromGateway` is called up to twice per gateway — once for spec hostnames
and once inside the FQDN template closure — each time calling `targetsFromGateway` which scans the
service cache.

```go
// current — two calls when combineFQDNAnnotation=true or spec hostnames are empty
gwEndpoints, err := sc.endpointsFromGateway(gwHostnames, gateway)
...
gwEndpoints, err = fqdn.CombineWithTemplatedEndpoints(
    gwEndpoints, sc.fqdnTemplate, sc.combineFQDNAnnotation,
    func() ([]*endpoint.Endpoint, error) {
        hostnames, _ := fqdn.ExecTemplate(sc.fqdnTemplate, gateway)
        return sc.endpointsFromGateway(hostnames, gateway)  // ← second targetsFromGateway call
    },
)
```

**Fix:** hoist `targetsFromGateway` out of `endpointsFromGateway`, compute once per gateway, and
pass the result into both calls via a renamed `endpointsFromGatewayWithTargets`:

```go
targets, err := sc.targetsFromGateway(gateway)
if err != nil {
    return nil, err
}

gwEndpoints, err := sc.endpointsFromGatewayWithTargets(gwHostnames, targets, gateway)
...
gwEndpoints, err = fqdn.CombineWithTemplatedEndpoints(
    gwEndpoints, sc.fqdnTemplate, sc.combineFQDNAnnotation,
    func() ([]*endpoint.Endpoint, error) {
        hostnames, err := fqdn.ExecTemplate(sc.fqdnTemplate, gateway)
        if err != nil {
            return nil, err
        }
        return sc.endpointsFromGatewayWithTargets(hostnames, targets, gateway)
    },
)
```

---

### 3. `AddEventHandler` ignores Service and Ingress changes (correctness gap)

**Files:** `source/istio_gateway.go:196`, `source/istio_virtualservice.go:195`

Both sources only register the caller-supplied handler for Gateway / VirtualService events. If a
backing Service gains or loses a LoadBalancer IP, or a referenced Ingress changes, DNS records are
not refreshed until the Gateway or VirtualService object itself is also updated.

**Fix:** register the same handler on `serviceInformer` and `ingressInformer` in `AddEventHandler`:

```go
func (sc *gatewaySource) AddEventHandler(_ context.Context, handler func()) {
    informers.MustAddEventHandler(sc.gatewayInformer.Informer(), eventHandlerFunc(handler))
    informers.MustAddEventHandler(sc.serviceInformer.Informer(), eventHandlerFunc(handler))
    informers.MustAddEventHandler(sc.ingressInformer.Informer(), eventHandlerFunc(handler))
}
```

---

### 4. Duplicate informer factories when both sources are active (architectural)

**Files:** `source/istio_gateway.go:86`, `source/istio_virtualservice.go:88`

Each source constructs its own `kubeinformers.NewSharedInformerFactory`, resulting in separate
watches and in-memory caches for Services and Ingresses when both the gateway and virtualservice
sources are enabled simultaneously. Memory and API-server watch connections are doubled for those
resource types.

**Fix:** accept a shared `kubeinformers.SharedInformerFactory` (and a shared
`istioinformers.SharedInformerFactory`) as constructor parameters rather than creating them
internally. The caller (source pipeline / wrapper) owns the factory lifecycle and passes it to both
sources, so the underlying caches and watches are shared.

This is a broader refactor that touches the source construction pipeline but aligns with how other
sources in the codebase manage shared factories.

---

## Rejected / Not Worth Implementing

### Service cache index for selector matching

`EndpointTargetsFromServices` calls `svcInformer.Lister().Services(namespace).List(labels.Everything())`
and filters with `MatchesServiceSelector` in Go. The lister is an in-memory cache — no network
call is made. Because `MatchesServiceSelector` checks that the gateway selector is a **subset** of
each service's `spec.selector`, an index key cannot express this relationship without intersecting
multiple index results per query. For the typical service counts per namespace (tens, not hundreds),
the in-memory linear scan is negligible. The complexity of a multi-key intersection index is not
justified.

### `slices.Contains` deduplication in `targetsFromVirtualService`

Replacing `slices.Contains` with a `map[string]struct{}` seen-set only helps when the target list
grows large. In practice a VirtualService has 1–3 gateways each with 1–3 load balancer targets, so
the slice at dedup time has fewer than 10 elements — well below the crossover where map allocation
pays off.
