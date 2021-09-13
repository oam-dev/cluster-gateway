# Cluster Gateway

## Etcd-Persisted apiserver:

##### Local Run

```shell
$go run ./cmd/apiserver \
  --standalone-debug-mode=true \
  --bind-address=127.0.0.1 \
  --etcd-servers=127.0.0.1:2379 \
  --secure-port=9443
```

#### Resources

```yaml
apiVersion: "cluster.core.oam.dev/v1alpha1"
kind: "ClusterGateway"
metadata:
  name: <..>
spec:
  provider: ""
  access:
    endpoint: "https://127.0.0.1:9443"
    caBundle: "..."
    credential:
      type: X509Certificate
      x509:
        certificate: "..."
        privateKey: "..."
status: { }      
```

## Non-Etcd apiserver:

##### Local Run

```shell
$go run -tags vela ./cmd/non-etcd-apiserver \
  --bind-address=127.0.0.1 \
  --secure-port=9443 \
  --kubeconfig=$KUBECONFIG \
  --authorization-kubeconfig=$KUBECONFIG
  --authentication-kubeconfig=$KUBECONFIG
  --secret-namespace=default
```
