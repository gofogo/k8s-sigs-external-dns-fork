# Source Directory Patterns and Improvements Analysis

**Date**: 2026-01-08
**Directory Analyzed**: `/source`
**Files Analyzed**: 101 Go files across multiple subdirectories

## Directory Structure Overview

The source directory contains:

- **101 Go files** across multiple subdirectories
- **Main subdirectories**: `annotations/`, `fqdn/`, `informers/`, `types/`, `wrappers/`
- **27+ source implementations** for different Kubernetes resources and service meshes
- Multiple test files (typically matching pattern `*_test.go`)

## Key Architectural Patterns Found

### 1. Source Interface Pattern

All sources implement a common interface defined in `source/source.go`:

```go
type Source interface {
    Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error)
    AddEventHandler(context.Context, func())
}
```

### 2. Factory Pattern

The codebase uses a centralized factory pattern in `BuildWithConfig()` function (store.go) that creates sources based on string identifiers. This is well-documented and follows good practices.

### 3. Singleton Pattern for Clients

The `SingletonClientGenerator` uses `sync.Once` to ensure thread-safe, single initialization of Kubernetes clients - good for resource efficiency.

### 4. Informer Pattern

Consistent use of Kubernetes informers for watching resources with shared informer factories to reduce API server load.

---

## Issues and Recommendations

### 1. CRITICAL: Typo in Filename

**File**: `source/informers/transfomers.go`

**Issue**: The filename has a typo - "transfomers" should be "transformers"

**Impact**:

- Confusing for developers
- Inconsistent with test file `transformers_test.go`
- May cause issues with IDE navigation and tooling

**Recommendation**: Rename `transfomers.go` to `transformers.go`

---

### 2. Inconsistent Receiver Names

**Issue**: Receiver names across different source implementations are inconsistent:

- Most common: `sc` (51 occurrences)
- Others: `in` (45), `rt` (30), `ts` (14), `ps` (9), `ns` (4), `vs` (3), etc.

**Examples**:

- `serviceSource` uses `sc`
- `ingressSource` uses `sc`
- `nodeSource` uses `ns`
- `podSource` uses `ps`
- `traefikSource` uses `ts`
- `f5TransportServerSource` uses `ts`

**Problems**:

- Both `traefikSource` and `f5TransportServerSource` use `ts`
- No clear naming convention
- Reduces code readability when switching between files

**Recommendation**: Standardize receiver names using either:

- **Option A**: First letter(s) of the type name (e.g., `ss` for serviceSource, `is` for ingressSource, `ns` for nodeSource)
- **Option B**: Generic `s` for all sources (simpler, more consistent)
- Document the chosen convention in a coding standards guide

---

### 3. Extensive Use of context.TODO() in Tests

**Issue**: 14 test files use `context.TODO()` extensively

**Files affected**:

- `store_test.go`, `ambassador_host_test.go`, `kong_tcpingress_test.go`, `service_test.go`, `gloo_proxy_test.go`, `f5_virtualserver_test.go`, `ingress_test.go`, etc.

**Problems**:

- `context.TODO()` is meant to be temporary
- Makes it harder to add timeout/cancellation logic later
- Not following best practices for production-ready code

**Recommendation**:

- Create a helper function like `testContext()` that returns `context.Background()` or a context with appropriate timeout
- Replace `context.TODO()` with proper context in tests
- Example:

```go
func testContext(t *testing.T) context.Context {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    t.Cleanup(cancel)
    return ctx
}
```

---

### 4. Duplicate Code in Source Constructors

**Issue**: Each source's `NewXXXSource()` function follows similar patterns but duplicates:

- Informer factory creation
- Event handler registration
- Cache synchronization
- Error handling

**Example Pattern** (repeated across 15+ files):

```go
informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
xxxInformer := informerFactory.XXX().V1().XXXs()
_, _ = xxxInformer.Informer().AddEventHandler(informers.DefaultEventHandler())
informerFactory.Start(ctx.Done())
if err := informers.WaitForCacheSync(ctx, informerFactory); err != nil {
    return nil, err
}
```

