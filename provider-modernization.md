# Provider Package Modernization Plan

This document outlines code duplication patterns and improvement opportunities identified in the provider package.

## Status: Completed Items

### Phase 1: Low-Hanging Fruit (COMPLETED)

#### 1. Configuration-Based Record Type Support

- **File**: `provider/record_type_config.go`
- **Status**: DONE
- Created `RecordTypeConfig` type with predefined configs:
  - `DefaultRecordTypeConfig` - base types only
  - `MXRecordTypeConfig` - base + MX
  - `MXNAPTRRecordTypeConfig` - base + MX + NAPTR (AWS)
- Updated providers: AWS, Azure, Google, DigitalOcean, Cloudflare

#### 2. Standardized Error Utilities

- **File**: `provider/errors.go`
- **Status**: DONE
- Created error helpers:
  - `SoftErrorZones(err)` / `SoftErrorZonesWithContext(err, context)`
  - `SoftErrorRecords(err)` / `SoftErrorRecordsForZone(err, zone)`
  - `SoftErrorApplyChanges(err)` / `SoftErrorApplyChangesForZones(zones)`
  - `SoftErrorTags(err)`
- Updated providers: AWS, Azure, Google, OCI, PDNS, Cloudflare

#### 3. Tests

- **Files**: `provider/record_type_config_test.go`, `provider/errors_test.go`
- **Status**: DONE

#### 4. Type Converters

- **File**: `provider/type_converters.go`
- **Status**: DONE
- Created safe parsing utilities:
  - `ParseInt64(value, defaultVal, context)` - int64 with logging on error
  - `ParseInt32(value, defaultVal, context)` - int32 with logging on error
  - `ParseUint32(value, defaultVal, context)` - uint32 with logging on error
  - `ParseFloat64(value, defaultVal, context)` - float64 with logging on error
  - `ParseBool(value, defaultVal, context)` - bool with logging on error
  - `ParseInt64OrError(value)` - int64 with error return
  - `ParseFloat64OrError(value)` - float64 with error return
- Updated providers:
  - AWS: weight, bias, coordinates parsing
  - Scaleway: page size, priority parsing
  - Azure: MX preference parsing
  - Cloudflare: proxied annotation parsing
  - Godaddy: retry-after header parsing
  - Pihole: TTL parsing
  - Akamai: max body, debug flag parsing

#### 5. Generic Zone Cache

- **File**: `provider/zone_cache.go`
- **Status**: DONE
- Created generic, thread-safe zone caching utilities:
  - `ZoneCache[T]` - generic cache with TTL expiration
  - `NewSliceZoneCache[T]()` - cache for slice types
  - `NewMapZoneCache[K, V]()` - cache for map types
  - `CachedZoneProvider[T]` - wrapper that combines cache with fetcher
- Tests: `provider/zone_cache_test.go`
- Migrated providers:
  - AWS: Uses `NewMapZoneCache[string, *profiledZone]` for zone caching
  - Azure: Uses `NewSliceZoneCache[dns.Zone]` and `NewSliceZoneCache[privatedns.PrivateZone]`
    - Deleted `provider/azure/cache.go` and `provider/azure/cache_test.go` (redundant with shared cache)
- Available for new providers or future refactoring of existing providers

---

## 1. Code Duplication Patterns

### 1.1 SupportedRecordType (16 providers)

The same delegation pattern is repeated across 16 providers:

```go
// Example: azure/azure.go:203-210
func (p *AzureProvider) SupportedRecordType(recordType string) bool {
    switch recordType {
    case "MX":
        return true
    default:
        return provider.SupportedRecordType(recordType)
    }
}
```

**Affected providers:** akamai, alibabacloud, aws, azure, civo, cloudflare, digitalocean, gandi, godaddy, google, linode, ns1, oci, ovh, scaleway, transip

**Locations:**

- `provider/aws/aws.go:1396-1403`
- `provider/azure/azure.go:203-210`
- `provider/google/google.go:248-255`

### 1.2 Zone Retrieval & Caching (20+ providers)

Each provider implements 60-100 lines of similar zone retrieval logic with caching:

- `provider/aws/aws.go:372-480` - Uses `zonesListCache` with TTL
- `provider/azure/azure.go:176-200` - Uses `zonesCache[dns.Zone]`
- `provider/google/google.go:168-179` - Similar filtering pattern

Core pattern: "get zones -> filter by domain/ID/type/tags -> cache results"

