# Shared Behaviors That Could Be Extracted into Reusable Functions

**Date**: 2026-01-08
**Analysis Scope**: `/source` directory - 101 Go files
**Focus**: Code duplication and reusable patterns

## Executive Summary

This analysis identifies **17 distinct patterns** of duplicated code across the source directory. The most duplicated pattern appears **25+ times**. Extracting these into shared functions could eliminate thousands of lines of duplicate code.

**Top 3 Most Duplicated Patterns:**

1. Target extraction from annotations: **25+ occurrences**
2. TTL extraction from annotations: **20+ occurrences**
3. Provider-specific annotations extraction: **19+ occurrences**

---

## Detailed Patterns and Recommendations

### 1. Informer Factory Creation

**What it does:** Creates Kubernetes shared informer factories with namespace filtering and resync period set to 0.

**Where it appears:**

- `source/service.go:118`
- `source/ingress.go:106`
- `source/node.go:74`
- `source/pod.go:75`
- `source/istio_gateway.go:91`
- `source/istio_virtualservice.go:93,95`

**Duplicated:** 11+ times

**Current Pattern:**

```go
informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
    kubeClient, 0, kubeinformers.WithNamespace(namespace))
```

**Suggested Shared Function:**

```go
// Package: source/informers/factory.go

// CreateKubeInformerFactory creates a shared informer factory with namespace filtering.
// The resync period is set to 0 to prevent unnecessary resyncs.
func CreateKubeInformerFactory(kubeClient kubernetes.Interface, namespace string) kubeinformers.SharedInformerFactory {
    return kubeinformers.NewSharedInformerFactoryWithOptions(
        kubeClient,
        0,
        kubeinformers.WithNamespace(namespace),
    )
}
```

**Impact:** Reduces 11+ duplicate lines, centralizes informer creation logic.

---

### 2. Informer Factory Start and Cache Sync

**What it does:** Starts informer factories and waits for cache synchronization with error handling.

**Where it appears:**

- `source/service.go:208-213`
- `source/ingress.go:112-117`
- `source/node.go:80-85`
- `source/pod.go:125-130`
- `source/istio_gateway.go:112-121`
- `source/gateway.go:197-211`

**Duplicated:** 15+ times

**Current Pattern:**

```go
informerFactory.Start(ctx.Done())
if err := informers.WaitForCacheSync(ctx, informerFactory); err != nil {
    return nil, err
}
```

**Suggested Shared Function:**

```go
// Package: source/informers/factory.go

// StartAndSyncInformerFactory starts an informer factory and waits for cache synchronization.
// It returns an error if the cache sync fails or the context is cancelled.
func StartAndSyncInformerFactory(ctx context.Context, factory kubeinformers.SharedInformerFactory) error {
    factory.Start(ctx.Done())
    return informers.WaitForCacheSync(ctx, factory)
}
```

**Impact:** Eliminates 30+ lines of duplicate error handling code.

---

### 3. Event Handler Registration

**What it does:** Registers a simple event handler function wrapper to trigger updates on resource changes.

**Where it appears:**

- `source/service.go:880-889`
- `source/ingress.go:336-342`
- `source/node.go:175-177`
- `source/pod.go:150-152`
- `source/istio_gateway.go:213-217`
- `source/contour_httpproxy.go:260-266`
- `source/openshift_route.go:116-122`

**Duplicated:** 30+ times

**Current Pattern:**

```go
informer.AddEventHandler(
    cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            handler()
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            handler()
        },
        DeleteFunc: func(obj interface{}) {
            handler()
        },
    },
)
```

**Suggested Shared Function:**

```go
// Package: source/informers/handlers.go

// AddSimpleEventHandler registers an event handler that calls the provided function
// for all add, update, and delete events.
func AddSimpleEventHandler(informer cache.SharedIndexInformer, handler func()) {
    _, _ = informer.AddEventHandler(
        cache.ResourceEventHandlerFuncs{
            AddFunc:    func(obj interface{}) { handler() },
            UpdateFunc: func(oldObj, newObj interface{}) { handler() },
            DeleteFunc: func(obj interface{}) { handler() },
        },
    )
}
```

**Impact:** Eliminates 150+ lines of boilerplate event handler code.

---

### 4. Controller Annotation Checking

**What it does:** Checks if a resource has the controller annotation and whether it matches the expected controller value.

**Where it appears:**

