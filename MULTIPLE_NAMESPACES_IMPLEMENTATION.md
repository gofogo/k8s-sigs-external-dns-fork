# Multiple Namespaces Support - Implementation Plan

## Table of Contents

- [Overview](#overview)
- [Options Analysis](#options-analysis)
- [Chosen Approach](#chosen-approach)
- [Current Architecture](#current-architecture)
- [Implementation Plan](#implementation-plan)
- [Code Examples](#code-examples)
- [Testing Strategy](#testing-strategy)
- [RBAC Considerations](#rbac-considerations)
- [Migration Path](#migration-path)
- [Implementation Phases](#implementation-phases)

---

## Overview

This document outlines the implementation plan for adding multiple namespace support to external-dns sources. The goal is to allow external-dns to watch resources in multiple specific namespaces instead of requiring cluster-wide access.

**Target CLI Usage:**

```bash
external-dns \
  --source=service \
  --source=ingress \
  --namespace=team-a \
  --namespace=team-b \
  --namespace=team-c \
  --provider=aws
```

---

## Options Analysis

### Option 1: Array of Namespaces (Gloo Pattern)

Add a `Namespaces []string` field alongside existing `Namespace string`.

**Pros:**

- Already has precedent in `GlooNamespaces` (source/gloo_proxy.go:183)
- Simple, explicit list
- Backward compatible if you keep `Namespace` and treat `Namespaces` as override

**Cons:**

- Requires creating multiple informers OR manual iteration through namespace listers
- Less efficient for many namespaces
- Potential duplication between `Namespace` and `Namespaces` fields

**Implementation:**

```go
// In source.Config
Namespaces []string

// In each source
for _, ns := range namespaces {
    items, err := informer.Lister().ByNamespace(ns).List(selector)
    // merge results
}
```

### Option 2: Namespace Label Selector

Use Kubernetes namespace label selector to dynamically match namespaces.

**Pros:**

- Already used in Gateway API (source/gateway.go:449)
- Dynamic - automatically picks up new namespaces
- Flexible filtering (e.g., `environment=production`)
- More Kubernetes-native

**Cons:**

- Requires NamespaceInformer to watch all namespaces
- More complex implementation
- Slight performance overhead from watching namespace objects

### Option 3: Comma-Separated String

Parse `Namespace` as comma-separated values: `"ns1,ns2,ns3"`.

**Pros:**

- Backward compatible - single namespace still works
- No new config fields needed
- Simple CLI flag handling

**Cons:**

- String parsing required
- Less type-safe
- Doesn't match Go conventions
- No precedent in codebase

### Option 4: Hybrid Approach (CHOSEN)

Combine single namespace and array, similar to how Gateway API handles dual namespaces.

**Pros:**

- Maximum flexibility
- Clear migration path
- Can support both simple and complex use cases
- Type-safe

**Cons:**

- Need clear precedence rules

---

## Chosen Approach

**Option 4: Hybrid with Multiple Informer Factories**

This approach creates one informer factory per namespace, which:

- ✅ **RBAC-scoped**: Only needs permissions for specified namespaces
- ✅ **Follows Gateway API precedent** (gateway.go:141-143)
- ✅ **Clean separation** of concerns
- ✅ **Backward compatible** with single namespace
- ✅ **Type-safe** using `[]string`

**Architecture Decision:**

- Create multiple `kubeinformers.SharedInformerFactory` instances (one per namespace)
- Each factory is scoped via `kubeinformers.WithNamespace(namespace)`
- Aggregate results from all factories in the `Endpoints()` method
- Start and sync all factories independently

---

## Current Architecture

### Source Structure

**Main Interface:** `source/source.go`

```go
type Source interface {
    Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error)
    AddEventHandler(context.Context, func())
}
```

**Configuration:** `source/store.go`

```go
type Config struct {
    Namespace string  // Single namespace field (line 64)
    // ... other fields
}
```

### Current Namespace Patterns

**Pattern A: Standard Single Namespace (Most Common)**
Used by: Service, Ingress, Pod, CRD, Contour, etc.

```go
informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
    kubeClient, 0,
    kubeinformers.WithNamespace(namespace)  // Single namespace
)
```

- Empty string (`""`) means all namespaces
- Non-empty namespace means watch only that namespace

**Pattern B: Multiple Namespaces (Gloo-specific)**
Used by: Gloo Proxy source only

```go
// Config field: GlooNamespaces []string
for _, ns := range gs.glooNamespaces {
    proxyObjects, err := gs.proxyInformer.Lister().ByNamespace(ns).List(labels.Everything())
}
```

- Creates single factory watching ALL namespaces
- Filters at query time
- ❌ Requires cluster-wide RBAC

**Pattern C: Dual Namespace Configuration (Gateway API)**
Used by: Gateway route sources

```go
// Two separate fields:
//   - Namespace - for routes
//   - GatewayNamespace - for gateways
if namespace != "" {
    opts = append(opts, gwinformers.WithNamespace(namespace))
}
```

- Creates separate informer factories if namespaces differ
- Can watch different namespaces for routes vs gateways

### Why NOT Pattern B (Gloo Pattern)?

The Gloo pattern watches ALL namespaces and filters at query time:

- ❌ **RBAC Issue**: Requires cluster-wide list/watch permissions
- ❌ Watches namespaces you don't care about
- ❌ More memory if you only need few namespaces

**Our Approach (Pattern A Extended):**

- Create multiple informer factories (one per namespace)
- Each factory scoped to specific namespace
- Only requires RBAC for specified namespaces

---

## Implementation Plan

### 1. Configuration Changes

#### 1.1 Update `pkg/apis/externaldns/types.go`

**Add new field (around line 50-51):**

```go
type Config struct {
    // ... existing fields

    // Deprecated: Use Namespaces for multi-namespace support
    Namespace  string

    // Target namespaces for source operations (empty means all namespaces)
    Namespaces []string

    // ... other fields
}
```

**Update default config (around line 317):**

```go
Namespace:  "",
Namespaces: []string{},
```

**Add CLI flag binding (around line 545 in `bindFlags`):**

```go
b.StringVar("namespace", "Limit resources queried for endpoints to a specific namespace (default: all namespaces)", defaultConfig.Namespace, &cfg.Namespace)

// NEW: Add this line
b.StringsVar("namespaces", "Limit resources queried for endpoints to specific namespaces; specify multiple times for multiple namespaces (default: all namespaces)", defaultConfig.Namespaces, &cfg.Namespaces)
```

#### 1.2 Update `pkg/apis/externaldns/validation/validation.go`

**Add validation function:**

```go
func validateNamespaceConfig(cfg *externaldns.Config) error {
    // Ensure mutual exclusivity
    if cfg.Namespace != "" && len(cfg.Namespaces) > 0 {
        return fmt.Errorf("--namespace and --namespaces are mutually exclusive; use only one")
    }

    // Normalize: if old --namespace is used, convert to --namespaces
    if cfg.Namespace != "" {
        cfg.Namespaces = []string{cfg.Namespace}
        cfg.Namespace = "" // Clear for consistency
    }

    // Log what we're watching
    if len(cfg.Namespaces) == 0 {
        log.Info("No namespace specified, watching all namespaces (cluster-wide)")
    } else {
        log.Infof("Watching namespaces: %v", cfg.Namespaces)
    }

    return nil
}
```

**Call from main validation function:**

```go
func Validate(cfg *externaldns.Config) error {
    if err := validateNamespaceConfig(cfg); err != nil {
        return err
    }
    // ... rest of validation
}
```

#### 1.3 Update `source/store.go`

**Update Config struct (around line 64):**

```go
type Config struct {
    Namespace  string   // Deprecated: Use Namespaces
    Namespaces []string // Target namespaces for source operations
    // ... rest of fields
}
```

**Update `NewSourceConfig` function (around line 106):**

```go
func NewSourceConfig(cfg *externaldns.Config) *Config {
    labelSelector, _ := labels.Parse(cfg.LabelFilter)

    // Normalize namespace configuration
    namespaces := cfg.Namespaces
    if len(namespaces) == 0 && cfg.Namespace != "" {
        namespaces = []string{cfg.Namespace}
    }

    return &Config{
        Namespace:  cfg.Namespace,  // Keep for backward compat
        Namespaces: namespaces,
        // ... rest of fields
    }
}
```

### 2. Create Shared Helper Functions

#### 2.1 Create NEW file: `source/multi_namespace.go`

This file contains shared helper functions to avoid code duplication across sources.

```go
package source

import (
    "context"
    kubeinformers "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
)

// MultiNamespaceInformerFactories manages multiple informer factories, one per namespace
type MultiNamespaceInformerFactories struct {
    factories  []kubeinformers.SharedInformerFactory
    namespaces []string
}

// NewMultiNamespaceInformerFactories creates informer factories for multiple namespaces
// If namespaces is empty, creates a single cluster-wide factory
func NewMultiNamespaceInformerFactories(
    client kubernetes.Interface,
    namespaces []string,
) *MultiNamespaceInformerFactories {
    if len(namespaces) == 0 {
        // Cluster-wide (all namespaces)
        return &MultiNamespaceInformerFactories{
            factories: []kubeinformers.SharedInformerFactory{
                kubeinformers.NewSharedInformerFactory(client, 0),
            },
            namespaces: []string{""},
        }
    }

    factories := make([]kubeinformers.SharedInformerFactory, len(namespaces))
    for i, ns := range namespaces {
        factories[i] = kubeinformers.NewSharedInformerFactoryWithOptions(
            client, 0,
            kubeinformers.WithNamespace(ns),
        )
    }

    return &MultiNamespaceInformerFactories{
        factories:  factories,
        namespaces: namespaces,
    }
}

// Start starts all informer factories
func (m *MultiNamespaceInformerFactories) Start(stopCh <-chan struct{}) {
    for _, factory := range m.factories {
        factory.Start(stopCh)
    }
}

// WaitForCacheSync waits for all caches to sync
func (m *MultiNamespaceInformerFactories) WaitForCacheSync(ctx context.Context) error {
    for _, factory := range m.factories {
        if err := informers.WaitForCacheSync(ctx, factory); err != nil {
            return err
        }
    }
    return nil
}

// GetFactories returns all informer factories
func (m *MultiNamespaceInformerFactories) GetFactories() []kubeinformers.SharedInformerFactory {
    return m.factories
}

// GetNamespaces returns the namespaces covered by these factories
func (m *MultiNamespaceInformerFactories) GetNamespaces() []string {
    return m.namespaces
}
```

### 3. Update Source Implementations

Each source follows the same pattern. We'll show `service.go` as the reference implementation.

#### 3.1 Update `source/service.go`

**Step 1: Update struct (lines 65-89)**

BEFORE:

```go
type serviceSource struct {
    client                kubernetes.Interface
    namespace             string
    // ... other fields
    serviceInformer                coreinformers.ServiceInformer
    endpointSlicesInformer         discoveryinformers.EndpointSliceInformer
    podInformer                    coreinformers.PodInformer
    nodeInformer                   coreinformers.NodeInformer
    // ...
}
```

AFTER:

```go
type serviceSource struct {
    client                kubernetes.Interface
    namespace             string   // Deprecated, for backward compat
    namespaces            []string // NEW: support multiple namespaces
    // ... other fields

    // Replace single informers with slices
    serviceInformers           []coreinformers.ServiceInformer
    endpointSlicesInformers    []discoveryinformers.EndpointSliceInformer
    podInformers               []coreinformers.PodInformer
    nodeInformer               coreinformers.NodeInformer  // Cluster-scoped, stays single
    // ...
}
```

**Step 2: Update `NewServiceSource` constructor (lines 92-230)**

Key changes:

1. Use `NewMultiNamespaceInformerFactories` helper
2. Create informer slices (one per namespace)
3. Start and sync all factories

```go
func NewServiceSource(
    ctx context.Context,
    kubeClient kubernetes.Interface,
    namespace, annotationFilter, fqdnTemplate string,
    combineFqdnAnnotation bool,
    compatibility string,
    publishInternal, publishHostIP bool,
    alwaysPublishNotReadyAddresses bool,
    serviceTypeFilter []string,
    ignoreHostnameAnnotation bool,
    labelSelector labels.Selector,
    resolveLoadBalancerHostname bool,
    listenEndpointEvents bool,
    exposeInternalIPv6 bool,
    excludeUnschedulable bool,
) (Source, error) {
    tmpl, err := fqdn.ParseTemplate(fqdnTemplate)
    if err != nil {
        return nil, err
    }

    // Determine namespaces to watch
    namespaces := []string{}
    if namespace != "" {
        namespaces = []string{namespace}
    }
    // Empty namespaces = all namespaces (cluster-wide)

    // Create multi-namespace informer factories
    multiFactory := NewMultiNamespaceInformerFactories(kubeClient, namespaces)
    factories := multiFactory.GetFactories()

    // Create informers for each namespace
    serviceInformers := make([]coreinformers.ServiceInformer, len(factories))
    var endpointSlicesInformers []discoveryinformers.EndpointSliceInformer
    var podInformers []coreinformers.PodInformer

    sTypesFilter, err := newServiceTypesFilter(serviceTypeFilter)
    if err != nil {
        return nil, err
    }

    for i, factory := range factories {
        serviceInformers[i] = factory.Core().V1().Services()
        _, _ = serviceInformers[i].Informer().AddEventHandler(informers.DefaultEventHandler())

        if sTypesFilter.isRequired(v1.ServiceTypeNodePort, v1.ServiceTypeClusterIP) {
            esInformer := factory.Discovery().V1().EndpointSlices()
            endpointSlicesInformers = append(endpointSlicesInformers, esInformer)
            _, _ = esInformer.Informer().AddEventHandler(informers.DefaultEventHandler())

            // Add indexer
            err = esInformer.Informer().AddIndexers(cache.Indexers{
                serviceNameIndexKey: func(obj any) ([]string, error) {
                    endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
                    if !ok {
                        return nil, fmt.Errorf("unexpected object type: %T", obj)
                    }
                    serviceName, ok := endpointSlice.Labels[discoveryv1.LabelServiceName]
                    if !ok {
                        return nil, fmt.Errorf("endpointslice %s/%s missing service name label",
                            endpointSlice.Namespace, endpointSlice.Name)
                    }
                    return []string{serviceName}, nil
                },
            })
            if err != nil {
                return nil, err
            }

            podInformer := factory.Core().V1().Pods()
            podInformers = append(podInformers, podInformer)
            _, _ = podInformer.Informer().AddEventHandler(informers.DefaultEventHandler())

            // Add transformer
            _ = podInformer.Informer().SetTransform(func(i any) (any, error) {
                pod, ok := i.(*v1.Pod)
                if !ok {
                    return i, fmt.Errorf("unexpected object type: %T", i)
                }
                pod.ManagedFields = nil
                pod.Status.Conditions = nil
                return pod, nil
            })
        }
    }

    // Node informer stays cluster-scoped (single instance)
    var nodeInformer coreinformers.NodeInformer
    if sTypesFilter.isRequired(v1.ServiceTypeNodePort) {
        // Use cluster-wide factory for nodes
        clusterFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
        nodeInformer = clusterFactory.Core().V1().Nodes()
        _, _ = nodeInformer.Informer().AddEventHandler(informers.DefaultEventHandler())
        clusterFactory.Start(ctx.Done())
    }

    multiFactory.Start(ctx.Done())

    // Wait for all caches to sync
    if err := multiFactory.WaitForCacheSync(ctx); err != nil {
        return nil, err
    }

    return &serviceSource{
        client:                         kubeClient,
        namespace:                      namespace,  // Keep for backward compat
        namespaces:                     multiFactory.GetNamespaces(),
        annotationFilter:               annotationFilter,
        fqdnTemplate:                   tmpl,
        combineFQDNAnnotation:          combineFqdnAnnotation,
        compatibility:                  compatibility,
        publishInternal:                publishInternal,
        publishHostIP:                  publishHostIP,
        alwaysPublishNotReadyAddresses: alwaysPublishNotReadyAddresses,
        serviceTypeFilter:              sTypesFilter,
        ignoreHostnameAnnotation:       ignoreHostnameAnnotation,
        serviceInformers:               serviceInformers,
        endpointSlicesInformers:        endpointSlicesInformers,
        podInformers:                   podInformers,
        nodeInformer:                   nodeInformer,
        labelSelector:                  labelSelector,
        resolveLoadBalancerHostname:    resolveLoadBalancerHostname,
        listenEndpointEvents:           listenEndpointEvents,
        exposeInternalIPv6:             exposeInternalIPv6,
        excludeUnschedulable:           excludeUnschedulable,
    }, nil
}
```

**Step 3: Update `Endpoints` method (lines 232-334)**

BEFORE:

```go
func (sc *serviceSource) Endpoints(_ context.Context) ([]*endpoint.Endpoint, error) {
    services, err := sc.serviceInformer.Lister().Services(sc.namespace).List(sc.labelSelector)
    if err != nil {
        return nil, err
    }
    // ... process services
}
```

AFTER:

```go
func (sc *serviceSource) Endpoints(_ context.Context) ([]*endpoint.Endpoint, error) {
    // Aggregate services from all namespace informers
    var allServices []*v1.Service

    for _, serviceInformer := range sc.serviceInformers {
        // Each informer is namespace-scoped, so List() returns all from that namespace
        services, err := serviceInformer.Lister().List(sc.labelSelector)
        if err != nil {
            return nil, err
        }
        allServices = append(allServices, services...)
    }

    // Rest of the logic remains the same
    services := sc.filterByServiceType(allServices)
    services, err := annotations.Filter(services, sc.annotationFilter)
    if err != nil {
        return nil, err
    }

    endpoints := make([]*endpoint.Endpoint, 0)
    // ... rest of existing logic unchanged
}
```

**Step 4: Update `AddEventHandler` (lines 873-882)**

```go
func (sc *serviceSource) AddEventHandler(_ context.Context, handler func()) {
    log.Debug("Adding event handler for service")

    // Add handler to all service informers
    for _, serviceInformer := range sc.serviceInformers {
        _, _ = serviceInformer.Informer().AddEventHandler(eventHandlerFunc(handler))
    }

    if sc.listenEndpointEvents && sc.serviceTypeFilter.isRequired(v1.ServiceTypeNodePort, v1.ServiceTypeClusterIP) {
        for _, esInformer := range sc.endpointSlicesInformers {
            _, _ = esInformer.Informer().AddEventHandler(eventHandlerFunc(handler))
        }
    }
}
```

**Step 5: Update helper methods**

Update methods that access informers to loop through all informer slices:

- `extractHeadlessEndpoints` (line 337): Search across all `endpointSlicesInformers`
- `pods` method (line 761): Aggregate from all `podInformers`

#### 3.2 Update `source/ingress.go`

Follow the same pattern as service.go:

1. Update struct: `ingressInformers []netinformers.IngressInformer`
2. Update `NewIngressSource`: Use `NewMultiNamespaceInformerFactories`
3. Update `Endpoints`: Aggregate from all `ingressInformers`
4. Update `AddEventHandler`: Register with all informers

```go
type ingressSource struct {
    client               kubernetes.Interface
    namespace            string   // Deprecated
    namespaces           []string // NEW
    annotationFilter     string
    ingressClassNames    []string
    fqdnTemplate         *template.Template
    combineFQDNAnnotation    bool
    ignoreHostnameAnnotation bool
    ingressInformers     []netinformers.IngressInformer  // Changed from single
    ignoreIngressTLSSpec     bool
    ignoreIngressRulesSpec   bool
    labelSelector        labels.Selector
}

func NewIngressSource(/* params */) (Source, error) {
    // ... template parsing and validation

    namespaces := []string{}
    if namespace != "" {
        namespaces = []string{namespace}
    }

    multiFactory := NewMultiNamespaceInformerFactories(kubeClient, namespaces)
    factories := multiFactory.GetFactories()

    ingressInformers := make([]netinformers.IngressInformer, len(factories))
    for i, factory := range factories {
        ingressInformers[i] = factory.Networking().V1().Ingresses()
        _, _ = ingressInformers[i].Informer().AddEventHandler(informers.DefaultEventHandler())
    }

    multiFactory.Start(ctx.Done())
    if err := multiFactory.WaitForCacheSync(ctx); err != nil {
        return nil, err
    }

    return &ingressSource{
        // ... fields
        namespaces:       multiFactory.GetNamespaces(),
        ingressInformers: ingressInformers,
        // ...
    }, nil
}

func (sc *ingressSource) Endpoints(_ context.Context) ([]*endpoint.Endpoint, error) {
    var allIngresses []*networkv1.Ingress

    for _, ingressInformer := range sc.ingressInformers {
        ingresses, err := ingressInformer.Lister().List(sc.labelSelector)
        if err != nil {
            return nil, err
        }
        allIngresses = append(allIngresses, ingresses...)
    }

    // ... rest of logic unchanged
}
```

#### 3.3 Update `source/pod.go`

Same pattern:

```go
type podSource struct {
    client                kubernetes.Interface
    namespace             string   // Deprecated
    namespaces            []string // NEW
    fqdnTemplate          *template.Template
    combineFQDNAnnotation bool

    podInformers          []coreinformers.PodInformer    // Changed to slice
    nodeInformers         []coreinformers.NodeInformer   // Changed to slice
    compatibility         string
    ignoreNonHostNetworkPods bool
    podSourceDomain       string
}

// Constructor and methods follow same pattern as service.go
```

#### 3.4 Update `source/crd.go`

CRD source is special - it uses REST client, not standard informers:

```go
type crdSource struct {
    crdClient        rest.Interface
    namespace        string   // Deprecated
    namespaces       []string // NEW
    crdResource      string
    codec            runtime.ParameterCodec
    annotationFilter string
    labelSelector    labels.Selector
    informer         cache.SharedInformer
}

func (cs *crdSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
    endpoints := []*endpoint.Endpoint{}

    var allResults []*apiv1alpha1.DNSEndpoint

    if len(cs.namespaces) == 0 {
        // All namespaces
        result, err := cs.List(ctx, &metav1.ListOptions{LabelSelector: cs.labelSelector.String()})
        if err != nil {
            return nil, err
        }
        for i := range result.Items {
            allResults = append(allResults, &result.Items[i])
        }
    } else {
        // Specific namespaces
        for _, ns := range cs.namespaces {
            result, err := cs.ListNamespace(ctx, ns, &metav1.ListOptions{
                LabelSelector: cs.labelSelector.String(),
            })
            if err != nil {
                return nil, err
            }
            for i := range result.Items {
                allResults = append(allResults, &result.Items[i])
            }
        }
    }

    // ... rest of processing
}

func (cs *crdSource) ListNamespace(ctx context.Context, namespace string, opts *metav1.ListOptions) (*apiv1alpha1.DNSEndpointList, error) {
    result := &apiv1alpha1.DNSEndpointList{}
    return result, cs.crdClient.Get().
        Namespace(namespace).
        Resource(cs.crdResource).
        VersionedParams(opts, cs.codec).
        Do(ctx).
        Into(result)
}
```

### 4. Other Sources to Update

Apply the same pattern to all remaining sources:

- `source/openshift_route.go`
- `source/istio_gateway.go`
- `source/istio_virtualservice.go`
- `source/gateway_httproute.go`
- `source/gateway_grpcroute.go`
- `source/gateway_tcproute.go`
- `source/gateway_udproute.go`
- `source/gateway_tlsroute.go`
- `source/contour_httpproxy.go`
- `source/f5_virtualserver.go`
- `source/ambassador_host.go`
- `source/kong_tcpingress.go`
- `source/traefik_proxy.go`

**Note:** `source/node.go` is cluster-scoped and doesn't use namespaces - minimal/no changes needed.

---

## Code Examples

### Example 1: Single Namespace (Backward Compatible)

```bash
# Old way (still works)
external-dns --source=service --namespace=production --provider=aws

# New way (equivalent)
external-dns --source=service --namespaces=production --provider=aws
```

### Example 2: Multiple Namespaces

```bash
external-dns \
  --source=service \
  --source=ingress \
  --namespaces=team-a \
  --namespaces=team-b \
  --namespaces=team-c \
  --provider=aws \
  --txt-owner-id=my-cluster
```

### Example 3: All Namespaces (Cluster-wide)

```bash
# No namespace specified = all namespaces
external-dns --source=service --provider=aws
```

### Example 4: Helm Values

```yaml
namespaces:
  - team-a
  - team-b
  - team-c

sources:
  - service
  - ingress

provider: aws
```

---

## Testing Strategy

### Unit Tests

For each source, add multi-namespace test cases.

**Example: `source/service_test.go`**

```go
func TestServiceSourceMultipleNamespaces(t *testing.T) {
    t.Parallel()

    fakeClient := fake.NewClientset()

    // Create services in different namespaces
    svc1 := &v1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: "ns1",
            Name:      "service1",
            Annotations: map[string]string{
                "external-dns.alpha.kubernetes.io/hostname": "svc1.example.com",
            },
        },
        Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
        Status: v1.ServiceStatus{
            LoadBalancer: v1.LoadBalancerStatus{
                Ingress: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}},
            },
        },
    }

    svc2 := &v1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: "ns2",
            Name:      "service2",
            Annotations: map[string]string{
                "external-dns.alpha.kubernetes.io/hostname": "svc2.example.com",
            },
        },
        Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
        Status: v1.ServiceStatus{
            LoadBalancer: v1.LoadBalancerStatus{
                Ingress: []v1.LoadBalancerIngress{{IP: "5.6.7.8"}},
            },
        },
    }

    _, _ = fakeClient.CoreV1().Services("ns1").Create(context.Background(), svc1, metav1.CreateOptions{})
    _, _ = fakeClient.CoreV1().Services("ns2").Create(context.Background(), svc2, metav1.CreateOptions{})

    // Create source watching both namespaces
    source, err := NewServiceSource(
        context.TODO(),
        fakeClient,
        "", // empty namespace triggers multi-namespace logic
        "",
        "",
        false,
        "",
        false,
        false,
        false,
        []string{},
        false,
        labels.Everything(),
        false,
        false,
        false,
        false,
    )
    require.NoError(t, err)

    endpoints, err := source.Endpoints(context.Background())
    require.NoError(t, err)

    // Should have endpoints from both namespaces
    assert.Len(t, endpoints, 2)

    dnsNames := []string{}
    for _, ep := range endpoints {
        dnsNames = append(dnsNames, ep.DNSName)
    }
    assert.ElementsMatch(t, []string{"svc1.example.com", "svc2.example.com"}, dnsNames)
}

func TestServiceSourceBackwardCompatibility(t *testing.T) {
    t.Parallel()

    fakeClient := fake.NewClientset()

    svc := &v1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: "default",
            Name:      "service1",
            Annotations: map[string]string{
                "external-dns.alpha.kubernetes.io/hostname": "svc.example.com",
            },
        },
        Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
        Status: v1.ServiceStatus{
            LoadBalancer: v1.LoadBalancerStatus{
                Ingress: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}},
            },
        },
    }

    _, _ = fakeClient.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})

    // Test old single-namespace API
    source, err := NewServiceSource(
        context.TODO(),
        fakeClient,
        "default",  // old namespace param
        "",
        "",
        false,
        "",
        false,
        false,
        false,
        []string{},
        false,
        labels.Everything(),
        false,
        false,
        false,
        false,
    )
    require.NoError(t, err)

    endpoints, err := source.Endpoints(context.Background())
    require.NoError(t, err)

    assert.Len(t, endpoints, 1)
    assert.Equal(t, "svc.example.com", endpoints[0].DNSName)
}

func TestServiceSourceNamespaceIsolation(t *testing.T) {
    t.Parallel()

    fakeClient := fake.NewClientset()

    // Create services in three namespaces
    for i, ns := range []string{"ns1", "ns2", "ns3"} {
        svc := &v1.Service{
            ObjectMeta: metav1.ObjectMeta{
                Namespace: ns,
                Name:      "service",
                Annotations: map[string]string{
                    "external-dns.alpha.kubernetes.io/hostname": fmt.Sprintf("svc.%s.com", ns),
                },
            },
            Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
            Status: v1.ServiceStatus{
                LoadBalancer: v1.LoadBalancerStatus{
                    Ingress: []v1.LoadBalancerIngress{{IP: fmt.Sprintf("1.2.3.%d", i)}},
                },
            },
        }
        _, _ = fakeClient.CoreV1().Services(ns).Create(context.Background(), svc, metav1.CreateOptions{})
    }

    // Watch only ns1 and ns2 (not ns3)
    source, err := NewServiceSource(/* params with namespaces=["ns1", "ns2"] */)
    require.NoError(t, err)

    endpoints, err := source.Endpoints(context.Background())
    require.NoError(t, err)

    // Should only have 2 endpoints (ns1, ns2), NOT ns3
    assert.Len(t, endpoints, 2)

    dnsNames := []string{}
    for _, ep := range endpoints {
        dnsNames = append(dnsNames, ep.DNSName)
    }
    assert.ElementsMatch(t, []string{"svc.ns1.com", "svc.ns2.com"}, dnsNames)
    assert.NotContains(t, dnsNames, "svc.ns3.com")
}
```

### Integration Tests

Create `source/multi_namespace_integration_test.go`:

```go
func TestMultiNamespaceIntegration(t *testing.T) {
    t.Parallel()

    client := fake.NewClientset()

    // Setup: Create resources in multiple namespaces
    namespaces := []string{"team-a", "team-b", "team-c"}
    for _, ns := range namespaces {
        // Create namespace
        _, _ = client.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
            ObjectMeta: metav1.ObjectMeta{Name: ns},
        }, metav1.CreateOptions{})

        // Create service
        svc := &v1.Service{
            ObjectMeta: metav1.ObjectMeta{
                Namespace: ns,
                Name:      "app",
                Annotations: map[string]string{
                    "external-dns.alpha.kubernetes.io/hostname": fmt.Sprintf("app.%s.example.com", ns),
                },
            },
            Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
            Status: v1.ServiceStatus{
                LoadBalancer: v1.LoadBalancerStatus{
                    Ingress: []v1.LoadBalancerIngress{{Hostname: fmt.Sprintf("lb.%s.cloud.com", ns)}},
                },
            },
        }
        _, _ = client.CoreV1().Services(ns).Create(context.Background(), svc, metav1.CreateOptions{})
    }

    // Test: Only watch team-a and team-b
    source, err := NewServiceSource(/* namespaces: ["team-a", "team-b"] */)
    require.NoError(t, err)

    endpoints, err := source.Endpoints(context.Background())
    require.NoError(t, err)

    assert.Len(t, endpoints, 2)

    dnsNames := make(map[string]bool)
    for _, ep := range endpoints {
        dnsNames[ep.DNSName] = true
    }

    assert.True(t, dnsNames["app.team-a.example.com"])
    assert.True(t, dnsNames["app.team-b.example.com"])
    assert.False(t, dnsNames["app.team-c.example.com"])
}
```

### Test Coverage Requirements

- ✅ Multiple namespaces work correctly
- ✅ Backward compatibility with single namespace
- ✅ Empty namespace (all namespaces) works
- ✅ Namespace isolation (only watch specified namespaces)
- ✅ Event handlers registered for all informers
- ✅ Cache sync for all factories
- ✅ Error handling for invalid namespace configs

---

## RBAC Considerations

### Current RBAC (Cluster-wide)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "pods", "nodes"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "watch", "list"]
```

### New RBAC (Namespace-scoped)

With multiple namespace support, users can use namespace-scoped Roles:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: external-dns
  namespace: team-a
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "pods"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "watch", "list"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: external-dns
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: external-dns
subjects:
- kind: ServiceAccount
  name: external-dns
  namespace: external-dns-system

---
# Repeat Role + RoleBinding for team-b, team-c, etc.
```

### Limitations

**Nodes are cluster-scoped:**

- NodePort services require ClusterRole for node access
- Cannot use namespace-scoped Role for NodePort services

**Workaround:**
Create minimal ClusterRole for nodes only:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns-nodes
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]

---
# Namespace-scoped Roles for services, ingresses, etc.
```

---

## Migration Path

### Phase 1: Current Release (v1.0)

**What changes:**

- Both `--namespace` and `--namespaces` work
- `--namespace` internally converts to `--namespaces` with single element
- No breaking changes

**User experience:**

```bash
# Old way (still works)
external-dns --source=service --namespace=default

# New way (recommended)
external-dns --source=service --namespaces=default

# Multi-namespace (new feature)
external-dns --source=service --namespaces=ns1 --namespaces=ns2
```

**Documentation:**

- Mark `--namespace` as deprecated
- Add migration guide
- Update all examples to use `--namespaces`

### Phase 2: Deprecation Period (6-12 months)

**What changes:**

- Add deprecation warnings when `--namespace` is used
- All documentation uses `--namespaces`
- Release notes highlight deprecation

**User experience:**

```bash
external-dns --source=service --namespace=default
# Warning: --namespace is deprecated, use --namespaces instead
```

### Phase 3: Future Release (v2.0+)

**What changes:**

- Remove `--namespace` CLI flag
- Keep `Namespace` field in Config struct (for API compatibility) but mark as internal/deprecated
- Clean up old code paths

**User experience:**

```bash
external-dns --source=service --namespace=default
# Error: unknown flag --namespace, use --namespaces
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1)

**Tasks:**

1. Update `pkg/apis/externaldns/types.go`
   - Add `Namespaces []string` field
   - Add CLI flag binding
2. Create `pkg/apis/externaldns/validation/validation.go`
   - Add `validateNamespaceConfig()`
   - Add mutual exclusivity check
3. Update `source/store.go`
   - Add `Namespaces` to Config struct
   - Update `NewSourceConfig()`
4. Create `source/multi_namespace.go`
   - Implement `MultiNamespaceInformerFactories`

**Deliverables:**

- Configuration infrastructure in place
- Helper functions ready
- Unit tests for validation logic

### Phase 2: Core Sources (Week 2)

**Tasks:**

1. Update `source/service.go`
   - Change to multiple informers
   - Update constructor
   - Update Endpoints method
   - Update event handlers
2. Update `source/ingress.go`
   - Follow service.go pattern
3. Add unit tests
   - Multi-namespace tests for service
   - Multi-namespace tests for ingress
   - Backward compatibility tests

**Deliverables:**

- Service and Ingress sources support multiple namespaces
- Comprehensive test coverage
- Reference implementation for other sources

### Phase 3: Remaining Sources (Week 3)

**Tasks:**

1. Update `source/pod.go`
2. Update `source/crd.go` (special case - REST client)
3. Update Gateway API sources
   - `gateway_httproute.go`
   - `gateway_grpcroute.go`
   - `gateway_tcproute.go`
   - `gateway_tlsroute.go`
   - `gateway_udproute.go`
4. Update Istio sources
   - `istio_gateway.go`
   - `istio_virtualservice.go`
5. Update other sources
   - `openshift_route.go`
   - `contour_httpproxy.go`
   - `f5_virtualserver.go`
   - `ambassador_host.go`
   - `kong_tcpingress.go`
   - `traefik_proxy.go`

**Deliverables:**

- All sources support multiple namespaces
- Unit tests for each source

### Phase 4: Testing & Documentation (Week 4)

**Tasks:**

1. Create integration tests
   - Multi-namespace integration test
   - RBAC scenario tests
2. Update documentation
   - Create `docs/tutorials/multi-namespace.md`
   - Update README with examples
   - Update Helm chart documentation
3. Update RBAC examples
   - Namespace-scoped Role examples
   - Migration guide from ClusterRole to Role
4. Update Helm chart (if applicable)
   - Support `namespaces` value
   - Update RBAC templates

**Deliverables:**

- Comprehensive documentation
- Integration test suite
- RBAC examples
- Migration guide

---

## Critical Files Summary

| File | Changes | Priority |
|------|---------|----------|
| `pkg/apis/externaldns/types.go` | Add `Namespaces []string` field and CLI flag | Critical |
| `pkg/apis/externaldns/validation/validation.go` | Add namespace validation | Critical |
| `source/store.go` | Update Config struct and NewSourceConfig() | Critical |
| `source/multi_namespace.go` | **NEW FILE** - Helper functions | Critical |
| `source/service.go` | Reference implementation | High |
| `source/ingress.go` | Core source | High |
| `source/pod.go` | Core source | High |
| `source/crd.go` | Special case (REST client) | High |
| Gateway API sources | Multiple files | Medium |
| Istio sources | Multiple files | Medium |
| Other sources | Multiple files | Medium |
| Test files | All `*_test.go` files | High |
| Documentation | `docs/tutorials/multi-namespace.md` | Medium |

---

## Benefits Summary

✅ **RBAC-scoped**: Namespace-specific Roles instead of cluster-wide ClusterRole
✅ **Backward compatible**: Old `--namespace` flag still works
✅ **Memory efficient**: Only caches specified namespaces
✅ **Follows existing patterns**: Uses Gateway API precedent
✅ **Type-safe**: Uses `[]string` instead of parsing strings
✅ **Clear migration path**: Gradual deprecation over 6-12 months
✅ **Comprehensive testing**: Unit and integration tests
✅ **Well-documented**: Migration guide and RBAC examples

---

## Risk Mitigation

### Memory Impact

**Risk:** Multiple informer factories increase memory usage

**Mitigation:**

- Each factory maintains separate caches
- Typical overhead: ~1MB per namespace per source type
- For 10 namespaces with 100 services = ~10MB additional memory
- Document memory requirements in release notes
- Add metrics to monitor cache sizes

### API Server Load

**Risk:** Multiple watchers increase API server load

**Mitigation:**

- Kubernetes API server handles multiple watchers efficiently
- Same load as running N separate external-dns instances
- Informers use efficient watch protocol
- No more load than current cluster-wide watch for small namespace counts

### Backward Compatibility

**Risk:** Breaking existing deployments

**Mitigation:**

- Keep `--namespace` flag working
- Automatic conversion to new format
- Deprecation warnings with timeline
- Extensive backward compatibility tests
- Clear migration documentation

### Testing Complexity

**Risk:** Increased test matrix

**Mitigation:**

- Shared test helper functions
- Table-driven tests for multi-namespace scenarios
- Integration tests for common use cases
- Automated CI/CD testing

---

## Next Steps

1. **Review this plan** - Get team/maintainer approval
2. **Set up development environment** - Fork, branch, etc.
3. **Start Phase 1** - Configuration infrastructure
4. **Implement Phase 2** - Core sources (service, ingress)
5. **Continue with Phases 3-4** - Remaining sources and testing
6. **Create PR** - Submit for review with comprehensive tests and docs

---

## Questions & Clarifications

### Q: Should we support namespace label selectors?

**A:** Not in initial implementation. Start with explicit list, add label selector in future release if needed.

### Q: What about dynamic namespace addition/removal?

**A:** Current approach doesn't support dynamic namespace changes at runtime. Requires restart. Could be enhanced in future release with namespace watcher.

### Q: Performance with 100+ namespaces?

**A:** Each namespace gets its own informer cache. For 100 namespaces × 5 sources = 500 caches. Memory intensive but functional. Recommend using label selectors for this scale (future enhancement).

### Q: What about CRDs that are cluster-scoped?

**A:** Cluster-scoped CRDs won't benefit from namespace filtering. Document this limitation. They'll continue to use cluster-wide watch.

---

**Document Version:** 1.0
**Last Updated:** 2026-01-03
**Status:** Draft - Pending Approval
