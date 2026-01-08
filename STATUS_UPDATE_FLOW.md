# DNSEndpoint Status Update Flow - Design Document

## Problem Statement

The DNSEndpoint CRD needs status fields (Conditions, LastSyncTime, ProviderStatus) to be updated based on the actual sync results with the DNS provider. However, the status cannot be set directly in `source/crd.go` because the sync happens later in the flow:

**Flow:** Source → Provider → Plan → ApplyChanges

The source doesn't know if the provider sync will succeed or fail, so status must be updated **after** `Registry.ApplyChanges()` in the controller.

## Current Architecture

### Controller Flow (controller/controller.go:190-263)

```go
func (c *Controller) RunOnce(ctx context.Context) error {
    // 1. Get current records from DNS provider
    regRecords, err := c.Registry.Records(ctx)

    // 2. Get desired endpoints from sources (including CRD source)
    sourceEndpoints, err := c.Source.Endpoints(ctx)

    // 3. Calculate plan (what changes are needed)
    plan := plan.Calculate()

    // 4. Apply changes to DNS provider
    if plan.Changes.HasChanges() {
        err = c.Registry.ApplyChanges(ctx, plan.Changes)
        if err != nil {
            // SYNC FAILED - status should be updated here
            return err
        } else {
            // SYNC SUCCESSFUL - status should be updated here
            emitChangeEvent(c.EventEmitter, *plan.Changes, events.RecordReady)
        }
    }

    return nil
}
```

### Current CRD Source (source/crd.go:221-230)

Currently only updates `ObservedGeneration`:

```go
if dnsEndpoint.Status.ObservedGeneration == dnsEndpoint.Generation {
    continue
}

dnsEndpoint.Status.ObservedGeneration = dnsEndpoint.Generation
_, err = cs.UpdateStatus(ctx, dnsEndpoint)
```

## Solution: Using RefObject Pattern

### Pattern Discovery

The endpoint package already supports attaching source object references via `RefObject`:

**Endpoint Methods (endpoint/endpoint.go:354-361):**

```go
func (e *Endpoint) WithRefObject(obj *events.ObjectReference) *Endpoint {
    e.refObject = obj
    return e
}

func (e *Endpoint) RefObject() *events.ObjectReference {
    return e.refObject
}
```

**ObjectReference Structure (pkg/events/types.go:72-79):**

```go
type ObjectReference struct {
    Kind       string
    ApiVersion string
    Namespace  string
    Name       string
    UID        types.UID
    Source     string
}
```

**Helper Function (pkg/events/types.go:90-99):**

```go
func NewObjectReference(obj runtime.Object, source string) *ObjectReference {
    return &ObjectReference{
        Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
        ApiVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
        Namespace:  obj.GetNamespace(),
        Name:       obj.GetName(),
        UID:        obj.GetUID(),
        Source:     source,
    }
}
```

### Example Usage (source/fake.go:79-88)

```go
ep.WithRefObject(events.NewObjectReference(&v1.Pod{
    TypeMeta: metav1.TypeMeta{
        Kind:       "Pod",
        APIVersion: "v1",
    },
    ObjectMeta: metav1.ObjectMeta{
        Name:      types.Fake + "-" + ep.DNSName,
        Namespace: v1.NamespaceDefault,
    },
}, types.Fake))
```

## Implementation Plan

### Step 1: Update CRD Source to Attach RefObject

**File:** `source/crd.go`
**Location:** Around line 214 (after `ep.WithLabel(...)`)

**Change:**

```go
// Current code at line 214:
ep.WithLabel(endpoint.ResourceLabelKey, fmt.Sprintf("crd/%s/%s", dnsEndpoint.Namespace, dnsEndpoint.Name))

// ADD after line 214:
ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))
```

**Full Context (lines 214-217):**

```go
ep.WithLabel(endpoint.ResourceLabelKey, fmt.Sprintf("crd/%s/%s", dnsEndpoint.Namespace, dnsEndpoint.Name))
ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

crdEndpoints = append(crdEndpoints, ep)
```

### Step 2: Update Controller to Set DNSEndpoint Status

**File:** `controller/controller.go`
**Location:** After line 247 (after `Registry.ApplyChanges`)

**Current Code (lines 246-258):**

```go
if plan.Changes.HasChanges() {
    err = c.Registry.ApplyChanges(ctx, plan.Changes)
    if err != nil {
        registryErrorsTotal.Counter.Inc()
        deprecatedRegistryErrors.Counter.Inc()
        return err
    } else {
        emitChangeEvent(c.EventEmitter, *plan.Changes, events.RecordReady)
    }
} else {
    controllerNoChangesTotal.Counter.Inc()
    log.Info("All records are already up to date")
}
```

**Updated Code:**

