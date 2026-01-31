# Integration Tests Plan

## Overview

Create integration tests that wire up real sources (ingress, service) with fake Kubernetes clients and real wrappers.

## Directory Structure

```
tests/integration/
├── source_integration_test.go
└── scenarios/
    └── integration_test.yaml
```

## Flow

Fake K8s Client → Real Source → Real Wrappers → Validate Endpoints

## Pros

- Tests full pipeline without mocking sources

## Cons

- Still uses fake K8s client, not truly end-to-end

## YAML Schema

| Field                      | Type     | Description                              |
|----------------------------|----------|------------------------------------------|
| name                       | string   | Test scenario name                       |
| sources                    | []string | Sources to create: ingress, service      |
| config.defaultTargets      | []string | --default-targets flag values            |
| config.forceDefaultTargets | bool     | --force-default-targets flag             |
| resources                  | []object | K8s resources (Ingress, Service)         |
| expected                   | []object | Expected endpoints                       |

## Test Scenarios

All scenarios defined in `scenarios/integration_test.yaml` (13 scenarios):

### Ingress Source (6 scenarios)

| Name | Status | --default-targets | --force | Expected |
|------|--------|-------------------|---------|----------|
| ingress-with-ip-no-defaults | IP: 1.2.3.4 | [] | false | [1.2.3.4] |
| ingress-with-ip-defaults-not-forced | IP: 1.2.3.4 | [10.0.0.1] | false | [1.2.3.4] |
| ingress-with-ip-defaults-forced | IP: 1.2.3.4 | [10.0.0.1] | true | [10.0.0.1] |
| ingress-empty-status-defaults-applied | empty | [10.0.0.1] | false | [10.0.0.1] |
| ingress-with-hostname-defaults-not-forced | hostname: lb.example.com | [10.0.0.1] | false | [lb.example.com] |
| ingress-with-hostname-defaults-forced | hostname: lb.example.com | [10.0.0.1] | true | [10.0.0.1] |

### Service Source (6 scenarios)

| Name | Type | Status | --default-targets | --force | Expected |
|------|------|--------|-------------------|---------|----------|
| service-loadbalancer-with-ip-no-defaults | LoadBalancer | IP: 1.2.3.4 | [] | false | [1.2.3.4] |
| service-loadbalancer-with-ip-defaults-not-forced | LoadBalancer | IP: 1.2.3.4 | [10.0.0.1] | false | [1.2.3.4] |
| service-loadbalancer-with-ip-defaults-forced | LoadBalancer | IP: 1.2.3.4 | [10.0.0.1] | true | [10.0.0.1] |
| service-loadbalancer-pending-defaults-applied | LoadBalancer | empty | [10.0.0.1] | false | [10.0.0.1] |
| service-externalname-defaults-not-forced | ExternalName | external.example.com | [10.0.0.1] | false | [external.example.com] |
| service-externalname-defaults-forced | ExternalName | external.example.com | [10.0.0.1] | true | [10.0.0.1] |

### Mixed Sources (1 scenario)

| Name | Sources | --default-targets | --force | Expected |
|------|---------|-------------------|---------|----------|
| mixed-sources-with-forced-defaults | ingress, service | [10.0.0.1] | true | [10.0.0.1] for both |

## Implementation Status

### Completed

- [x] Created `tests/integration/scenarios/integration_test.yaml` with 13 test scenarios
- [x] Created `tests/integration/toolkit.go` with YAML parsing and helper functions
- [x] Created `tests/integration/source_integration_test.go` test runner
- [x] YAML parsing verified working (resources parse correctly with annotations and status)

### In Progress

- [ ] Fix informer cache sync issue - sources return 0 endpoints

### Blocking Issue

The fake K8s client + informer integration is not working as expected. When resources are created via the fake client API before creating sources, the informer cache does not see them.

**Verified working:**

- YAML parsing correctly extracts Ingress with annotations and status
- Resources created via `client.NetworkingV1().Ingresses().Create()` are retrievable via `List()`

**Not working:**

- Source's informer-based `Endpoints()` returns empty results

**Next steps to investigate:**

1. Compare with existing `source/ingress_test.go` pattern more closely
2. Check if informer needs explicit start/sync handling different from what `NewIngressSource` does internally
3. Consider if fake client requires different setup for informer watchers

## Future Considerations

### ScenarioConfig vs source.Config

Currently using a simple `ScenarioConfig` struct with only `DefaultTargets` and `ForceDefaultTargets`. Plan to support more `source.Config` fields in the future.

**Options:**

1. **Use `source.Config` directly** - Problem: `sources` field is private, `LabelFilter` is `labels.Selector` (doesn't YAML-serialize cleanly), has internal fields (`clientGen`, `clientGenOnce`)

2. **Create a YAML-friendly config struct that converts to `source.Config`** - Have a `ScenarioSourceConfig` with string-based fields (e.g., `LabelFilter string`) and a `ToSourceConfig()` method that converts it. Keeps YAML simple while allowing full `source.Config` support.

3. **Extend `source.Config` to be YAML-friendly** - Add JSON/YAML tags to `source.Config`, export `sources` field, add custom unmarshaler for `labels.Selector`. Requires changes to the source package.

4. **Use `map[string]interface{}` in YAML** - Parse config as generic map, then programmatically build `source.Config`. Flexible but loses type safety in YAML.

**Recommendation:** Option 2 is cleanest - maintains separation between YAML schema and internal types.