### 1.3 Record Pagination Loop (20+ providers)

Identical structure across providers:

```go
// AWS: provider/aws/aws.go:492-506
paginator := route53.NewListResourceRecordSetsPaginator(...)
for paginator.HasMorePages() {
    resp, err := paginator.NextPage(ctx)
    if err != nil {
        return nil, provider.NewSoftErrorf("failed to list...: %w", err)
    }
    for _, r := range resp.ResourceRecordSets {
        if !p.SupportedRecordType(r.Type) {
            continue
        }
        // ... record processing
    }
}

// Azure: provider/azure/azure.go:118-124
pager := p.recordSetsClient.NewListAllByDNSZonePager(...)
for pager.More() {
    nextResult, err := pager.NextPage(ctx)
    if err != nil {
        return nil, provider.NewSoftErrorf("failed to fetch dns records: %w", err)
    }
    for _, recordSet := range nextResult.Value {
        if !p.SupportedRecordType(recordType) {
            continue
        }
        // ... record processing
    }
}
```

### 1.4 Inconsistent Error Handling

Different error message formatting patterns with inconsistent use of `%w` vs `%v`:

```go
// AWS uses both:
return nil, provider.NewSoftErrorf("failed to list hosted zones: %w", err)
return nil, provider.NewSoftErrorf("records retrieval failed: %v", err)

// Azure sometimes returns raw errors:
return nil, err
```

### 1.5 ApplyChanges Structure (20+ providers)

All follow "get zones -> map changes -> submit" but with inconsistent error handling:

- `provider/aws/aws.go:683-697` - Wraps zone errors as soft errors
- `provider/azure/azure.go:164-174` - Returns raw errors
- `provider/google/google.go:208-219` - Different change aggregation

## 2. Improvement Opportunities

### 2.1 Configuration-Based Record Type Support

Replace 16 identical method implementations with configuration:

```go
// provider/record_type_config.go
type RecordTypeConfig struct {
    BaseSupported []string // A, AAAA, CNAME, SRV, TXT, NS
    Additional    []string // MX, NAPTR, etc.
}

func (c RecordTypeConfig) Supports(recordType string) bool {
    // Implementation handles both base + additional
}

// Usage in provider:
var supportedTypes = RecordTypeConfig{
    BaseSupported: []string{"A", "AAAA", "CNAME", "SRV", "TXT", "NS"},
    Additional:    []string{"MX"},
}

func (p *AWSProvider) SupportedRecordType(recordType string) bool {
    return supportedTypes.Supports(recordType)
}
```

**Impact:** Reduces ~15 lines per provider to ~5 lines

### 2.2 Generic Zone Manager with Caching

Abstract common zone retrieval pattern:

```go
// provider/zone_manager.go
type ZoneManager[Z any] interface {
    GetZones(ctx context.Context) ([]Z, error)
    GetZonesFiltered(ctx context.Context, filters ZoneFilters) ([]Z, error)
}

type CachedZoneManager[Z any] struct {
    fetcher func(ctx context.Context) ([]Z, error)
    filter  func(Z, ZoneFilters) bool
    cache   []Z
    ttl     time.Duration
    expire  time.Time
}

func (m *CachedZoneManager[Z]) GetZones(ctx context.Context) ([]Z, error) {
    if time.Now().Before(m.expire) && m.cache != nil {
        return m.cache, nil
    }
    zones, err := m.fetcher(ctx)
    if err != nil {
        return nil, err
    }
    m.cache = zones
    m.expire = time.Now().Add(m.ttl)
    return zones, nil
}
```

**Impact:** ~100 lines per provider x 20 providers = ~2000 lines reduction

### 2.3 Standardized Error Utilities

Create consistent error wrapping:

```go
// provider/errors.go
func WrapZoneRetrievalError(err error, provider string) error {
    return NewSoftErrorf("failed to list zones for %s: %w", provider, err)
}

func WrapRecordRetrievalError(err error, zone string) error {
    return NewSoftErrorf("failed to retrieve records for zone %s: %w", zone, err)
}

func WrapApplyChangesError(err error, operation string) error {
    return NewSoftErrorf("failed to %s: %w", operation, err)
}
```

### 2.4 Generic Pagination Helper

Abstract the pagination pattern:

