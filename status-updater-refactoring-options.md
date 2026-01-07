# Status Updater Refactoring Options

## Problem with Current Design

### Issue: `crdSource.UpdateDNSEndpointStatus()` Violates SRP

**Current state**: `crdSource` has two responsibilities:
1. **Reading**: Implements `Source` interface - discovers and provides DNS endpoints
2. **Writing**: Updates DNSEndpoint status after sync

### Symptoms of Poor Design

```go
// crdSource has mixed responsibilities
type crdSource struct {
    client           crd.DNSEndpointClient
    annotationFilter string
    labelSelector    labels.Selector
    informer         cache.SharedInformer
}

// Source interface method (READ)
func (cs *crdSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error)

// NOT part of Source interface (WRITE) ❌
func (cs *crdSource) UpdateDNSEndpointStatus(ctx context.Context, namespace, name string, success bool, message string) error
```

**Problems**:

1. **Two Separate Instances Created**
   - Instance #1 uses `Endpoints()` but never `UpdateDNSEndpointStatus()`
   - Instance #2 uses `UpdateDNSEndpointStatus()` but never `Endpoints()`
   - Clear sign of wrong abstraction

2. **Mixed Concerns**
   - "Source" implies reading/discovering
   - Status updates are write operations
   - Violates single responsibility

3. **Interface Pollution**
   - Method not part of `Source` interface
   - Added only for callback mechanism
   - Type assertion needed: `crdSource.(dnsEndpointStatusUpdater)`

4. **Testing Confusion**
   - Need to mock/test both reading and writing in same type
   - Can't test status updates without full source setup

## Refactoring Options

---

## Option 1: Dedicated Status Updater in pkg/crd ⭐ RECOMMENDED

Create a separate status updater that uses the same `DNSEndpointClient`.

### Implementation

**File: `pkg/crd/status_updater.go`**

```go
package crd

import (
    "context"
    "fmt"

    log "github.com/sirupsen/logrus"
    apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// StatusUpdater handles DNSEndpoint status updates
type StatusUpdater interface {
    UpdateDNSEndpointStatus(ctx context.Context, namespace, name string, success bool, message string) error
}

type dnsEndpointStatusUpdater struct {
    client DNSEndpointClient
}

// NewDNSEndpointStatusUpdater creates a status updater for DNSEndpoint CRDs
func NewDNSEndpointStatusUpdater(client DNSEndpointClient) StatusUpdater {
    return &dnsEndpointStatusUpdater{
        client: client,
    }
}

// UpdateDNSEndpointStatus updates the status of a single DNSEndpoint
func (u *dnsEndpointStatusUpdater) UpdateDNSEndpointStatus(ctx context.Context, namespace, name string, success bool, message string) error {
    // Get current DNSEndpoint
    dnsEndpoint, err := u.client.Get(ctx, namespace, name)
    if err != nil {
        return fmt.Errorf("failed to get DNSEndpoint: %w", err)
    }

    // Update status conditions
    if success {
        apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    } else {
        apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    }

    // Update status subresource
    _, err = u.client.UpdateStatus(ctx, dnsEndpoint)
    if err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }

    log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v", namespace, name, success)
    return nil
}
```

**File: `controller/execute.go`** (modified)

```go
func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    kubeClient, err := getKubeClient(cfg)
    if err != nil {
        log.Warnf("Could not create Kubernetes client for status updates: %v", err)
        return
    }

    // Create REST client
    restClient, _, err := crd.NewCRDClientForAPIVersionKind(
        kubeClient,
        cfg.KubeConfig,
        cfg.APIServerURL,
        cfg.CRDSourceAPIVersion,
        cfg.CRDSourceKind,
    )
    if err != nil {
        log.Warnf("Could not create CRD client for status updates: %v", err)
        return
    }

    // Create DNSEndpoint client
    dnsEndpointClient := crd.NewDNSEndpointClient(
        restClient,
        cfg.Namespace,
        cfg.CRDSourceKind,
        metav1.ParameterCodec,
    )

    // Create status updater (NEW - clean separation)
    statusUpdater := crd.NewDNSEndpointStatusUpdater(dnsEndpointClient)

    // Register callback
    callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
        dnsEndpoints := extractDNSEndpointsFromChanges(changes)
        log.Debugf("Updating status for %d DNSEndpoint(s)", len(dnsEndpoints))

        for key, ref := range dnsEndpoints {
            err := statusUpdater.UpdateDNSEndpointStatus(ctx, ref.namespace, ref.name, success, message)
            if err != nil {
                log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
            }
        }
    }

    ctrl.RegisterStatusUpdateCallback(callback)
    log.Info("Registered DNSEndpoint status update callback")
}
```

