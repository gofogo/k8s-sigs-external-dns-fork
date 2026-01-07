# Status Updater Implementation Testing Guide

## Overview

Two implementations of DNSEndpoint status updates are available for testing and comparison:

- **Option 1 (Recommended - Default)**: `StatusUpdater` in `pkg/crd` package
- **Option 2**: `DNSEndpointStatusManager` in `controller` package

Both implementations are **fully functional** and can be switched using an environment variable.

**Note**: The legacy implementation (`crdSource.UpdateDNSEndpointStatus()`) has been **removed** because it violated Single Responsibility Principle and created dual crdSource instances.

## Quick Start

### Testing Each Option

Set the `STATUS_UPDATER_IMPL` environment variable before running external-dns:

```bash
# Test Option 1 (Recommended - Default)
# No environment variable needed, or explicitly set:
export STATUS_UPDATER_IMPL=pkg-crd
./external-dns --source=crd ...

# Test Option 2
export STATUS_UPDATER_IMPL=controller
./external-dns --source=crd ...
```

### Check Logs

Look for the implementation being used:

```bash
# You should see one of these log lines:
INFO Using status updater implementation: pkg-crd
INFO Using status updater implementation: controller
```

And one of these:

```bash
INFO Registered DNSEndpoint status update callback (Option 1: pkg/crd)
INFO Registered DNSEndpoint status update callback (Option 2: controller)
```

## Detailed Testing Instructions

### 1. Setup Test Environment

```bash
# Build external-dns
go build -o external-dns .

# Create test DNSEndpoint CRD
kubectl apply -f - <<EOF
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: test-endpoint
  namespace: default
spec:
  endpoints:
  - dnsName: test.example.com
    recordTTL: 300
    recordType: A
    targets:
    - 1.2.3.4
EOF
```

### 2. Test Each Implementation

#### Option 1: pkg-crd (Recommended - Default)

```bash
export STATUS_UPDATER_IMPL=pkg-crd

# Run external-dns
./external-dns \
  --source=crd \
  --provider=inmemory \
  --once \
  --log-level=debug

# Check status was updated
kubectl get dnsendpoint test-endpoint -o yaml | grep -A10 status:
```

**Expected Output**: Same as above

#### Option 2: controller

```bash
export STATUS_UPDATER_IMPL=controller

# Run external-dns
./external-dns \
  --source=crd \
  --provider=inmemory \
  --once \
  --log-level=debug

# Check status was updated
kubectl get dnsendpoint test-endpoint -o yaml | grep -A10 status:
```

**Expected Output**: Same as above

### 3. Test Error Handling

Test that status is updated on failure:

```bash
# Create DNSEndpoint with invalid target
kubectl apply -f - <<EOF
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: test-error
  namespace: default
spec:
  endpoints:
  - dnsName: test.invalid-tld
    recordTTL: 300
    recordType: A
    targets:
    - invalid-target.
EOF

# Run with each option and verify error status
export STATUS_UPDATER_IMPL=legacy  # or pkg-crd or controller
./external-dns --source=crd --provider=inmemory --once

# Check for error in status
kubectl get dnsendpoint test-error -o yaml | grep -A5 'type: Programmed'
```

**Expected Output**:
```yaml
  - lastTransitionTime: "2025-01-XX..."
    message: "Failed to sync: ..."
    reason: NotProgrammed
    status: "False"
    type: Programmed
```

### 4. Performance Testing

Compare performance of each implementation:

```bash
# Create multiple DNSEndpoints
for i in {1..100}; do
  kubectl apply -f - <<EOF
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: test-$i
  namespace: default
spec:
  endpoints:
  - dnsName: test-$i.example.com
    recordTTL: 300
    recordType: A
    targets:
    - 1.2.3.$i
EOF
done

# Time each implementation
time (export STATUS_UPDATER_IMPL=legacy; ./external-dns --source=crd --provider=inmemory --once)
time (export STATUS_UPDATER_IMPL=pkg-crd; ./external-dns --source=crd --provider=inmemory --once)
time (export STATUS_UPDATER_IMPL=controller; ./external-dns --source=crd --provider=inmemory --once)
```

## Comparison Criteria

### Functionality ✓
All three implementations should:
- ✓ Update status on successful sync
- ✓ Update status on failed sync
- ✓ Set Accepted condition when DNSEndpoint is processed
- ✓ Set Programmed condition after sync
- ✓ Update observedGeneration correctly

### Code Quality

| Aspect | Option 1 (pkg-crd) | Option 2 (controller) |
|--------|--------------------|-----------------------|
| **Separation of Concerns** | ✅ Excellent | ✅ Good |
| **Reusability** | ✅ Highly reusable | ⚠️ Controller-specific |
| **Testability** | ✅ Easy to mock | ✅ Easy to mock |
| **Package Dependencies** | ✅ Clean | ⚠️ Controller depends on crd |
| **Code Simplicity** | ✅ Direct usage | ✅ Direct usage |

### Performance

Expected: **Both options should have identical performance**
- Same underlying operations (Get + UpdateStatus)
- Same number of API calls
- Negligible difference in object creation overhead

## Making the Final Decision

### Choose Option 1 (pkg-crd) if:
✅ You want clean separation of concerns
✅ You value reusability
✅ You follow repository/service pattern
✅ You want to avoid dual crdSource instances
✅ You plan to use status updates from multiple places

### Choose Option 2 (controller) if:
✅ You want simpler package structure
✅ You prefer controller-owned status logic
✅ You don't need status updates outside controller
✅ You want fewer abstractions


