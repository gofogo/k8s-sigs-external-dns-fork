---
tags:
  - sources
  - fake
  - testing
---

# Fake Source

The fake source generates synthetic DNS endpoints without requiring a Kubernetes cluster or any real resources. It produces one endpoint per supported record type on every reconciliation cycle, using documentation-reserved address ranges (RFC 5737 for IPv4, RFC 3849 for IPv6).

## Use cases

**Dry-running a DNS provider**
Validate provider credentials, API connectivity, and record formatting before pointing ExternalDNS at a real cluster:

```console
external-dns --source=fake --provider=aws --domain-filter=example.com --dry-run
```

**Testing a webhook provider**
Webhook providers developed by third parties can use the fake source to verify that their implementation handles every supported record type correctly, without needing a Kubernetes cluster with CRDs or Services:

```console
external-dns --source=fake --provider=webhook --webhook-provider-url=http://localhost:8888
```

**CI integration tests**
Spin up ExternalDNS with `--source=fake` in a pipeline to exercise provider logic end-to-end without any cluster dependency.

## Generated endpoints

On each reconciliation the fake source emits one endpoint per record type:

| Record type | DNS name                   | Example target                                                |
|:------------|:---------------------------|:--------------------------------------------------------------|
| `A`         | `<random>.example.com`     | `192.0.2.1`                                                   |
| `AAAA`      | `<random>.example.com`     | `2001:db8::1a2b:3c4d`                                         |
| `CNAME`     | `<random>.example.com`     | `<random>.example.com`                                        |
| `TXT`       | `<random>.example.com`     | `"heritage=external-dns,external-dns/owner=fake"`             |
| `SRV`       | `_sip._udp.example.com`    | `10 20 5060 <random>.example.com.`                            |
| `NS`        | `example.com`              | `<random>.example.com`                                        |
| `PTR`       | `<n>.2.0.192.in-addr.arpa` | `<random>.example.com`                                        |
| `MX`        | `example.com`              | `10 <random>.example.com`                                     |
| `NAPTR`     | `_sip._udp.example.com`    | `100 10 "u" "E2U+sip" "!^.*$!sip:info@example.com!" .`        |

The NAPTR target fields are: `order preference flags service regexp replacement` (RFC 2915). In the example above: order=100, preference=10, flags=`"u"` (URI result), service=`"E2U+sip"`, regexp=`"!^.*$!sip:info@example.com!"`, replacement=`.` (none).

IPv4 addresses are drawn from `192.0.2.0/24` and IPv6 from `2001:db8::/32` — both reserved for documentation and examples, so they will never accidentally match real infrastructure.

## Enabling specific record types

By default ExternalDNS only manages `A` and `CNAME` records. Use `--managed-record-types` to opt in to additional types:

```console
external-dns \
  --source=fake \
  --provider=webhook \
  --webhook-provider-url=http://localhost:8888 \
  --managed-record-types=A \
  --managed-record-types=AAAA \
  --managed-record-types=CNAME \
  --managed-record-types=TXT \
  --managed-record-types=SRV \
  --managed-record-types=NS \
  --managed-record-types=MX \
  --managed-record-types=NAPTR
```

To test all record types at once, list every type explicitly. The fake source always generates a full set; `--managed-record-types` controls which ones the provider receives.

## Kubernetes events

When `--emit-events` is configured, the fake source emits Kubernetes events for every DNS change, referencing a synthetic `Pod` object in the `default` namespace. This lets you observe the event stream during testing without real workloads:

```console
external-dns --source=fake --provider=webhook --webhook-provider-url=http://localhost:8888 \
  --emit-events=RecordReady
```

```console
kubectl get events -n default --field-selector reason=RecordReady
```

## Custom domain with `--fqdn-template`

By default the fake source generates endpoints under `example.com`. Use `--fqdn-template` to replace it with your own domain. The template is rendered against a synthetic `Pod` object with `Name=fake` and `Namespace=fake`.

```console
# plain domain
external-dns --source=fake --provider=webhook --webhook-provider-url=http://localhost:8888 \
  --fqdn-template=my-company.com

# template expression
external-dns --source=fake --provider=webhook --webhook-provider-url=http://localhost:8888 \
  --fqdn-template={{.Name}}.my-company.com
```

The second example renders to `fake.my-company.com`, so endpoints are generated as `<random>.fake.my-company.com`.

## Limitations

- Endpoints are regenerated with random names on every reconciliation; the fake source does not model resource updates or deletions.
- No Kubernetes cluster is required, but `--emit-events` will fail to post events if no cluster API server is reachable.