**File: `source/crd.go`** (modified)

```go
// REMOVE UpdateDNSEndpointStatus method entirely
// crdSource now only implements Source interface
```

### Pros
✅ Clean separation: `crdSource` = reading, `StatusUpdater` = writing
✅ Single responsibility for each type
✅ No need for dual crdSource instances
✅ `StatusUpdater` is reusable in other contexts
✅ Clear package organization in `pkg/crd`
✅ Easy to test independently

### Cons
❌ Need to create helper to get KubeClient in execute.go
❌ Slightly more code in execute.go (but clearer)

---

## Option 2: Status Updates in Controller Package

Move status update logic directly to controller package.

### Implementation

**File: `controller/dnsendpoint_status.go`**

```go
package controller

import (
    "context"
    "fmt"

    log "github.com/sirupsen/logrus"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
    "sigs.k8s.io/external-dns/pkg/crd"
)

// DNSEndpointStatusManager manages status updates for DNSEndpoint CRDs
type DNSEndpointStatusManager struct {
    client crd.DNSEndpointClient
}

// NewDNSEndpointStatusManager creates a status manager
func NewDNSEndpointStatusManager(client crd.DNSEndpointClient) *DNSEndpointStatusManager {
    return &DNSEndpointStatusManager{
        client: client,
    }
}

// UpdateStatus updates the status of a single DNSEndpoint
func (m *DNSEndpointStatusManager) UpdateStatus(ctx context.Context, namespace, name string, success bool, message string) error {
    dnsEndpoint, err := m.client.Get(ctx, namespace, name)
    if err != nil {
        return fmt.Errorf("failed to get DNSEndpoint: %w", err)
    }

    if success {
        apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    } else {
        apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    }

    _, err = m.client.UpdateStatus(ctx, dnsEndpoint)
    if err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }

    log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v", namespace, name, success)
    return nil
}
```

**File: `controller/execute.go`** (modified)

```go
func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    // ... create DNSEndpointClient ...

    // Create status manager in controller package
    statusManager := NewDNSEndpointStatusManager(dnsEndpointClient)

    callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
        dnsEndpoints := extractDNSEndpointsFromChanges(changes)

        for key, ref := range dnsEndpoints {
            err := statusManager.UpdateStatus(ctx, ref.namespace, ref.name, success, message)
            if err != nil {
                log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
            }
        }
    }

    ctrl.RegisterStatusUpdateCallback(callback)
}
```

### Pros
✅ Status updates owned by controller (who orchestrates them)
✅ No dependency on source package for status updates
✅ `crdSource` stays pure (only reading)
✅ Clear: controller package controls all status logic

### Cons
❌ Controller package depends on `pkg/crd`
❌ Less reusable (tied to controller context)
❌ Mixes controller orchestration with CRD-specific logic

---

## Option 3: Inline Helper Function in execute.go

Skip creating a type, just use a helper function.

### Implementation

**File: `controller/execute.go`**

```go
func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    // ... create DNSEndpointClient ...

    // Helper function for status updates
    updateDNSEndpointStatus := func(ctx context.Context, client crd.DNSEndpointClient, namespace, name string, success bool, message string) error {
        dnsEndpoint, err := client.Get(ctx, namespace, name)
        if err != nil {
            return fmt.Errorf("failed to get DNSEndpoint: %w", err)
        }

        if success {
            apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
        } else {
            apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
        }

        _, err = client.UpdateStatus(ctx, dnsEndpoint)
        return err
    }

    // Register callback using helper
    callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
        dnsEndpoints := extractDNSEndpointsFromChanges(changes)

        for key, ref := range dnsEndpoints {
            err := updateDNSEndpointStatus(ctx, dnsEndpointClient, ref.namespace, ref.name, success, message)
            if err != nil {
                log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
            }
        }
    }

    ctrl.RegisterStatusUpdateCallback(callback)
}
```

