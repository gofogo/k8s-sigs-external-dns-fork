# Monitoring & Observability

Monitoring is a crucial aspect of maintaining the health and performance of your applications.
It involves collecting, analyzing, and using information to ensure that your system is running smoothly and efficiently. Effective monitoring helps in identifying issues early, understanding system behavior, and making informed decisions to improve performance and reliability.

For `external-dns`, all metrics available for scraping are exposed on the `/metrics` endpoint. The metrics are in the Prometheus exposition format, which is widely used for monitoring and alerting.

To access the metrics:

```sh
curl http://localhost:7979/metrics
```

In the metrics output, you'll see the help text, type information, and current value of the `external_dns_registry_endpoints_total` counter:

```yml
# HELP external_dns_registry_endpoints_total Number of Endpoints in the registry
# TYPE external_dns_registry_endpoints_total gauge
external_dns_registry_endpoints_total 11
```

You can configure a locally running [Prometheus instance](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) to scrape metrics from the application. Here's an example prometheus.yml configuration:

```yml
scrape_configs:
- job_name: external-dns
  scrape_interval: 10s
  static_configs:
  - targets:
    - localhost:7979
```

For more detailed information on how to instrument application with Prometheus, you can refer to the [Prometheus Go client library documentation](https://prometheus.io/docs/guides/go-application/).

## What metrics can I get from ExternalDNS and what do they mean?

- The project maintains a [metrics page](./metrics.md) with a list of all supported custom metrics.
- [Go runtime](https://pkg.go.dev/runtime/metrics#hdr-Supported_metrics) metrics are also available for scraping.

ExternalDNS exposes metrics across six subsystems: `controller`, `source`, `registry`, `provider`, `webhook_provider`, and `http`.

### Source errors

`Source`s are mostly Kubernetes API objects. Examples of `source` errors may be connection errors to the Kubernetes API server itself or missing RBAC permissions.
It can also stem from incompatible configuration in the objects itself like invalid characters, processing a broken fqdnTemplate, etc.

### Registry / Provider errors

`Registry` errors are mostly Provider errors, unless there's some coding flaw in the registry package. Provider errors often arise due to accessing their APIs due to network or missing cloud-provider permissions when reading records.
When applying a changeset, errors will arise if the changeset applied is incompatible with the current state.

The `external_dns_registry_errors_total` metric includes an `operation` label to distinguish between read and write failures:

| `operation` value | Meaning |
|:------------------|:--------|
| `records`         | Failed to read current DNS state from the provider |
| `apply_changes`   | Failed to write a changeset to the provider |

Read failures (`records`) prevent ExternalDNS from knowing the current state and will block the sync loop. Write failures (`apply_changes`) mean the desired state could not be applied.

### Reconciliation metrics

`external_dns_controller_reconcile_duration_seconds{success="true|false"}` is a histogram tracking how long each reconciliation loop takes. A sustained increase indicates a slow provider or growing record set.

`external_dns_controller_changes_total{action, record_type}` counts every DNS record change applied, broken down by action (`create`, `update`, `delete`) and record type (`A`, `AAAA`, `CNAME`, etc.). This is the primary signal for understanding what ExternalDNS is doing each cycle.

`external_dns_controller_filtered_endpoints_total{reason}` counts endpoints dropped before the planning phase:

| `reason` value  | Meaning |
|:----------------|:--------|
| `domain_filter` | Record did not match `--domain-filter` |
| `record_type`   | Record type excluded via `--managed-record-types` / `--exclude-record-types` |

A non-zero value for `domain_filter` is normal when `--domain-filter` is set. An unexpected rise in `record_type` may indicate misconfigured record type filters.

### Source type breakdown

`external_dns_source_endpoints_by_type{source_type, record_type}` shows how many endpoints each configured source type (e.g., `ingress`, `service`, `gateway-httproute`) is currently contributing. This is the first place to look when endpoint counts change unexpectedly after adding or removing a source.

### Provider operation latency

`external_dns_provider_operation_duration_seconds{operation}` is a histogram of the time spent on actual DNS provider API calls, partitioned by operation (`records`, `apply_changes`, `adjust_endpoints`). This is only recorded when the provider cache is enabled (`--provider-cache-time`); cache hits are not measured since no API call is made.

### HTTP request latency

In case of an increased error count, you can correlate errors with `external_dns_http_request_duration_seconds{handler="instrumented_http"}`, which records latency and status codes for all outbound HTTP calls. This covers the DNS provider API (AWS Route53, etc.) as well as the webhook provider.

Use the `host` label to distinguish between the Kubernetes API server (source errors) and the DNS provider API (registry/provider errors). Use the `status` label to distinguish permission errors (`4xx`) from provider outages (`5xx`).

## Owner Mismatch Metrics

The `external_dns_registry_skipped_records_owner_mismatch_per_sync` metric tracks DNS records that were skipped during synchronization because they are owned by a different ExternalDNS instance. This is useful for detecting ownership conflicts in multi-tenant or multi-instance deployments.

The metric includes the following labels:

| Label           | Description                                      |
|:----------------|:-------------------------------------------------|
| `record_type`   | DNS record type (A, AAAA, CNAME, etc.)           |
| `owner`         | The owner ID of the current ExternalDNS instance |
| `foreign_owner` | The owner ID found on the existing record        |
| `domain`        | The naked/apex domain (e.g., "example.com")      |

**Note:** The `domain` label uses the naked/apex domain rather than the full FQDN to prevent metric cardinality explosion. With thousands of subdomains under one apex domain, using full FQDNs would create excessive metric series.

## Metrics Best Practices

When scraping ExternalDNS metrics, consider the following best practices:

### Cardinality Management

- **Vector metrics** (those with labels like `record_type`, `source_type`, `domain`) can generate multiple time series. Monitor your Prometheus storage and memory usage accordingly.
- The `domain` label on owner mismatch metrics is intentionally limited to apex domains to bound cardinality.
- Use recording rules to pre-aggregate high-cardinality metrics if you only need totals.

### Recommended Scrape Interval

- A scrape interval of 10-30 seconds is typically sufficient for ExternalDNS metrics.
- Align your scrape interval with ExternalDNS's sync interval (`--interval` flag) for meaningful data.

### Alerting Recommendations

Consider alerting on:

- `rate(external_dns_source_errors_total[5m]) > 0` — source connectivity or RBAC issues.
- `rate(external_dns_registry_errors_total{operation="records"}[5m]) > 0` — provider read failures; ExternalDNS cannot determine current DNS state.
- `rate(external_dns_registry_errors_total{operation="apply_changes"}[5m]) > 0` — provider write failures; desired state is not being applied.
- `external_dns_controller_consecutive_soft_errors > 0` — repeated transient failures; increasing values indicate a persistent problem.
- `time() - external_dns_controller_last_sync_timestamp_seconds > 2 * <interval>` — sync loop appears stuck.
- `rate(external_dns_controller_changes_total{action="delete"}[5m]) > <threshold>` — unexpected mass deletion of DNS records.
- `external_dns_registry_skipped_records_owner_mismatch_per_sync > 0` — ownership conflicts in multi-instance deployments.

## Resources

- [Prometheus Instrumentation](https://prometheus.io/docs/practices/instrumentation/)
- [Prometheus Alerting Best Practices](https://prometheus.io/docs/practices/alerting/)
- [Prometheus Recording Rules](https://prometheus.io/docs/practices/rules/)
- [Grafana: How to Manage High Cardinality Metrics](https://grafana.com/blog/2022/02/15/what-are-cardinality-spikes-and-why-do-they-matter/)