**Recommendation**:

- Extract common initialization logic into helper functions
- Create a builder pattern or template method for source initialization
- Example:

```go
type InformerSetup struct {
    Namespace string
    Client kubernetes.Interface
    Context context.Context
}

func (s *InformerSetup) CreateAndStartFactory() (kubeinformers.SharedInformerFactory, error) {
    factory := kubeinformers.NewSharedInformerFactoryWithOptions(
        s.Client, 0, kubeinformers.WithNamespace(s.Namespace))
    factory.Start(s.Context.Done())
    if err := informers.WaitForCacheSync(s.Context, factory); err != nil {
        return nil, err
    }
    return factory, nil
}
```

---

### 5. Inconsistent Error Handling

**Issue**: Some sources check `err != nil` and return immediately, while others have different patterns

**241 occurrences** of `if err != nil` across 51 files suggests varied approaches

**Problems**:

- Some functions ignore errors (using `_` discard)
- Inconsistent error wrapping (some use `fmt.Errorf`, others don't)
- Missing error context in some places

**Recommendation**:

- Standardize error handling patterns
- Always wrap errors with context using `fmt.Errorf("operation failed: %w", err)`
- Document when it's acceptable to ignore errors
- Consider using linters like `errcheck` and `errorlint`

---

### 6. TODOs Indicating Incomplete Work

**Found Multiple TODOs**:

- Line 178 in `gateway.go`: "Gateway informer should be shared across gateway sources"
- Line 194 in `gateway.go`: "Namespace informer should be shared across gateway sources"
- Line 204, 196 in `istio_*.go`: "sort on endpoint creation"
- Line 451, 456 in `gateway.go`: "The ignore-hostname-annotation flag help says..."
- Line 26 in `utils.go`: "move this to the endpoint package?"
- Line 15 in `gateway_hostname.go`: "refactor common DNS label functions into a shared package"
- Line 393 in `service.go`: "Consider refactoring with generics when available"

**Recommendation**:

- Create GitHub issues for each TODO
- Prioritize TODOs related to:
  - Performance (shared informers)
  - Code organization (refactoring common functions)
  - Technical debt (generics refactoring)

---

### 7. Large File Sizes

**Largest implementation files**:

- `service_test.go`: 5,726 lines
- `istio_virtualservice_test.go`: 2,432 lines
- `istio_gateway_test.go`: 2,000 lines
- `traefik_proxy_test.go`: 1,808 lines
- `ingress_test.go`: 1,722 lines
- `gateway_httproute_test.go`: 1,590 lines
- `service.go`: 943 lines
- `traefik_proxy.go`: 915 lines
- `gateway.go`: 706 lines
- `store.go`: 704 lines

**Problems**:

- Difficult to navigate and understand
- Increases cognitive load
- Makes refactoring harder

**Recommendation**:

- Split large test files into subtests in separate files (e.g., `service_loadbalancer_test.go`, `service_nodeport_test.go`)
- Extract helper functions into separate files
- Consider table-driven tests to reduce repetition
- For implementation files, extract logical sections into helper files

---

### 8. Naming Inconsistencies

**Issues**:

- Istio Gateway source uses `gatewaySource` but Gateway API source uses `gatewayRouteSource`
- OpenShift Route source uses `ocpRouteSource` (abbreviation) while others use full names
- Some sources use full descriptive names (`f5VirtualServerSource`) while others abbreviate (`httpProxySource` for Contour)

**Recommendation**:

- Standardize naming: always use full, descriptive names
- Rename `ocpRouteSource` to `openShiftRouteSource` for clarity
- Consider renaming conflicting types (both Istio and Gateway API use "gateway")

---

### 9. Informer Lifecycle Management Inconsistency

**Issue**: Gateway sources use `wait.NeverStop` while others use `ctx.Done()`

**Example from gateway.go line 197**:

```go
informerFactory.Start(wait.NeverStop)
```

**Others use**:

```go
informerFactory.Start(ctx.Done())
```

**Problems**:

- Inconsistent behavior across sources
- `wait.NeverStop` prevents graceful shutdown
- May cause resource leaks

**Recommendation**:

- Standardize on `ctx.Done()` for all sources
- Ensures proper cleanup on context cancellation
- Better alignment with Go best practices

---

### 10. Missing Abstraction for Common Endpoint Creation

**Issue**: Multiple sources have similar endpoint creation logic but implement it differently

**Patterns found**:

- Converting hostnames to endpoints
- TTL extraction from annotations
- Provider-specific annotations handling
- Target extraction

**Recommendation**:

- Create a common `EndpointBuilder` type
- Centralize common endpoint creation logic
- Example:

```go
type EndpointBuilder struct {
    Annotations map[string]string
    Resource string
}

func (b *EndpointBuilder) Build(hostname string, targets []string) *endpoint.Endpoint {
    ttl := annotations.TTLFromAnnotations(b.Annotations, b.Resource)
    providerSpecific, setIdentifier := annotations.ProviderSpecificAnnotations(b.Annotations)
    return endpoint.NewEndpointWithTTL(hostname, recordType, ttl).
        WithProviderSpecific(providerSpecific).
        WithSetIdentifier(setIdentifier)
}
```

---

### 11. Lack of Interface Documentation

**Issue**: The main `Source` interface in `source.go` has minimal documentation

**Current**:

```go
// Source defines the interface Endpoint sources should implement.
type Source interface {
    Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error)
    AddEventHandler(context.Context, func())
}
```

**Recommendation**:

- Add comprehensive interface documentation including:
  - Expected behavior of each method
  - Contract for implementations
  - Thread-safety guarantees
  - Example usage
- Document the lifecycle: initialization → event registration → endpoint retrieval

---

### 12. Wrappers Directory Organization

**Current Structure**:

```
wrappers/
  ├── dedupsource.go
  ├── multisource.go
  ├── nat64source.go
  ├── post_processor.go
  ├── targetfiltersource.go
  └── types.go
```

**Good Practices**:

- Clear separation of concerns
- Each wrapper has a single responsibility
- Good use of decorator pattern

**Potential Improvements**:

- Add package-level documentation explaining the wrapper pattern
- Consider adding examples in documentation
- Ensure all wrappers follow consistent patterns

---

## Summary of Key Recommendations (Prioritized)

### High Priority

1. **Fix typo**: Rename `transfomers.go` → `transformers.go`
2. **Standardize receiver names** across all sources
3. **Replace `context.TODO()`** in tests with proper contexts
4. **Fix informer lifecycle**: Use `ctx.Done()` instead of `wait.NeverStop`

### Medium Priority

5. **Extract duplicate initialization code** into helper functions
6. **Address TODOs** - create issues and implement shared informers
7. **Split large test files** into smaller, focused files
8. **Standardize error handling** patterns

### Low Priority (Long-term)

9. **Refactor common endpoint creation** logic
10. **Improve interface documentation**
11. **Standardize naming conventions** across all source types
12. **Consider generics refactoring** (where applicable)

---

## Positive Patterns Worth Maintaining

1. **Well-documented factory pattern** in `store.go`
2. **Singleton client generator** - excellent use of `sync.Once`
3. **Separation of concerns** - annotations, fqdn, informers in separate packages
4. **Consistent use of informers** for Kubernetes resource watching
5. **Comprehensive test coverage** (even if files are large)
6. **Clear source type definitions** in `types/types.go`

---

## Conclusion

The codebase shows good architectural foundations with consistent patterns for implementing sources. However, it would benefit significantly from:

- Consistency improvements (receiver names, error handling, lifecycle management)
- Reducing code duplication through helper functions
- Better file organization (splitting large files)
- Addressing technical debt (TODOs and informer sharing)

These improvements would enhance maintainability, readability, and reduce the cognitive load for developers working on the source implementations.