## Removing Unused Options

### If Keeping Option 1 (pkg-crd)

**Delete these files:**
```bash
# Remove Option 2
rm controller/dnsendpoint_status.go

# Keep source/crd.go but remove UpdateDNSEndpointStatus method
# (See detailed steps below)
```

**Update execute.go:**
```go
// In controller/execute.go

// 1. Remove the switch statement (lines 444-463)
// 2. Replace registerStatusUpdateCallbacks with just the Option 1 code:

func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    kubeClient, err := getKubeClient(cfg)
    if err != nil {
        log.Warnf("Could not create Kubernetes client for status updates: %v", err)
        return
    }

    restClient, _, err := crd.NewCRDClientForAPIVersionKind(
        kubeClient, cfg.KubeConfig, cfg.APIServerURL,
        cfg.CRDSourceAPIVersion, cfg.CRDSourceKind,
    )
    if err != nil {
        log.Warnf("Could not create CRD REST client for status updates: %v", err)
        return
    }

    dnsEndpointClient := crd.NewDNSEndpointClient(
        restClient, cfg.Namespace, cfg.CRDSourceKind, metav1.ParameterCodec,
    )

    statusUpdater := crd.NewDNSEndpointStatusUpdater(dnsEndpointClient)

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

// 3. Delete registerStatusUpdateCallbacksLegacy function (lines 470-511)
// 4. Delete registerStatusUpdateCallbacksOption2 function (lines 570-617)
// 5. Keep getKubeClient helper (lines 621-628)
```

**Update source/crd.go:**
```go
// In source/crd.go

// Delete the UpdateDNSEndpointStatus method (lines 174-195)
```

### If Keeping Option 2 (controller)

**Delete these files:**
```bash
# Remove Option 1
rm pkg/crd/status_updater.go
```

**Update execute.go:**
```go
// In controller/execute.go

// 1. Remove the switch statement (lines 444-463)
// 2. Replace registerStatusUpdateCallbacks with just the Option 2 code:

func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
    kubeClient, err := getKubeClient(cfg)
    if err != nil {
        log.Warnf("Could not create Kubernetes client for status updates: %v", err)
        return
    }

    restClient, _, err := crd.NewCRDClientForAPIVersionKind(
        kubeClient, cfg.KubeConfig, cfg.APIServerURL,
        cfg.CRDSourceAPIVersion, cfg.CRDSourceKind,
    )
    if err != nil {
        log.Warnf("Could not create CRD REST client for status updates: %v", err)
        return
    }

    dnsEndpointClient := crd.NewDNSEndpointClient(
        restClient, cfg.Namespace, cfg.CRDSourceKind, metav1.ParameterCodec,
    )

    statusManager := NewDNSEndpointStatusManager(dnsEndpointClient)

    callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
        dnsEndpoints := extractDNSEndpointsFromChanges(changes)
        log.Debugf("Updating status for %d DNSEndpoint(s)", len(dnsEndpoints))

        for key, ref := range dnsEndpoints {
            err := statusManager.UpdateStatus(ctx, ref.namespace, ref.name, success, message)
            if err != nil {
                log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
            }
        }
    }

    ctrl.RegisterStatusUpdateCallback(callback)
    log.Info("Registered DNSEndpoint status update callback")
}

// 3. Delete registerStatusUpdateCallbacksLegacy function (lines 470-511)
// 4. Delete registerStatusUpdateCallbacksOption1 function (lines 517-564)
// 5. Keep getKubeClient helper (lines 621-628)
```

**Update source/crd.go:**
```go
// In source/crd.go

// Delete the UpdateDNSEndpointStatus method (lines 174-195)
```

## Verification After Cleanup

After removing unused options, verify:

```bash
# 1. Build still works
go build ./...

# 2. Tests still pass
go test ./...

# 3. Run external-dns without STATUS_UPDATER_IMPL env var
unset STATUS_UPDATER_IMPL
./external-dns --source=crd --provider=inmemory --once

# 4. Verify status updates still work
kubectl get dnsendpoint test-endpoint -o yaml | grep -A10 status:
```

## Troubleshooting

### "Unknown STATUS_UPDATER_IMPL value"

**Cause**: Typo in environment variable value

**Fix**: Use one of: `pkg-crd`, `controller`

### "Could not create CRD source for status updates"

**Cause**: Invalid kubeconfig or API server URL

**Fix**: Verify Kubernetes connectivity:
```bash
kubectl get nodes  # Should work
```

### Status not updating

**Possible causes:**
1. `--source=crd` not specified
2. DNSEndpoint CRD not installed
3. External-dns doesn't have RBAC permissions for status subresource

**Fix for RBAC:**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns
rules:
- apiGroups: ["externaldns.k8s.io"]
  resources: ["dnsendpoints"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["externaldns.k8s.io"]
  resources: ["dnsendpoints/status"]  # <- This is required
  verbs: ["update", "patch"]
```

## Questions?

If you have questions about which option to choose or how to remove unused code:

1. Review `status-updater-refactoring-options.md` for detailed pros/cons
2. Check the comments in the code files (marked with "REFACTORING NOTE")
3. Look at the comparison matrix in the options document

## Recommendation Summary

**Choose Option 1 (pkg-crd)** for:
- ✅ Best separation of concerns
- ✅ Most reusable
- ✅ Follows established patterns
- ✅ Future-proof architecture

This is the **recommended choice** unless you have specific reasons to prefer controller-owned status management.