```go
if plan.Changes.HasChanges() {
    err = c.Registry.ApplyChanges(ctx, plan.Changes)
    if err != nil {
        registryErrorsTotal.Counter.Inc()
        deprecatedRegistryErrors.Counter.Inc()

        // Update DNSEndpoint status on failure
        c.updateDNSEndpointStatus(ctx, plan.Changes, false, err.Error())

        return err
    } else {
        emitChangeEvent(c.EventEmitter, *plan.Changes, events.RecordReady)

        // Update DNSEndpoint status on success
        c.updateDNSEndpointStatus(ctx, plan.Changes, true, "Successfully synced DNS records")
    }
} else {
    controllerNoChangesTotal.Counter.Inc()
    log.Info("All records are already up to date")

    // Update DNSEndpoint status on no-op (still considered success)
    c.updateDNSEndpointStatus(ctx, plan.Changes, true, "All records are already up to date")
}
```

### Step 3: Implement updateDNSEndpointStatus Method

**File:** `controller/controller.go`
**Location:** Add as new method

**Implementation:**

```go
// updateDNSEndpointStatus updates the status of DNSEndpoint CRDs based on sync results
func (c *Controller) updateDNSEndpointStatus(ctx context.Context, changes *plan.Changes, success bool, message string) {
    // Collect unique DNSEndpoint references from all endpoints in the plan
    dnsEndpoints := make(map[string]*events.ObjectReference)

    // Check all endpoints in Creates, UpdateOld, UpdateNew, and Delete
    allEndpoints := append(append(append(
        changes.Create,
        changes.UpdateOld...),
        changes.UpdateNew...),
        changes.Delete...)

    for _, ep := range allEndpoints {
        ref := ep.RefObject()
        if ref != nil && ref.Kind == "DNSEndpoint" {
            key := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
            dnsEndpoints[key] = ref
        }
    }

    // Update status for each unique DNSEndpoint
    for _, ref := range dnsEndpoints {
        if err := c.updateSingleDNSEndpointStatus(ctx, ref, success, message); err != nil {
            log.Warnf("Failed to update status for DNSEndpoint %s/%s: %v",
                ref.Namespace, ref.Name, err)
        }
    }
}

// updateSingleDNSEndpointStatus updates status for a single DNSEndpoint
func (c *Controller) updateSingleDNSEndpointStatus(ctx context.Context, ref *events.ObjectReference, success bool, message string) error {
    // Get the CRD source (type assertion)
    crdSrc, ok := c.Source.(*source.crdSource)
    if !ok {
        // Not a CRD source, skip
        return nil
    }

    // Fetch the current DNSEndpoint
    dnsEndpoint, err := crdSrc.Get(ctx, ref.Namespace, ref.Name)
    if err != nil {
        return fmt.Errorf("failed to get DNSEndpoint: %w", err)
    }

    // Update status fields based on success/failure
    if success {
        apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    } else {
        apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
    }

    dnsEndpoint.Status.ObservedGeneration = dnsEndpoint.Generation

    // Update the status
    _, err = crdSrc.UpdateStatus(ctx, dnsEndpoint)
    return err
}
```

### Step 4: Add Get Method to CRD Source

**File:** `source/crd.go`
**Location:** Add new method after UpdateStatus (after line 265)

**Implementation:**

```go
func (cs *crdSource) Get(ctx context.Context, namespace, name string) (*apiv1alpha1.DNSEndpoint, error) {
    result := &apiv1alpha1.DNSEndpoint{}
    return result, cs.crdClient.Get().
        Namespace(namespace).
        Resource(cs.crdResource).
        Name(name).
        Do(ctx).
        Into(result)
}
```

## Alternative Approaches Considered

### 1. Add Method to Source Interface

- **Pros:** Clean interface design
- **Cons:** All sources must implement it (breaking change)

### 2. Track DNSEndpoints Separately

- **Pros:** No source coupling
- **Cons:** Requires parsing endpoint labels, more complex

### 3. Using RefObject (CHOSEN)

- **Pros:**
  - Existing pattern already in codebase (see fake.go)
  - No interface changes needed
  - Type assertion allows optional behavior
  - Clean separation of concerns
- **Cons:**
  - Requires type assertion in controller
  - Only works if source implements Get method

## Key Files Modified

1. **apis/v1alpha1/dnsendpoint.go** - Status struct and constants (DONE)
2. **apis/v1alpha1/status_helpers.go** - Helper functions (DONE)
3. **apis/v1alpha1/status_helpers_test.go** - Tests (DONE)
4. **source/crd.go** - Add WithRefObject() and Get() method (TODO)
5. **controller/controller.go** - Update status after ApplyChanges (TODO)

## Testing Strategy

1. **Unit Tests:** Already created for status helpers
2. **Integration Tests:**
   - Create DNSEndpoint
   - Trigger sync
   - Verify status conditions are set correctly
   - Test both success and failure scenarios
3. **Manual Testing:**
   - Deploy with updated CRDs
   - Create DNSEndpoint resource
   - Watch status updates: `kubectl get dnsendpoint -o yaml -w`

## Notes

- The controller currently doesn't distinguish between CRD source and other sources
- Type assertion allows this feature to work only when CRD source is used
- Other sources (Service, Ingress, etc.) won't be affected
- The RefObject pattern is already used in fake.go, so this is consistent with existing patterns
- Status updates are best-effort (logged warnings on failure, don't block reconciliation)
