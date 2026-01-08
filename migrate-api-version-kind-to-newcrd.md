# Migration Plan: REST Client to Controller-Runtime Client

## Overview

Migrate from manual REST client (`NewCRDClientForAPIVersionKind`) to controller-runtime client with **TWO parallel implementations** for testing and comparison, similar to the status updater pattern. Both approaches will be switchable via environment variables with clear removal comments.

## User Requirements

1. ✅ Implement BOTH options with removal comments
2. ✅ Migrate both status updaters (pkg/crd and controller)
3. ✅ No fallback - complete migration (but structured for easy removal)
4. ✅ Similar pattern to existing status updater implementations

## Architecture: 2x2 Testing Matrix

**Two Dimensions:**

### Dimension 1: Status Updater Location (existing)
- **Option 1 (pkg-crd)**: `pkg/crd/status_updater.go` - Service layer in pkg/crd
- **Option 2 (controller)**: `controller/dnsendpoint_status.go` - Controller-owned status manager

### Dimension 2: Client Implementation (new)
- **REST**: Manual REST client (existing) - `pkg/crd/dnsendpoint_client.go`
- **Controller-Runtime**: controller-runtime client (new) - `pkg/crd/dnsendpoint_client_ctrlruntime.go`

### Testing Combinations

```
1. STATUS_UPDATER_IMPL=pkg-crd + CLIENT_IMPL=rest (DEFAULT)
   → Option 1 with REST client (existing baseline)

2. STATUS_UPDATER_IMPL=pkg-crd + CLIENT_IMPL=controller-runtime
   → Option 1 with controller-runtime backed DNSEndpointClient interface

3. STATUS_UPDATER_IMPL=controller + CLIENT_IMPL=rest
   → Option 2 with REST client via DNSEndpointClient interface

4. STATUS_UPDATER_IMPL=controller + CLIENT_IMPL=controller-runtime
   → Option 2 with direct client.Client usage (no interface)
```

## Implementation Strategy

### Option 1: Controller-Runtime Backed Interface

**Keep DNSEndpointClient interface**, create new implementation backed by controller-runtime:
- Interface: `DNSEndpointClient` (unchanged)
- New Implementation: `ctrlRuntimeDNSEndpointClient`
- Wraps `client.Client` from controller-runtime
- Drop-in replacement for REST implementation
- Minimal changes to existing code

### Option 2: Direct client.Client Usage

**Bypass interface**, use controller-runtime client directly:
- No DNSEndpointClient interface
- Direct `client.Client` usage in status managers
- New file: `controller/dnsendpoint_status_ctrlruntime.go`
- More idiomatic controller-runtime style

## Files to Create

### 1. Controller-Runtime Client Factory
**Path:** `pkg/crd/client_factory_ctrlruntime.go`

Creates controller-runtime client with DNSEndpoint types registered:
```go
func NewControllerRuntimeClient(kubeConfig, apiServerURL, namespace string) (client.Client, error)
```

### 2. Controller-Runtime DNSEndpointClient (Option 1)
**Path:** `pkg/crd/dnsendpoint_client_ctrlruntime.go`

Implements DNSEndpointClient interface using controller-runtime:
```go
type ctrlRuntimeDNSEndpointClient struct {
    client    client.Client
    namespace string
}

// Implements: Get(), List(), UpdateStatus(), Watch()
```

### 3. Direct Controller-Runtime Status Manager (Option 2)
**Path:** `controller/dnsendpoint_status_ctrlruntime.go`

Status manager using client.Client directly (no interface):
```go
type DNSEndpointStatusManagerCtrlRuntime struct {
    client client.Client
}

func (m *DNSEndpointStatusManagerCtrlRuntime) UpdateStatus(ctx, namespace, name, success, message) error
```

### 4. Test Files
- `pkg/crd/dnsendpoint_client_ctrlruntime_test.go`
- `controller/dnsendpoint_status_ctrlruntime_test.go`

## Files to Modify

### controller/execute.go (lines 434-565)

Update both `registerStatusUpdateCallbacksOption1` and `registerStatusUpdateCallbacksOption2`:

