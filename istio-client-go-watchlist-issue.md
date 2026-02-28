# Bug: Fake clientset hangs for 10s per informer with client-go v0.35+

**Affects:** `istio.io/client-go` — fake clientset used in unit tests
**Upstream repo:** https://github.com/istio/client-go
**Discovered while:** bumping `k8s.io/client-go` from v0.34.3 → v0.35.2 in external-dns

---

## Summary

Since `k8s.io/client-go v0.35`, the `WatchListClient` feature gate is **enabled by default**
(it was `Beta=false` up to v1.34, became `Beta=true` from v1.35). When enabled, the reflector
attempts to use the WatchList streaming protocol instead of the legacy List+Watch pattern.
WatchList requires the server to emit a bookmark event to signal the end of the initial event
stream. The `istio.io/client-go` fake clientset does not emit bookmark events, so the reflector
stalls for **10 seconds per informer** before logging a warning and falling back:

```sh
Warning: event bookmark expired
  err="awaiting required bookmark event for initial events stream,
       no events received for 10.001112875s"
```

In a test that creates several informers (Gateway, Service, Ingress) this adds 30+ seconds of
artificial latency, making tests appear to time out.

---

## Root Cause

`k8s.io/client-go v0.35` introduced a lightweight opt-out interface for fake clients:

```go
// util/watchlist/watch_list.go
type unSupportedWatchListSemantics interface {
    IsWatchListSemanticsUnSupported() bool
}
```

The reflector checks for this interface before attempting WatchList
(`tools/cache/reflector.go:301`):

```go
if r.useWatchList && watchlist.DoesClientNotSupportWatchListSemantics(lw) {
    r.useWatchList = false   // gracefully falls back to List+Watch
}
```

`k8s.io/client-go`'s own updated fake `Clientset` (the new `NewClientset()`, not the deprecated
`NewSimpleClientset()`) already implements the opt-out:

```go
// kubernetes/fake/clientset_generated.go
func (c *Clientset) IsWatchListSemanticsUnSupported() bool {
    return true
}
```

`istio.io/client-go v1.29.0`'s fake `Clientset`
(`pkg/clientset/versioned/fake/clientset_generated.gen.go`) is still generated against the
pre-0.35 pattern and has **no such method**, so `DoesClientNotSupportWatchListSemantics` returns
`false`, the reflector assumes WatchList is supported, and the 10-second stall occurs.

---

## Reproducer

Any test that calls `istiofake.NewSimpleClientset()` and passes the result to a function that
creates a `SharedInformerFactory` + `WaitForCacheSync` will hang for 10 s per informer when
`k8s.io/client-go >= v0.35`.

Example from `external-dns` (`source/istio_gateway_test.go`):

```go
fakeIstioClient := istiofake.NewSimpleClientset()
// ...
istioInformerFactory := istioinformers.NewSharedInformerFactory(fakeIstioClient, 0)
gatewayInformer := istioInformerFactory.Networking().V1beta1().Gateways()
// WaitForCacheSync stalls 10 s waiting for a WatchList bookmark
```

---

## Fix

Add `IsWatchListSemanticsUnSupported()` to the fake `Clientset` in
`pkg/clientset/versioned/fake/clientset_generated.gen.go`:

```go
// IsWatchListSemanticsUnSupported informs the reflector that this fake client
// does not support WatchList semantics (no bookmark events are emitted).
// Returning true causes the reflector to fall back to the legacy List+Watch path immediately,
// avoiding a 10-second stall per informer in unit tests.
func (c *Clientset) IsWatchListSemanticsUnSupported() bool {
    return true
}
```

This mirrors what `k8s.io/client-go` did for its own fake clientset in v0.35
(commit context: `kubernetes/fake/clientset_generated.go`).

If `clientset_generated.gen.go` is code-generated, the generator template also needs updating
so the fix survives regeneration.

---

## Workaround (consumer side)

Until the upstream fix is released, consumers can disable the `WatchListClient` feature gate
for the entire test binary via `TestMain`:

```go
// source/main_test.go
package source

import (
    "os"
    "testing"

    clientfeatures "k8s.io/client-go/features"
)

func TestMain(m *testing.M) {
    type featureGatesSetter interface {
        clientfeatures.Gates
        Set(clientfeatures.Feature, bool) error
    }
    if gates, ok := clientfeatures.FeatureGates().(featureGatesSetter); ok {
        _ = gates.Set(clientfeatures.WatchListClient, false)
    }
    os.Exit(m.Run())
}
```

---

## Affected versions

| Package | Version with bug | Fix required in |
|---|---|---|
| `istio.io/client-go` | v1.29.0 (latest at time of writing) | fake `Clientset` |
| `k8s.io/client-go` | trigger: v0.35.0+ (`WatchListClient` default=true) | already fixed in own fake |

## Related

- `k8s.io/client-go` known_features.go: `WatchListClient` default changed to `true` at v1.35
- Same issue will affect any other third-party client-go-based fake that was generated before
  this opt-out interface existed (e.g. ambassador, f5, traefik, kong fake clientsets if they
  also use `SharedInformerFactory` in tests)