### Pros
✅ Simplest approach
✅ No new types or files
✅ Clear that it's a one-off utility
✅ `crdSource` stays clean

### Cons
❌ Not reusable
❌ Harder to test in isolation
❌ Helper function in middle of larger function

---

## Option 4: Batch Status Updater with Plan Awareness

Create a status updater that works with plan.Changes directly.

### Implementation

**File: `pkg/crd/batch_status_updater.go`**

```go
package crd

import (
    "context"
    "fmt"

    log "github.com/sirupsen/logrus"
    apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
    "sigs.k8s.io/external-dns/endpoint"
    "sigs.k8s.io/external-dns/plan"
)

// BatchStatusUpdater updates DNSEndpoint status for multiple endpoints at once
type BatchStatusUpdater interface {
    UpdateFromChanges(ctx context.Context, changes *plan.Changes, success bool, message string) error
}

type batchStatusUpdater struct {
    client DNSEndpointClient
}

// NewBatchStatusUpdater creates a batch status updater
func NewBatchStatusUpdater(client DNSEndpointClient) BatchStatusUpdater {
    return &batchStatusUpdater{client: client}
}

// UpdateFromChanges extracts DNSEndpoint refs from changes and updates their status
func (u *batchStatusUpdater) UpdateFromChanges(ctx context.Context, changes *plan.Changes, success bool, message string) error {
    // Extract unique DNSEndpoint references
    refs := extractDNSEndpointRefs(changes)

    log.Debugf("Updating status for %d DNSEndpoint(s)", len(refs))

    var firstErr error
    for key, ref := range refs {
        if err := u.updateOne(ctx, ref.namespace, ref.name, success, message); err != nil {
            log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
            if firstErr == nil {
                firstErr = err
            }
        }
    }

    return firstErr
}

func (u *batchStatusUpdater) updateOne(ctx context.Context, namespace, name string, success bool, message string) error {
    dnsEndpoint, err := u.client.Get(ctx, namespace, name)
    if err != nil {
        return fmt.Errorf("failed to get DNSEndpoint: %w", err)
    }

    if success {
        apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    } else {
        apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    }

    _, err = u.client.UpdateStatus(ctx, dnsEndpoint)
    if err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }

    log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v", namespace, name, success)
    return nil
}

type dnsEndpointRef struct {
    namespace string
    name      string
}

func extractDNSEndpointRefs(changes *plan.Changes) map[string]dnsEndpointRef {
    refs := make(map[string]dnsEndpointRef)

    addEndpoint := func(ep *endpoint.Endpoint) {
        if ep == nil {
            return
        }
        ref := ep.RefObject()
        if ref == nil || ref.Kind != "DNSEndpoint" {
            return
        }
        key := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
        if _, exists := refs[key]; !exists {
            refs[key] = dnsEndpointRef{
                namespace: ref.Namespace,
                name:      ref.Name,
            }
        }
    }

    for _, ep := range changes.Create {
        addEndpoint(ep)
    }
    for _, ep := range changes.UpdateOld {
        addEndpoint(ep)
    }
    for _, ep := range changes.UpdateNew {
        addEndpoint(ep)
    }
    for _, ep := range changes.Delete {
        addEndpoint(ep)
    }

    return refs
}
```

**File: `controller/execute.go`** (simplified)

```go
func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    // ... create DNSEndpointClient ...

    // Create batch status updater
    batchUpdater := crd.NewBatchStatusUpdater(dnsEndpointClient)

    // Super simple callback - just delegate
    callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
        if err := batchUpdater.UpdateFromChanges(ctx, changes, success, message); err != nil {
            log.Warnf("Some DNSEndpoint status updates failed: %v", err)
        }
    }

    ctrl.RegisterStatusUpdateCallback(callback)
    log.Info("Registered DNSEndpoint status update callback")
}
```