```go
// provider/pagination.go
type Paginator[T any] interface {
    HasMore() bool
    Next(ctx context.Context) ([]T, error)
}

func PaginateAndFilter[T any](
    ctx context.Context,
    pager Paginator[T],
    filter func(T) bool,
) ([]T, error) {
    var results []T
    for pager.HasMore() {
        page, err := pager.Next(ctx)
        if err != nil {
            return nil, err
        }
        for _, item := range page {
            if filter(item) {
                results = append(results, item)
            }
        }
    }
    return results, nil
}
```

### 2.5 Type Conversion Utilities

Replace scattered conversion logic:

```go
// provider/type_converters.go
func SafeParseInt64(value string, defaultVal int64, context string) int64 {
    val, err := strconv.ParseInt(value, 10, 64)
    if err != nil {
        log.Errorf("Failed parsing %s: %s: %v; using default %d", context, value, err, defaultVal)
        return defaultVal
    }
    return val
}

func SafeParseFloat64(value string, defaultVal float64, context string) float64 {
    val, err := strconv.ParseFloat(value, 64)
    if err != nil {
        log.Errorf("Failed parsing %s: %s: %v; using default %f", context, value, err, defaultVal)
        return defaultVal
    }
    return val
}
```

**Current location:** `provider/aws/aws.go:962-966` and similar patterns elsewhere

### 2.6 Test Helper Package

Large test files with repeated patterns:

- `provider/aws/aws_test.go` - 2,886 lines
- `provider/cloudflare/cloudflare_test.go` - 3,658 lines
- `provider/azure/azure_test.go` - 614 lines

```go
// provider/testutil/helpers.go
func AssertRecordsMatch(t *testing.T, got, want []*endpoint.Endpoint)
func AssertApplyChangesSucceeds(t *testing.T, p Provider, changes *plan.Changes)
func AssertDomainFilterWorks(t *testing.T, p Provider, included, excluded []string)
func NewTestEndpoint(name, recordType string, targets ...string) *endpoint.Endpoint
```

## 3. Architectural Inconsistencies

### 3.1 Zone Return Types

Different providers return zones in different formats:

- AWS: `map[string]*profiledZone`
- Azure: `[]dns.Zone`
- Google: `map[string]*dns.ManagedZone`

**Recommendation:** Create common internal zone representation

### 3.2 Change Batching Strategies

- AWS: Accumulates changes and submits in batches with size limits
- Azure: Immediately applies changes as they're mapped
- Google: Collects additions and deletions separately

**Recommendation:** Document why differences exist or standardize approach

### 3.3 Error Return Patterns

- AWS wraps as soft error in ApplyChanges
- Azure returns raw error

**Recommendation:** Establish consistent error policy in Provider interface

## 4. Refactoring Priority

| Priority | Change | Est. Code Reduction | Complexity |
|----------|--------|---------------------|------------|
| 1 | Zone manager abstraction | ~2000 lines | High |
| 2 | Error wrapping utilities | ~300 lines | Low |
| 3 | Pagination helpers | ~600 lines | Medium |
| 4 | Record type config | ~150 lines | Low |
| 5 | Test helpers | ~1500 lines | Medium |
| 6 | Type converters | ~100 lines | Low |

## 5. Implementation Phases

### Phase 1: Low-Hanging Fruit

1. Create `provider/errors.go` with standardized error utilities
2. Create `provider/record_type_config.go` and migrate providers
3. Create `provider/type_converters.go`

### Phase 2: Zone Management

1. Design `ZoneManager` interface with generics
2. Implement `CachedZoneManager`
3. Migrate AWS provider as pilot
4. Roll out to remaining providers

### Phase 3: Pagination

1. Create generic `Paginator` interface
2. Implement `PaginateAndFilter` helper
3. Create adapter implementations for AWS, Azure, Google SDK paginators
4. Migrate providers

### Phase 4: Testing

1. Create `provider/testutil` package
2. Extract common test fixtures
3. Create assertion helpers
4. Refactor provider tests to use shared utilities

## 6. Files to Create

```
provider/
  errors.go              # Standardized error wrapping
  record_type_config.go  # Configuration-based record type support
  type_converters.go     # Safe parsing utilities
  zone_manager.go        # Generic zone caching
  pagination.go          # Generic pagination helpers
  testutil/
    helpers.go           # Test assertion helpers
    fixtures.go          # Common test data
```

## 7. Migration Strategy

1. Add new utilities alongside existing code
2. Migrate one provider at a time (start with AWS as reference)
3. Run full test suite after each provider migration
4. Remove old code once all providers are migrated