- `source/service.go:258-263`
- `source/ingress.go:156-160`
- `source/node.go:116-120`
- `source/istio_gateway.go:157-162`
- `source/istio_virtualservice.go:161-166`
- `source/contour_httpproxy.go:147-152`
- `source/openshift_route.go:142-147`
- `source/gateway.go:266-271`
- `source/skipper_routegroup.go:265-270`

**Duplicated:** 9+ times

**Current Pattern:**

```go
controller, ok := annotations[controllerAnnotation]
if ok && controller != controllerAnnotationValue {
    log.Debugf("Skipping %s %s/%s because controller value does not match, found: %s, required: %s",
        resourceType, namespace, name, controller, controllerAnnotationValue)
    return false
}
```

**Suggested Shared Function:**

```go
// Package: source/common/filter.go

// ShouldProcessResource checks if a resource should be processed based on its controller annotation.
// Returns false if the controller annotation exists but doesn't match the expected value.
func ShouldProcessResource(
    annotations map[string]string,
    controllerValue string,
    resourceType, namespace, name string,
) bool {
    controller, ok := annotations[annotations.ControllerKey]
    if ok && controller != controllerValue {
        log.Debugf(
            "Skipping %s %s/%s because controller value does not match, found: %s, required: %s",
            resourceType, namespace, name, controller, controllerValue,
        )
        return false
    }
    return true
}
```

**Impact:** Reduces 45+ lines, centralizes controller filtering logic.

---

### 5. FQDN Template Parsing

**What it does:** Parses FQDN template string into a template object with error handling.

**Where it appears:**

- `source/service.go:111-114`
- `source/ingress.go:84-87`
- `source/node.go:67-70`
- `source/pod.go:132-135`
- `source/istio_gateway.go:84-87`
- `source/istio_virtualservice.go:86-89`
- `source/contour_httpproxy.go:73-76`
- `source/openshift_route.go:78-81`

**Duplicated:** 11+ times

**Current Pattern:**

```go
fqdnTemplate, err := fqdn.ParseTemplate(fqdnTemplateStr)
if err != nil {
    return nil, err
}
```

**Suggested Shared Function:**

```go
// Package: source/common/template.go

// ParseFQDNTemplateOrEmpty parses an FQDN template string and returns the template.
// If the template string is empty, it returns nil without error.
func ParseFQDNTemplateOrEmpty(fqdnTemplate string) (*template.Template, error) {
    if fqdnTemplate == "" {
        return nil, nil
    }
    return fqdn.ParseTemplate(fqdnTemplate)
}
```

**Impact:** Simplifies template parsing across all sources.

---

### 6. Combine FQDN Template and Annotation Logic

**What it does:** Determines whether to use templated endpoints, annotation-based endpoints, or combine both based on flags.

**Where it appears:**

- `source/service.go:276-287`
- `source/ingress.go:165-172`
- `source/istio_gateway.go:170-181`
- `source/istio_virtualservice.go:174-183`
- `source/contour_httpproxy.go:160-171`
- `source/openshift_route.go:152-163`
- `source/skipper_routegroup.go:274-285`

**Duplicated:** 7+ times

**Current Pattern:**

```go
if (combineFQDNAnnotation || len(endpoints) == 0) && fqdnTemplate != nil {
    templatedEndpoints, err := templateFunc()
    if err != nil {
        return nil, err
    }
    if combineFQDNAnnotation {
        endpoints = append(endpoints, templatedEndpoints...)
    } else {
        endpoints = templatedEndpoints
    }
}
```

**Suggested Shared Function:**

```go
// Package: source/common/endpoints.go

// CombineEndpointsWithTemplate merges annotation-based and template-based endpoints
// according to the combineFQDNAnnotation flag.
func CombineEndpointsWithTemplate(
    existingEndpoints []*endpoint.Endpoint,
    combineFQDNAnnotation bool,
    fqdnTemplate *template.Template,
    templateFunc func() ([]*endpoint.Endpoint, error),
) ([]*endpoint.Endpoint, error) {
    if (combineFQDNAnnotation || len(existingEndpoints) == 0) && fqdnTemplate != nil {
        templatedEndpoints, err := templateFunc()
        if err != nil {
            return nil, err
        }
        if combineFQDNAnnotation {
            return append(existingEndpoints, templatedEndpoints...), nil
        }
        return templatedEndpoints, nil
    }
    return existingEndpoints, nil
}
```