### Pros
✅ Encapsulates all batch update logic
✅ Moves `extractDNSEndpointsFromChanges` out of execute.go
✅ Clean callback registration
✅ Easy to test (mock DNSEndpointClient)
✅ Single place for all batch status update logic

### Cons
❌ `pkg/crd` depends on `plan` package
❌ More complex than Option 1
❌ Ties CRD package to external-dns domain concepts

---

## Comparison Matrix

| Aspect | Option 1: Status Updater | Option 2: Controller Owned | Option 3: Helper Function | Option 4: Batch Updater |
|--------|-------------------------|---------------------------|--------------------------|------------------------|
| **SRP Compliance** | ✅ Excellent | ✅ Good | ✅ Good | ✅ Excellent |
| **Reusability** | ✅ High | ❌ Low | ❌ None | ⚠️ Medium |
| **Testability** | ✅ Easy | ✅ Easy | ❌ Hard | ✅ Easy |
| **Simplicity** | ✅ Simple | ⚠️ Medium | ✅ Very Simple | ❌ Complex |
| **Package Coupling** | ✅ Low | ⚠️ Medium | ✅ Low | ❌ High (plan dep) |
| **Code Organization** | ✅ Clear | ⚠️ OK | ❌ Scattered | ✅ Clear |
| **No crdSource pollution** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |

---

## Recommendation: Option 1 ⭐

**Best Choice**: Create `pkg/crd/status_updater.go` with dedicated `StatusUpdater` interface.

### Why Option 1?

1. **Clean Separation of Concerns**
   - `crdSource` = reading (Source interface)
   - `StatusUpdater` = writing (status updates)
   - Each does one thing well

2. **Follows Repository Pattern**
   - `DNSEndpointClient` = CRUD operations
   - `StatusUpdater` = business logic for status updates
   - Clean layering

3. **Reusable & Testable**
   - Can use `StatusUpdater` anywhere (not tied to controller)
   - Easy to mock `DNSEndpointClient` in tests
   - Clear dependencies

4. **Consistent with pkg/crd Design**
   - `dnsendpoint_client.go` = repository layer
   - `client_factory.go` = infrastructure layer
   - `status_updater.go` = service layer
   - Natural progression

5. **Eliminates Dual Instance Problem**
   - No need for two `crdSource` instances
   - Main source does reading
   - StatusUpdater does writing
   - Clear ownership

### Implementation Steps

1. Create `pkg/crd/status_updater.go`
2. Remove `UpdateDNSEndpointStatus` from `source/crd.go`
3. Update `controller/execute.go` to use `StatusUpdater`
4. Update tests
5. Update documentation

### Example Usage After Refactoring

```go
// In execute.go - clean and clear
statusUpdater := crd.NewDNSEndpointStatusUpdater(dnsEndpointClient)

callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
    refs := extractDNSEndpointsFromChanges(changes)
    for key, ref := range refs {
        statusUpdater.UpdateDNSEndpointStatus(ctx, ref.namespace, ref.name, success, message)
    }
}
```

---

## Migration Path

If choosing Option 1, here's the migration:

### Step 1: Create StatusUpdater
- Add `pkg/crd/status_updater.go`
- Implement interface and type

### Step 2: Update execute.go
- Modify `registerStatusUpdateCallbacks()`
- Create `StatusUpdater` instead of `crdSource`
- Use `StatusUpdater` in callback

### Step 3: Remove from crdSource
- Delete `UpdateDNSEndpointStatus()` method from `source/crd.go`
- Update `callback-status-update-implementation.md`

### Step 4: Update Tests
- Remove status update tests from `source/crd_test.go`
- Add tests for `StatusUpdater` in `pkg/crd/status_updater_test.go`

### Step 5: Update Documentation
- Update `controller-crd-update-flow.md`
- Update `crd-refactoring-plan.md`

This keeps backwards compatibility while improving the design.
