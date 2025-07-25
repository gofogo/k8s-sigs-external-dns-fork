# Scaleway

This tutorial describes how to setup ExternalDNS for usage within a Kubernetes cluster using Scaleway DNS.

Make sure to use **>=0.7.4** version of ExternalDNS for this tutorial.

**Warning**: Scaleway DNS is currently in Public Beta and may not be suited for production usage.

## Importing a Domain into Scaleway DNS

In order to use your domain, you need to import it into Scaleway DNS. If it's not already done, you can follow [this documentation](https://www.scaleway.com/en/docs/scaleway-dns/)

Once the domain is imported you can either use the root zone, or create a subzone to use.

In this example we will use `example.com` as an example.

## Creating Scaleway Credentials

To use ExternalDNS with Scaleway DNS, you need to create an API token (composed of the Access Key and the Secret Key).
You can either use existing ones or you can create a new token, as explained in [How to generate an API token](https://www.scaleway.com/en/docs/generate-an-api-token/) or directly by going to the [credentials page](https://console.scaleway.com/account/organization/credentials).

Scaleway provider supports configuring credentials using profiles or supplying it directly with environment variables.

### Configuration using a config file

You can supply the credentials through a config file:

1. Create the config file. Check out [Scaleway docs](https://github.com/scaleway/scaleway-sdk-go/blob/master/scw/README.md#scaleway-config) for instructions
2. Mount it as a Secret into the Pod
3. Configure environment variable `SCW_PROFILE` to match the profile name in the config file
4. Configure environment variable `SCW_CONFIG_PATH` to match the location of the mounted config file

### Configuration using environment variables

Two environment variables are needed to run ExternalDNS with Scaleway DNS:

- `SCW_ACCESS_KEY` which is the Access Key.
- `SCW_SECRET_KEY` which is the Secret Key.

## Deploy ExternalDNS

Connect your `kubectl` client to the cluster you want to test ExternalDNS with.
Then apply one of the following manifests file to deploy ExternalDNS.

The following example are suited for development. For a production usage, prefer secrets over environment, and use a [tagged release](https://github.com/kubernetes-sigs/external-dns/releases).

### Manifest (for clusters without RBAC enabled)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-dns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: external-dns
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: external-dns
    spec:
      containers:
      - name: external-dns
        image: registry.k8s.io/external-dns/external-dns:v0.18.0
        args:
        - --source=service # ingress is also possible
        - --domain-filter=example.com # (optional) limit to only example.com domains; change to match the zone created above.
        - --provider=scaleway
        env:
        - name: SCW_ACCESS_KEY
          value: "<your access key>"
        - name: SCW_SECRET_KEY
          value: "<your secret key>"
        ### Set if configuring using a config file. Make sure to create the Secret first.
        # - name: SCW_PROFILE
        #   value: "<profile name>"
        # - name: SCW_CONFIG_PATH
        #   value: /etc/scw/config.yaml
    #     volumeMounts:
    #     - name: scw-config
    #       mountPath: /etc/scw/config.yaml
    #       readOnly: true
    # volumes:
    # - name: scw-config
    #   secret:
    #     secretName: scw-config
    ###
```

### Manifest (for clusters with RBAC enabled)

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-dns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns
rules:
- apiGroups: [""]
  resources: ["services","pods"]
  verbs: ["get","watch","list"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get","watch","list"]
- apiGroups: ["extensions"]
  resources: ["ingresses"]
  verbs: ["get","watch","list"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["list","watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-dns-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-dns
subjects:
- kind: ServiceAccount
  name: external-dns
  namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-dns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: external-dns
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: external-dns
    spec:
      serviceAccountName: external-dns
      containers:
      - name: external-dns
        image: registry.k8s.io/external-dns/external-dns:v0.18.0
        args:
        - --source=service # ingress is also possible
        - --domain-filter=example.com # (optional) limit to only example.com domains; change to match the zone created above.
        - --provider=scaleway
        env:
        - name: SCW_ACCESS_KEY
          value: "<your access key>"
        - name: SCW_SECRET_KEY
          value: "<your secret key>"
        ### Set if configuring using a config file. Make sure to create the Secret first.
        # - name: SCW_PROFILE
        #   value: "<profile name>"
        # - name: SCW_CONFIG_PATH
        #   value: /etc/scw/config.yaml
    #     volumeMounts:
    #     - name: scw-config
    #       mountPath: /etc/scw/config.yaml
    #       readOnly: true
    # volumes:
    # - name: scw-config
    #   secret:
    #     secretName: scw-config
    ###
```

## Deploying an Nginx Service

Create a service file called 'nginx.yaml' with the following contents:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    external-dns.alpha.kubernetes.io/hostname: my-app.example.com
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
```

Note the annotation on the service; use the same hostname as the Scaleway DNS zone created above.

ExternalDNS uses this annotation to determine what services should be registered with DNS. Removing the annotation will cause ExternalDNS to remove the corresponding DNS records.

Create the deployment and service:

```console
kubectl create -f nginx.yaml
```

Depending where you run your service it can take a little while for your cloud provider to create an external IP for the service.

Once the service has an external IP assigned, ExternalDNS will notice the new service IP address and synchronize the Scaleway DNS records.

## Verifying Scaleway DNS records

Check your [Scaleway DNS UI](https://console.scaleway.com/domains/external) to view the records for your Scaleway DNS zone.

Click on the zone for the one created above if a different domain was used.

This should show the external IP address of the service as the A record for your domain.

## Cleanup

Now that we have verified that ExternalDNS will automatically manage Scaleway DNS records, we can delete the tutorial's example:

```sh
kubectl delete service -f nginx.yaml
kubectl delete service -f externaldns.yaml
```