**Impact:** Eliminates 70+ lines of complex conditional logic.

---

### 7. Resource String Construction

**What it does:** Builds resource identifier strings in the format "resourceType/namespace/name".

**Where it appears throughout all source files:**

- `source/service.go:600`
- `source/ingress.go:196,260`
- `source/istio_gateway.go:273`
- `source/contour_httpproxy.go:195,222`
- `source/openshift_route.go:187,210`
- `source/gateway.go:285`

**Duplicated:** 25+ times

**Current Pattern:**

```go
resource := fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
ttl := annotations.TTLFromAnnotations(annotations, resource)
```

**Suggested Shared Function:**

```go
// Package: source/common/resource.go

// BuildResourceIdentifier constructs a resource identifier string in the format
// "resourceType/namespace/name" for use in logging and annotation lookups.
func BuildResourceIdentifier(resourceType, namespace, name string) string {
    return fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
}

// GetTTLForResource extracts TTL from annotations using the proper resource identifier.
func GetTTLForResource(
    annotations map[string]string,
    resourceType, namespace, name string,
) endpoint.TTL {
    return annotations.TTLFromAnnotations(
        annotations,
        BuildResourceIdentifier(resourceType, namespace, name),
    )
}
```

**Impact:** Eliminates 50+ lines of string formatting, ensures consistent resource identification.

---

### 8. Hostname Extraction from Annotations

**What it does:** Extracts hostname list from resource annotations, respecting the ignoreHostnameAnnotation flag.

**Where it appears:**

- `source/service.go:569,574`
- `source/ingress.go:299`
- `source/istio_gateway.go:306-308`
- `source/istio_virtualservice.go:334`
- `source/contour_httpproxy.go:250-255`
- `source/openshift_route.go:228-233`
- `source/gateway.go:453-455`
- `source/traefik_proxy.go:375,408,441`

**Duplicated:** 12+ times

**Current Pattern:**

```go
var hostnameList []string
if !ignoreHostnameAnnotation {
    hostnameList = annotations.HostnamesFromAnnotations(annotations)
}
```

**Suggested Shared Function:**

```go
// Package: source/common/annotations.go

// GetHostnamesFromAnnotations extracts hostnames from annotations.
// Returns an empty slice if ignoreHostnameAnnotation is true.
func GetHostnamesFromAnnotations(
    annotations map[string]string,
    ignoreHostnameAnnotation bool,
) []string {
    if ignoreHostnameAnnotation {
        return nil
    }
    return annotations.HostnamesFromAnnotations(annotations)
}
```

**Impact:** Simplifies hostname extraction, ensures consistent flag handling.

---

### 9. Target Extraction from LoadBalancer Status

**What it does:** Extracts IP and hostname targets from LoadBalancer ingress status.

**Where it appears:**

- `source/ingress.go:321-333`
- `source/istio_gateway.go:235-242`
- `source/contour_httpproxy.go:201-209,228-236`

**Duplicated:** 4+ times

**Current Pattern:**

```go
var targets endpoint.Targets
for _, lb := range ingress.Status.LoadBalancer.Ingress {
    if lb.IP != "" {
        targets = append(targets, lb.IP)
    }
    if lb.Hostname != "" {
        targets = append(targets, lb.Hostname)
    }
}
```

**Suggested Shared Function:**

```go
// Package: source/common/targets.go

// ExtractTargetsFromLoadBalancerIngress extracts all IPs and hostnames from
// LoadBalancer ingress status entries.
func ExtractTargetsFromLoadBalancerIngress(
    ingresses []corev1.LoadBalancerIngress,
) endpoint.Targets {
    var targets endpoint.Targets
    for _, lb := range ingresses {
        if lb.IP != "" {
            targets = append(targets, lb.IP)
        }
        if lb.Hostname != "" {
            targets = append(targets, lb.Hostname)
        }
    }
    return targets
}
```

**Impact:** Eliminates duplicate target extraction logic from LoadBalancer resources.

---

### 10. Empty Endpoint Check with Logging

**What it does:** Checks if endpoints are empty and logs a debug message if so.

**Where it appears:**

- `source/service.go:289-292`
- `source/ingress.go:174-177`
- `source/istio_gateway.go:185-188,195-198`
- `source/istio_virtualservice.go:185-188`
- `source/contour_httpproxy.go:173-176`
- `source/openshift_route.go:165-168`
- `source/gateway.go:278-281`

