# RFC2136 Provider with BIND9

This tutorial demonstrates how to configure ExternalDNS with an RFC2136-compatible DNS server such as BIND9.
The RFC2136 provider allows ExternalDNS to perform dynamic DNS updates using the DNS UPDATE protocol — ideal for self-hosted DNS zones.

- [RFC2136 spec](https://datatracker.ietf.org/doc/html/rfc2136)
- [BIND9](https://www.isc.org/bind/)
- [BIND9 docs](https://bind9.readthedocs.io/en/latest/chapter1.html)
- [BIND9 deep-dive](https://hostman.com/tutorials/setting-up-a-bind-dns-server/)
- [BIND9 do deep-dive](https://www.digitalocean.com/community/tutorials/how-to-configure-bind-as-a-private-network-dns-server-on-ubuntu-14-04)

### TL;DR

After completing this lab, you will have a Kubernetes environment running as containers in your local development machine with BIND9 and external-dns

## Notes

- **RFC2136** updates are atomic, sent via DNS UPDATE messages (port 53 TCP/UDP).
- Always use **TSIG* for authentication in production.
- Ensure your DNS server zone file is writable (`/var/lib/bind/db.example.org` in this example).
- For external queries (from host), expose BIND9 via NodePort or hostPort if needed.
- This setup runs entirely inside kind, making it ideal for local testing or CI pipelines.

## Prerequisite

Before you start, ensure you have:

- A running kubernetes cluster.
  - In this tutorial we are going to use [kind](https://kind.sigs.k8s.io/)
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) and [`helm`](https://helm.sh/)

## Bootstrap Environment

### 1. Create cluster

```sh
kind create cluster --config=docs/snippets/tutorials/rfc2136/kind.yaml

Creating cluster "rfc2136" ...
 ✓ Ensuring node image (kindest/node:v1.33.0) 🖼
 ✓ Preparing nodes 📦 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
 ✓ Joining worker nodes 🚜
Set kubectl context to "rfc2136"
You can now use your cluster with:

kubectl cluster-info --context rfc2136
```

### 2. Deploy BIND9 and Authoritative DNS Server

In this example, we’ll run a standalone BIND9 Pod in the default namespace.
It hosts the example.org zone and allows dynamic updates from ExternalDNS authenticated via TSIG.

```sh
kubectl apply -f docs/snippets/tutorials/rfc2136/bind9.yaml

kubectl rollout status deploy/bind9
❯❯ deployment "bind9" successfully rolled out

kubectl logs deploy/bind9 | grep "zone example" -B 2 -A 3

❯❯ managed-keys-zone: loaded serial 0
❯❯ zone example.org/IN: loaded serial 2025010101
❯❯ all zones loaded

kubectl exec deploy/bind9 -c bind9 -- cat /var/lib/bind/db.example.org
kubectl exec deploy/bind9 -c bind9 -- cat /etc/bind/named.conf.local
kubectl exec deploy/bind9 -c bind9 -- cat /etc/default/named

kubectl exec deploy/bind9 -c bind9 -- cat docker-entrypoint.sh
```

Verify DNS resolution

```sh
kubectl run -it --rm dnsutils --image=infoblox/dnstools --restart=Never

dig +short @bind9.default.svc.cluster.local example.org SOA
❯❯ ns1.example.org. hostmaster.example.org. 2025010101 60 30 1209600 60

kubectl exec deploy/bind9 -c bind9 -- ss -lntup
❯❯ udp   UNCONN 0      0      0.0.0.0:53     0.0.0.0:*     users:(("named",pid=1,fd=6))
❯❯ tcp   LISTEN 0      10     0.0.0.0:53     0.0.0.0:*     users:(("named",pid=1,fd=7))
```

### 3. Configure ExternalDNS

Deploy with helm and minimal configuration.

Add the `external-dns` helm repository and check available versions

```sh
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update
helm search repo external-dns --versions
```

Install with required configuration

```sh
helm upgrade --install external-dns external-dns/external-dns \
  -f docs/snippets/tutorials/rfc2136/values-extdns.yaml \
  -n default

❯❯ Release "external-dns" does not exist. Installing it now.

kubectl rollout status deploy/external-dns
❯❯ deployment "external-dns" successfully rolled out
```

Validate external-dns and rfc configuration

```sh
❯❯ kubectl logs deploy/external-dns | grep "Configured RFC2136"

time="2025-10-26T12:46:58Z" level=info msg="Configured RFC2136 with zone '[example.org.]' and nameserver 'bind9.default.svc.cluster.local:53'"
```

### 3. Configure Test Services

Apply manifest with

```sh
kubectl apply -f docs/snippets/tutorials/rfc2136/fixtures.yaml

kubectl get svc -l svc=test-svc-rfc2136

❯❯ NAME               TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)        AGE
❯❯ a-svc-rfc2136      LoadBalancer   10.96.12.98     <pending>     80:30766/TCP   116s
❯❯ aa-svc-rfc2136     LoadBalancer   10.96.186.64    <pending>     80:31816/TCP   43s
```

Patch services, to manually assign an Ingress IPs. It just makes the Service appear like a real LoadBalancer for tools/tests.

```sh
kubectl patch svc a-svc-rfc2136 --type=merge \
 -p '{"status":{"loadBalancer":{"ingress":[{"ip":"172.18.0.2"}]}}}' \
  --subresource=status
❯❯ service/a-svc-rfc2136 patched

kubectl patch svc aa-svc-rfc2136 --type=merge \
 -p '{"status":{"loadBalancer":{"ingress":[{"ip":"2001:db8::1"}]}}}' \
  --subresource=status
❯❯ service/aa-svc-rfc2136 patched

kubectl get svc -l svc=test-svc-rfc2136

kubectl get svc -l svc=test-svc-rfc2136

❯❯ NAME             TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)        AGE
❯❯ a-svc-rfc2136    LoadBalancer   10.96.12.98    172.18.0.2    80:30766/TCP   3m25s
❯❯ aa-svc-rfc2136   LoadBalancer   10.96.186.64   2001:db8::1   80:31816/TCP   2m12s
```

### 5. Test DNS resolution via BIND9

Launch a debug pod:

```sh
kubectl run --rm -it dnsutils --image=infoblox/dnstools --restart=Never
```

Run with expected output

```sh
dig +short @bind9.default.svc.cluster.local a.example.org

```