```go
func registerStatusUpdateCallbacksOption1(ctx, ctrl, cfg) {
    useCtrlRuntime := os.Getenv("CLIENT_IMPL") == "controller-runtime"

    if useCtrlRuntime {
        // NEW: Controller-runtime backed DNSEndpointClient
        ctrlClient, _ := crd.NewControllerRuntimeClient(cfg.KubeConfig, cfg.APIServerURL, cfg.Namespace)
        dnsEndpointClient := crd.NewDNSEndpointClientCtrlRuntime(ctrlClient, cfg.Namespace)
        statusUpdater := crd.NewDNSEndpointStatusUpdater(dnsEndpointClient)
        // ... register callback
    } else {
        // EXISTING: REST client
        // ... existing code
    }
}

func registerStatusUpdateCallbacksOption2(ctx, ctrl, cfg) {
    useCtrlRuntime := os.Getenv("CLIENT_IMPL") == "controller-runtime"

    if useCtrlRuntime {
        // NEW: Direct client.Client usage
        ctrlClient, _ := crd.NewControllerRuntimeClient(cfg.KubeConfig, cfg.APIServerURL, cfg.Namespace)
        statusManager := NewDNSEndpointStatusManagerCtrlRuntime(ctrlClient)
        // ... register callback
    } else {
        // EXISTING: REST client via DNSEndpointClient interface
        // ... existing code
    }
}
```

## Implementation Details

### List Options Conversion

Convert `metav1.ListOptions` to `client.ListOption`:
```go
func (c *ctrlRuntimeDNSEndpointClient) List(ctx context.Context, opts *metav1.ListOptions) (*apiv1alpha1.DNSEndpointList, error) {
    listOpts := []client.ListOption{}

    if c.namespace != "" {
        listOpts = append(listOpts, client.InNamespace(c.namespace))
    }

    if opts != nil && opts.LabelSelector != "" {
        selector, _ := labels.Parse(opts.LabelSelector)
        listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
    }

    dnsEndpointList := &apiv1alpha1.DNSEndpointList{}
    err := c.client.List(ctx, dnsEndpointList, listOpts...)
    return dnsEndpointList, err
}
```

### Status Subresource Update

Controller-runtime uses `.Status().Update()`:
```go
func (c *ctrlRuntimeDNSEndpointClient) UpdateStatus(ctx context.Context, dnsEndpoint *apiv1alpha1.DNSEndpoint) (*apiv1alpha1.DNSEndpoint, error) {
    err := c.client.Status().Update(ctx, dnsEndpoint)
    return dnsEndpoint, err
}
```

### Watch() Implementation

**Initial approach:** Return "not implemented" error:
```go
func (c *ctrlRuntimeDNSEndpointClient) Watch(ctx context.Context, opts *metav1.ListOptions) (watch.Interface, error) {
    return nil, fmt.Errorf("Watch not supported with controller-runtime client, use informer from source/crd.go")
}
```

**Rationale:** source/crd.go already creates its own informer (lines 71-82), so DNSEndpointClient.Watch() may not be actively used.

**Future enhancement:** Implement using controller-runtime's watch capabilities if needed.

### Scheme Setup

Reuse existing scheme registration from `apis/v1alpha1/groupversion_info.go`:
```go
scheme := runtime.NewScheme()
if err := apiv1alpha1.AddToScheme(scheme); err != nil {
    return nil, err
}
```

## Testing Strategy

### Unit Tests

Use `fake.NewClientBuilder()` for controller-runtime fake client:
```go
func TestCtrlRuntimeClient_Get(t *testing.T) {
    scheme := runtime.NewScheme()
    _ = apiv1alpha1.AddToScheme(scheme)

    dnsEndpoint := &apiv1alpha1.DNSEndpoint{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test", Namespace: "default",
        },
    }

    fakeClient := fake.NewClientBuilder().
        WithScheme(scheme).
        WithObjects(dnsEndpoint).
        Build()

    client := crd.NewDNSEndpointClientCtrlRuntime(fakeClient, "default")
    result, err := client.Get(context.Background(), "default", "test")
    require.NoError(t, err)
    require.Equal(t, "test", result.Name)
}
```

### Integration Tests

Test all 4 combinations:
```bash
# Baseline (existing)
export STATUS_UPDATER_IMPL=pkg-crd
export CLIENT_IMPL=rest
./external-dns --source=crd --provider=inmemory --once

# Option 1 + controller-runtime
export STATUS_UPDATER_IMPL=pkg-crd
export CLIENT_IMPL=controller-runtime
./external-dns --source=crd --provider=inmemory --once

# Option 2 + REST
export STATUS_UPDATER_IMPL=controller
export CLIENT_IMPL=rest
./external-dns --source=crd --provider=inmemory --once

# Option 2 + controller-runtime
export STATUS_UPDATER_IMPL=controller
export CLIENT_IMPL=controller-runtime
./external-dns --source=crd --provider=inmemory --once
```

## Comments for Removal

### File-Level Comments