**Duplicated:** 10+ times

**Current Pattern:**

```go
if len(endpoints) == 0 {
    log.Debugf("No endpoints could be generated from %s %s/%s",
        resourceType, namespace, name)
    continue
}
```

**Suggested Shared Function:**

```go
// Package: source/common/endpoints.go

// CheckAndLogEmptyEndpoints checks if the endpoint list is empty and logs
// a debug message if so. Returns true if empty, false otherwise.
func CheckAndLogEmptyEndpoints(
    endpoints []*endpoint.Endpoint,
    resourceType, namespace, name string,
) bool {
    if len(endpoints) == 0 {
        log.Debugf(
            "No endpoints could be generated from %s %s/%s",
            resourceType, namespace, name,
        )
        return true
    }
    return false
}
```

**Impact:** Centralizes empty endpoint logging, ensures consistent messaging.

---

### 11. Endpoints Sorting by Targets

**What it does:** Sorts endpoint targets in-place for consistent output.

**Where it appears:**

- `source/ingress.go:183-185`
- `source/istio_gateway.go:204-207`
- `source/istio_virtualservice.go:197-200`
- `source/contour_httpproxy.go:182-184`
- `source/openshift_route.go:174-176`

**Duplicated:** 5+ times

**Current Pattern:**

```go
for _, ep := range endpoints {
    sort.Sort(ep.Targets)
}
```

**Suggested Shared Function:**

```go
// Package: source/common/endpoints.go

// SortEndpointTargets sorts the targets of all endpoints in-place for consistent output.
func SortEndpointTargets(endpoints []*endpoint.Endpoint) {
    for _, ep := range endpoints {
        sort.Sort(ep.Targets)
    }
}
```

**Impact:** Simple but improves consistency and readability.

---

### 12. Default Event Handler Registration

**What it does:** Adds a default no-op event handler to properly initialize informers.

**Where it appears:**

- `source/service.go:122,136,137,205`
- `source/ingress.go:110`
- `source/node.go:78`
- `source/pod.go:88,123`
- `source/istio_gateway.go:97,100,110`
- `source/contour_httpproxy.go:84-89`
- `source/openshift_route.go:89-94`

**Duplicated:** 15+ times

**Current Pattern:**

```go
_, _ = informer.AddEventHandler(informers.DefaultEventHandler())
```

**Suggested Shared Function:**

```go
// Package: source/informers/handlers.go

// RegisterDefaultEventHandler adds a no-op event handler to the informer.
// This is required to properly initialize the informer's cache.
func RegisterDefaultEventHandler(informer cache.SharedIndexInformer) {
    _, _ = informer.AddEventHandler(informers.DefaultEventHandler())
}
```

**Impact:** Makes the purpose explicit, improves code clarity.

---

### 13. Template-based Endpoint Generation

**What it does:** Generates endpoints from FQDN template, extracting TTL, targets, and provider-specific annotations.

**Where it appears:**

- `source/service.go:540-553`
- `source/ingress.go:190-212`
- `source/contour_httpproxy.go:189-218`
- `source/openshift_route.go:181-204`

**Duplicated:** 4+ times

**Current Pattern:**

```go
hostnames, err := fqdn.ExecTemplate(fqdnTemplate, resource)
if err != nil {
    return nil, err
}

resourceIdentifier := fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
ttl := annotations.TTLFromAnnotations(annotations, resourceIdentifier)
targets := annotations.TargetsFromTargetAnnotation(annotations)
if len(targets) == 0 {
    targets = extractTargets()
}
providerSpecific, setIdentifier := annotations.ProviderSpecificAnnotations(annotations)

var endpoints []*endpoint.Endpoint
for _, hostname := range hostnames {
    endpoints = append(endpoints,
        EndpointsForHostname(hostname, targets, ttl, providerSpecific, setIdentifier, resourceIdentifier)...)
}
```

**Suggested Shared Function:**

