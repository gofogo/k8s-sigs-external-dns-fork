# Source Package Structure Options

Exploratory document for reorganizing `source/` into a 3-level hierarchy:
`source/<group>/<source-type>/`

Each source type becomes its own Go package, enabling `node.New`, `service.New`, etc.
in the factory registry.

---

## Option 1 — By API maturity / origin

```
source/
├── core/                  # kubernetes.io APIs
│   ├── node/
│   ├── pod/
│   ├── service/
│   └── ingress/
├── extension/             # k8s SIG-owned extension APIs
│   ├── gateway/
│   │   ├── httproute/
│   │   ├── grpcroute/
│   │   ├── tcproute/
│   │   ├── tlsroute/
│   │   └── udproute/
│   ├── crd/
│   └── unstructured/
├── vendor/                # third-party CRDs
│   ├── istio/
│   ├── ambassador/
│   ├── contour/
│   ├── traefik/
│   ├── gloo/
│   ├── kong/
│   ├── skipper/
│   ├── openshift/
│   └── f5/
└── special/
    ├── connector/
    └── fake/
```

**Intuition:** Who owns the API spec? kubernetes.io core → k8s SIG extension → third-party vendor.

---

## Option 2 — By traffic layer (L4/L7/mesh)

```
source/
├── infra/                 # node-level, cluster-level
│   ├── node/
│   └── pod/
├── ingress/               # L7 HTTP routing
│   ├── service/
│   ├── ingress/
│   ├── contour/
│   ├── traefik/
│   ├── kong/
│   └── skipper/
├── gateway/               # Gateway API (L4+L7)
│   ├── httproute/
│   ├── grpcroute/
│   ├── tcproute/
│   ├── tlsroute/
│   └── udproute/
├── mesh/                  # service mesh control planes
│   ├── istio/
│   ├── ambassador/
│   └── gloo/
└── platform/              # cloud/vendor platforms + meta
    ├── openshift/
    ├── f5/
    ├── crd/
    ├── unstructured/
    ├── connector/
    └── fake/
```

**Intuition:** Where in the network stack does this resource live?
infra → L7 ingress → Gateway API → service mesh → platform.

---

## Option 3 — By operator persona / who configures it

```
source/
├── cluster/               # cluster-admin owns these
│   ├── node/
│   ├── pod/
│   └── openshift/
├── app/                   # app-team owns these (annotates their own resources)
│   ├── service/
│   ├── ingress/
│   ├── httproute/
│   ├── grpcroute/
│   ├── tcproute/
│   ├── tlsroute/
│   └── udproute/
├── controller/            # ingress/mesh controller owns the DNS-relevant object
│   ├── contour/
│   ├── traefik/
│   ├── kong/
│   ├── skipper/
│   ├── istio/
│   ├── ambassador/
│   ├── gloo/
│   └── f5/
└── meta/                  # sources that aggregate or synthesize
    ├── crd/
    ├── unstructured/
    ├── connector/
    └── fake/
```

**Intuition:** Who annotates/manages the resource that carries DNS info?
cluster-admin → app team → ingress/mesh controller → synthetic/meta.

---

## Option 4 — By Kubernetes client needed

```
source/
├── typed/                 # uses typed kube client only
│   ├── node/
│   ├── pod/
│   ├── service/
│   ├── ingress/
│   └── openshift/
├── dynamic/               # uses dynamic.Interface (CRD-based)
│   ├── gateway/
│   │   ├── httproute/
│   │   ├── grpcroute/
│   │   ├── tcproute/
│   │   ├── tlsroute/
│   │   └── udproute/
│   ├── contour/
│   ├── traefik/
│   ├── ambassador/
│   ├── gloo/
│   ├── kong/
│   ├── f5/
│   └── unstructured/
├── external/              # own HTTP client / no kube client
│   ├── skipper/
│   └── connector/
├── istio/                 # dedicated istio client
│   ├── gateway/
│   └── virtualservice/
└── synthetic/
    ├── crd/
    └── fake/
```

**Intuition:** What Go client does constructing this source require?
typed kube → dynamic → external HTTP → istio-specific → no client.

---

## Option 5 — By source "shape" (how DNS records are derived)

```
source/
├── resource/              # DNS from k8s resource annotations/status
│   ├── node/
│   ├── pod/
│   ├── service/
│   └── ingress/
├── route/                 # DNS from routing rules (hostnames in spec)
│   ├── httproute/
│   ├── grpcroute/
│   ├── tcproute/
│   ├── tlsroute/
│   ├── udproute/
│   ├── contour/
│   ├── traefik/
│   ├── istio/
│   ├── ambassador/
│   ├── gloo/
│   ├── kong/
│   └── skipper/
├── endpoint/              # DNS from explicit endpoint declarations
│   ├── crd/
│   ├── unstructured/
│   └── connector/
└── platform/              # platform-specific routing objects
    ├── openshift/
    ├── f5/
    └── fake/
```

**Intuition:** How does the source derive DNS records?
resource annotations/status → routing rule hostnames → explicit endpoint declarations → platform-specific.

---

## Comparison

| Option | L2 concept | L2 count | Intuition |
|--------|-----------|----------|-----------|
| 1 — maturity | Who owns the API spec? | 4 | kubernetes.io core → SIG extension → vendor |
| 2 — traffic layer | Where in the stack? | 5 | infra → ingress → mesh → platform |
| 3 — persona | Who annotates/manages it? | 4 | cluster-admin → app-team → controller |
| 4 — client | What Go client does it need? | 5 | typed → dynamic → external → istio |
| 5 — shape | How does DNS get derived? | 4 | resource annotations → routing rules → explicit endpoints |

## Common properties across all options

- ~20 leaf packages total regardless of grouping
- Factory registry uses `node.New`, `service.New`, `httproute.New` — package name carries context
- `source.go`, `store.go`, `empty.go`, `utils.go` stay at `source/` root (imported by all)
- Existing sub-packages (`factory/`, `annotations/`, `fqdn/`, `informers/`, `types/`, `wrappers/`) unchanged
