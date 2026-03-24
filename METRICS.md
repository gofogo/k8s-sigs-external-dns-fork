# Prometheus Metrics Analysis

## Existing Metrics

| Subsystem | Metrics |
|---|---|
| `controller` | `last_sync_timestamp_seconds`, `last_reconcile_timestamp_seconds`, `no_op_runs_total`, `consecutive_soft_errors`, `verified_records{record_type}` |
| `registry` | `errors_total`, `endpoints_total`, `records{record_type}`, `skipped_records_owner_mismatch_per_sync{…}` |
| `source` | `errors_total`, `endpoints_total`, `records{record_type}` |
| `provider` | `cache_records_calls{from_cache}`, `cache_apply_changes_calls` |
| `webhook_provider` | `records_{requests,errors}_total`, `applychanges_{requests,errors}_total`, `adjustendpoints_{requests,errors}_total` |
| `http` | `request_duration_seconds` (SummaryVec) |
| _(global)_ | `build_info` |

---

## Missing Metrics — Suggestions

### 1. Plan change counts per sync (highest value, very easy)

The plan already computes `Changes.Create`, `Changes.UpdateNew`, `Changes.Delete` in `controller/controller.go:118-131`, but their sizes are never recorded.

```
external_dns_controller_changes_total{action="create|update|delete", record_type="A|CNAME|…"}
```

**Type:** CounterVec
**Where:** `controller.go` after `plan.Calculate()`, before `ApplyChanges`.
**Value:** Lets you alert on unexpected mass-deletes, track churn rate, audit provider cost (API calls correlate with change counts).

---

### 2. Reconciliation loop duration (high value, standard SRE practice)

`RunOnce` has no wall-clock histogram. There is a `last_reconcile` → `last_sync` gap you can compute externally, but it's lossy.

```
external_dns_controller_reconcile_duration_seconds (Histogram)
```

**Labels:** none (or `{success="true|false"}`)
**Where:** wrap the body of `RunOnce` with `time.Since`.
**Value:** Detect slow providers, correlate with rate-limit back-off.

---

### 3. DNS provider operation duration (medium value)

Only AWS gets HTTP-level timing via the instrumented round-tripper. All other providers (Azure, Cloudflare, Route53 non-HTTP path, etc.) are invisible.

```
external_dns_provider_operation_duration_seconds{operation="records|apply_changes|adjust_endpoints"} (Histogram)
```

**Where:** Best added in the `CachedProvider` wrapper (already wraps every provider) or as a generic provider middleware similar to how `cached_provider.go` wraps calls.
**Value:** Provider latency SLOs, timeout tuning.

---

### 4. Cache age / TTL utilization (low effort, medium value)

`cached_provider.go` tracks hit/miss but not how fresh the cache is or how long until expiry.

```
external_dns_provider_cache_age_seconds (Gauge)        # time since last fill
external_dns_provider_cache_ttl_seconds  (Gauge)       # configured TTL
```

**Where:** `cached_provider.go` — set `cache_age_seconds` when the cache is populated, expose `ttl_seconds` as a constant gauge at startup.
**Value:** Distinguish "cache is old because provider is slow" from "cache was just filled".

---

### 5. Source-type breakdown (medium value)

`source_records` and `source_endpoints_total` are aggregated across **all** sources. If you have both `ingress` and `gateway-httproute` sources configured, you cannot tell which contributes what.

```
external_dns_source_records{record_type="A", source_type="ingress|service|gateway-httproute|…"}
```

**Where:** `wrapper.Build` (recently introduced as a single entry point in `refactor(wrappers)`) is the right interception point — wrap each source with a labelled counter before composing them.
**Value:** Debug why endpoint counts change after adding/removing a source type.

---

### 6. `ApplyChanges` errors split from `Records` errors (medium value)

`registry_errors_total` is incremented on both `Records()` failure and `ApplyChanges()` failure (same counter, `controller.go:78` vs `controller.go:123`). You can't tell them apart.

```
external_dns_registry_records_errors_total       (Counter)
external_dns_registry_apply_changes_errors_total (Counter)
```

or add an `operation="records|apply_changes"` label to `registry_errors_total`.
**Value:** Distinguish "can't read DNS state" (read error) from "failed to write changes" (write error) — very different failure modes.

---

### 7. Filtered/excluded endpoint count (low value, but useful for debugging)

The plan filters endpoints by `DomainFilter` and `ExcludeRecords` before computing changes. These filtered-out endpoints are silent today.

```
external_dns_controller_filtered_endpoints_total{reason="domain_filter|exclude_record_type|policy"}
```

**Where:** `plan.Calculate()` / `filterRecordsForPlan()`.
**Value:** Helps diagnose "why isn't my record being created" — the common operator debugging question.

---

### 8. Webhook provider request latency (low-hanging fruit)

The webhook provider tracks request/error counts but not latency — unlike the HTTP instrumented client used by AWS.

```
external_dns_webhook_provider_request_duration_seconds{method="records|apply_changes|adjust_endpoints"} (Histogram)
```

**Where:** `provider/webhook/webhook.go` alongside existing request counters.
**Value:** Detect slow webhook backends before timeouts occur.

---

## Priority Order

| Priority | Metric | Reason |
|---|---|---|
| 1 | `controller_changes_total{action, record_type}` | Already have the data in `plan.Changes`; minimal code change; high operational value |
| 2 | `controller_reconcile_duration_seconds` | Standard SRE histogram; one `time.Since` call in `RunOnce` |
| 3 | `registry_errors_total` split by `operation` | Current counter conflates two very different failure modes |
| 4 | `provider_operation_duration_seconds` | Fills the biggest observability blind spot for non-AWS providers |
| 5 | `source_records` with `source_type` label | Requires source wrapper instrumentation, slightly more invasive |
| 6 | `provider_cache_age_seconds` | Nice-to-have for cache debugging |
| 7 | `webhook_provider_request_duration_seconds` | Parity with HTTP-instrumented providers |
| 8 | `controller_filtered_endpoints_total` | Useful but mostly a debugging aid |