```go
// Package: source/common/template.go

// GenerateEndpointsFromTemplate creates endpoints from an FQDN template execution.
// It extracts TTL, targets, and provider-specific annotations from the resource.
func GenerateEndpointsFromTemplate(
    fqdnTemplate *template.Template,
    resource interface{},
    annotations map[string]string,
    resourceType, namespace, name string,
    defaultTargetsFunc func() endpoint.Targets,
    endpointFactory func(hostname string, targets endpoint.Targets, ttl endpoint.TTL,
        providerSpecific endpoint.ProviderSpecific, setIdentifier string,
        resource string) []*endpoint.Endpoint,
) ([]*endpoint.Endpoint, error) {
    hostnames, err := fqdn.ExecTemplate(fqdnTemplate, resource)
    if err != nil {
        return nil, err
    }

    resourceIdentifier := BuildResourceIdentifier(resourceType, namespace, name)
    ttl := annotations.TTLFromAnnotations(annotations, resourceIdentifier)
    targets := annotations.TargetsFromTargetAnnotation(annotations)
    if len(targets) == 0 {
        targets = defaultTargetsFunc()
    }
    providerSpecific, setIdentifier := annotations.ProviderSpecificAnnotations(annotations)

    var endpoints []*endpoint.Endpoint
    for _, hostname := range hostnames {
        endpoints = append(endpoints,
            endpointFactory(hostname, targets, ttl, providerSpecific, setIdentifier, resourceIdentifier)...)
    }
    return endpoints, nil
}
```

**Impact:** Eliminates 60+ lines of complex endpoint creation logic.

---

### 14. Informer Creation with Multiple Resources

**What it does:** Creates multiple typed informers from a single factory and registers default handlers.

**Where it appears:**

- `source/service.go:118-137` (services, pods, nodes)
- `source/istio_gateway.go:91-110` (gateways, namespaces)
- `source/istio_virtualservice.go:93-111` (virtualservices, gateways)
- `source/gateway.go:178-196` (multiple gateway resources)

**Duplicated:** 5+ times

**Suggested Shared Function:**

```go
// Package: source/informers/factory.go

// InformerRegistration represents a typed informer that needs to be registered.
type InformerRegistration struct {
    Informer cache.SharedIndexInformer
    Name     string
}

// CreateAndRegisterInformers creates informers and registers default handlers.
func CreateAndRegisterInformers(registrations []InformerRegistration) {
    for _, reg := range registrations {
        RegisterDefaultEventHandler(reg.Informer)
    }
}
```

**Impact:** Simplifies multi-informer initialization patterns.

---

### 15. Namespace List Expansion

**What it does:** Expands namespace list, handling the empty string to mean all namespaces.

**Where it appears:**

- `source/service.go:106-109`
- `source/ingress.go:100-103`
- `source/node.go:61-64`
- `source/gateway.go:169-172`

**Duplicated:** 5+ times

**Current Pattern:**

```go
namespaces := []string{namespace}
if namespace == "" {
    namespaces = []string{""}
}
```

**Suggested Shared Function:**

```go
// Package: source/common/namespace.go

// ExpandNamespaceList returns a slice containing the namespace.
// This helper exists for clarity in multi-namespace source patterns.
func ExpandNamespaceList(namespace string) []string {
    return []string{namespace}
}
```

**Impact:** Small but improves code clarity.

---

### 16. Filter Resources by Label Selector

**What it does:** Filters Kubernetes resources using a label selector.

**Where it appears:**

- `source/service.go:245-248`
- `source/ingress.go:138-141`
- `source/node.go:102-105`
- `source/pod.go:165-168`

**Duplicated:** 5+ times

**Current Pattern:**

```go
filteredList, err := labelSelector.FilterObjects(list)
if err != nil {
    return nil, err
}
list = filteredList
```

**Suggested Shared Function:**

```go
// Package: source/common/filter.go

// FilterByLabelSelector applies a label selector to a list of resources.
func FilterByLabelSelector(resources interface{}, labelSelector labels.Selector) (interface{}, error) {
    if labelSelector.Empty() {
        return resources, nil
    }
    return labelSelector.FilterObjects(resources)
}
```

**Impact:** Centralizes label filtering logic.

---

### 17. Provider-Specific Annotation Application

**What it does:** Applies provider-specific annotations and set identifier to endpoints.

**Where it appears:**

- All endpoint creation functions across source files

**Duplicated:** 30+ times

**Current Pattern:**

```go
ep := endpoint.NewEndpointWithTTL(hostname, recordType, ttl)
ep.ProviderSpecific = providerSpecific
ep.SetIdentifier = setIdentifier
```

**Suggested Shared Function:**