Add to top of each new file:
```go
// OPTION 1 IMPLEMENTATION - Controller-Runtime Backed DNSEndpointClient
//
// This implements DNSEndpointClient interface using controller-runtime client.
// Maintains same interface as REST implementation for compatibility.
//
// TO REMOVE THIS OPTION AND KEEP REST CLIENT:
// 1. Delete this file (pkg/crd/dnsendpoint_client_ctrlruntime.go)
// 2. Delete pkg/crd/client_factory_ctrlruntime.go
// 3. Remove CLIENT_IMPL=controller-runtime from execute.go:registerStatusUpdateCallbacksOption1
//
// TO REMOVE REST CLIENT AND KEEP THIS OPTION:
// 1. Delete pkg/crd/dnsendpoint_client.go (REST implementation)
// 2. Delete pkg/crd/client_factory.go (REST factory)
// 3. Remove CLIENT_IMPL=rest from execute.go:registerStatusUpdateCallbacksOption1
// 4. Rename this file to dnsendpoint_client.go
// 5. Remove environment variable checks
```

### execute.go Comments

Add before `registerStatusUpdateCallbacks`:
```go
// REFACTORING NOTE: Dual Implementation - 2x2 Matrix
//
// DIMENSION 1 - Status Updater Location (STATUS_UPDATER_IMPL):
//   Option 1 (pkg-crd):     pkg/crd.StatusUpdater
//   Option 2 (controller):  controller.DNSEndpointStatusManager
//
// DIMENSION 2 - Client Type (CLIENT_IMPL):
//   rest:                k8s.io/client-go REST client
//   controller-runtime:  sigs.k8s.io/controller-runtime client
//
// TESTING MATRIX: 4 combinations
//   1. pkg-crd + rest (DEFAULT, baseline)
//   2. pkg-crd + controller-runtime
//   3. controller + rest
//   4. controller + controller-runtime
//
// TO FINALIZE:
//   A. Choose status updater location (Option 1 or 2)
//   B. Choose client type (REST or controller-runtime)
//   C. Remove environment variables
//   D. Delete unused code per file-level comments
```

## Implementation Sequence

### Phase 1: Option 1 + Controller-Runtime ✅
1. Create `pkg/crd/client_factory_ctrlruntime.go`
2. Create `pkg/crd/dnsendpoint_client_ctrlruntime.go`
3. Update `controller/execute.go` - registerStatusUpdateCallbacksOption1
4. Add unit tests in `pkg/crd/dnsendpoint_client_ctrlruntime_test.go`
5. Test: `STATUS_UPDATER_IMPL=pkg-crd CLIENT_IMPL=controller-runtime`

### Phase 2: Option 2 + Controller-Runtime ✅
1. Create `controller/dnsendpoint_status_ctrlruntime.go`
2. Update `controller/execute.go` - registerStatusUpdateCallbacksOption2
3. Add unit tests in `controller/dnsendpoint_status_ctrlruntime_test.go`
4. Test: `STATUS_UPDATER_IMPL=controller CLIENT_IMPL=controller-runtime`

### Phase 3: Integration & Documentation ✅
1. Test all 4 combinations in matrix
2. Verify status updates work in real cluster
3. Compare behavior and performance
4. Update STATUS-UPDATER-TESTING-GUIDE.md with CLIENT_IMPL dimension

### Phase 4: Future Cleanup (After Decision)
1. Choose winning combination
2. Remove environment variable switches
3. Delete unused files per comments
4. Update documentation to reflect single implementation

## Critical Files

1. **pkg/crd/dnsendpoint_client_ctrlruntime.go** (NEW) - Core Option 1 implementation
2. **controller/execute.go** (MODIFY lines 434-565) - Orchestration with CLIENT_IMPL support
3. **pkg/crd/client_factory_ctrlruntime.go** (NEW) - Client creation
4. **controller/dnsendpoint_status_ctrlruntime.go** (NEW) - Option 2 direct usage
5. **apis/v1alpha1/groupversion_info.go** (REFERENCE) - Scheme registration

## Benefits

### Controller-Runtime Advantages
- ✅ Simpler: No manual discovery, scheme setup, or serializer config
- ✅ Type-safe: Uses `client.Object` interface
- ✅ Modern: Industry standard for Kubernetes operators
- ✅ Better errors: Clearer error messages
- ✅ Status subresource: `client.Status().Update()` is cleaner
- ✅ Already included: v0.22.4 in go.mod

### Dual Implementation Advantages
- ✅ Risk mitigation: Easy rollback if issues found
- ✅ Performance comparison: Test both in production
- ✅ Learning: Team can evaluate both approaches
- ✅ Clean removal: Clear comments guide cleanup

## Trade-offs

- ⚠️ More code to maintain temporarily (until decision made)
- ⚠️ Watch() not implemented initially (defer if not needed)
- ⚠️ Environment variables needed for switching
- ⚠️ Testing matrix larger (4 combinations)

## Success Criteria

1. ✅ All 4 combinations build successfully
2. ✅ All 4 combinations pass tests
3. ✅ Status updates work in all combinations
4. ✅ No performance degradation with controller-runtime
5. ✅ Clear removal path documented for unused options