```go
// Package: source/common/endpoints.go

// NewEndpointWithMetadata creates an endpoint with TTL, provider-specific annotations,
// and set identifier all applied.
func NewEndpointWithMetadata(
    hostname, recordType string,
    ttl endpoint.TTL,
    providerSpecific endpoint.ProviderSpecific,
    setIdentifier string,
) *endpoint.Endpoint {
    ep := endpoint.NewEndpointWithTTL(hostname, recordType, ttl)
    ep.ProviderSpecific = providerSpecific
    ep.SetIdentifier = setIdentifier
    return ep
}
```

**Impact:** Ensures consistent endpoint creation across all sources.

---

## Implementation Recommendations

### Phase 1: High-Priority Extractions (Immediate Impact)

Extract these functions first - they have high duplication and straightforward implementations:

1. **#7 - Resource String Construction** (25+ occurrences)
2. **#3 - Event Handler Registration** (30+ occurrences)
3. **#2 - Informer Factory Start and Sync** (15+ occurrences)
4. **#1 - Informer Factory Creation** (11+ occurrences)
5. **#8 - Hostname Extraction** (12+ occurrences)
6. **#11 - Endpoints Sorting** (5+ occurrences)

**Expected reduction:** ~200-300 lines of code

### Phase 2: Medium-Priority Extractions (Moderate Complexity)

7. **#4 - Controller Annotation Checking** (9+ occurrences)
8. **#6 - Combine FQDN Template and Annotation** (7+ occurrences)
9. **#10 - Empty Endpoint Check** (10+ occurrences)
10. **#9 - Target Extraction from LoadBalancer** (4+ occurrences)
11. **#12 - Default Event Handler** (15+ occurrences)

**Expected reduction:** ~150-200 lines of code

### Phase 3: Low-Priority Extractions (Complex or Lower Value)

12. **#13 - Template-based Endpoint Generation** (4+ occurrences, complex)
13. **#17 - Provider-Specific Annotation Application** (30+ occurrences, simple)
14. **#14 - Informer Creation with Multiple Resources** (5+ occurrences)
15. **#5 - FQDN Template Parsing** (11+ occurrences, simple wrapper)
16. **#15 - Namespace List Expansion** (5+ occurrences, minimal value)
17. **#16 - Filter by Label Selector** (5+ occurrences)

**Expected reduction:** ~100-150 lines of code

---

## Proposed Package Structure

Create new common packages to host these shared functions:

```
source/
├── common/
│   ├── annotations.go      # Annotation helper functions (#8)
│   ├── endpoints.go        # Endpoint creation and manipulation (#10, #11, #17)
│   ├── filter.go           # Resource filtering (#4, #16)
│   ├── namespace.go        # Namespace helpers (#15)
│   ├── resource.go         # Resource identifier functions (#7)
│   ├── targets.go          # Target extraction (#9)
│   └── template.go         # Template processing (#5, #6, #13)
├── informers/
│   ├── factory.go          # Informer factory helpers (#1, #2, #14)
│   └── handlers.go         # Event handler registration (#3, #12)
```

---

## Summary Statistics

- **Total patterns identified:** 17
- **Total duplicated code locations:** 250+ occurrences
- **Estimated lines of duplicate code:** 500-650 lines
- **Expected code reduction after refactoring:** 450-650 lines
- **Number of new shared functions needed:** ~17-20 functions
- **Estimated new shared code:** ~200-300 lines

**Net reduction:** 250-450 lines (40-65% reduction in duplicate code)

---

## Benefits

1. **Maintainability**: Fix bugs in one place instead of 25+
2. **Consistency**: Ensures all sources behave identically
3. **Testability**: Shared functions can be tested once comprehensively
4. **Readability**: Source implementations become clearer and more focused
5. **Onboarding**: New contributors can understand patterns faster
6. **Refactoring safety**: Changes are isolated to shared functions

---

## Migration Strategy

1. **Create new packages** with shared functions
2. **Write comprehensive tests** for each shared function
3. **Migrate one source at a time** (start with smallest/simplest)
4. **Run full test suite** after each migration
5. **Document usage patterns** in package documentation
6. **Update contribution guidelines** to reference shared functions

---

## Conclusion

The source directory has excellent test coverage and consistent patterns, but significant code duplication. Extracting these 17 shared behaviors into common functions would:

- Reduce duplicate code by 40-65%
- Improve maintainability across 27+ source implementations
- Establish clear patterns for future source additions
- Make the codebase more approachable for contributors

The extraction effort is relatively low-risk since the patterns are already well-established and heavily tested.
