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
apiVersion: "core.oam.dev/v1alpha1"
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

#### Resources

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: "..."
  annotations:
    kubernetes.io/service-account.name: "..."
    kubernetes.io/service-account.uid: "..."
type: kubernetes.io/tls
data:
  endpoint: "..." # base64 encoded api-endpoint url
  ca.crt: "..."
  tls.crt: "..."
  tls.key: "..."
---
apiVersion: v1
kind: Secret
metadata:
  name: proxy-test
  annotations:
    kubernetes.io/service-account.name: "..."
    kubernetes.io/service-account.uid: "..."
type: kubernetes.io/service-account-token
data:
  ca.crt: "..."
  namespace: "..."
  token: "..."
  endpoint: "..."
```

#### Run Local w/ KinD Cluster

build the container:
```shell
docker build \
  -t "cluster-gateway:v0.0.0-non-etcd" \
  -f cmd/non-etcd-apiserver/Dockerfile .
```

spawn a local KinD cluster:
```shell
kind create cluster --name tmp
kind load docker-image "cluster-gateway:v0.0.0-non-etcd" --name tmp
```

apply the manifests below:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway-deployment
  labels:
    app: gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      containers:
        - name: gateway
          image: "cluster-gateway:v0.0.0-non-etcd"
          command:
            - ./apiserver
            - --secure-port=9443
            - --secret-namespace=default
            - --feature-gates=APIPriorityAndFairness=false
          ports:
            - containerPort: 9443
---
apiVersion: v1
kind: Service
metadata:
  name: gateway-service
spec:
  selector:
    app: gateway
  ports:
    - protocol: TCP
      port: 9443
      targetPort: 9443
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.core.oam.dev
  labels:
    api: cluster-extension-apiserver
    apiserver: "true"
spec:
  version: v1alpha1
  group: core.oam.dev
  groupPriorityMinimum: 2000
  service:
    name: gateway-service
    namespace: default
    port: 9443
  versionPriority: 10
  insecureSkipTLSVerify: true
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system::extension-apiserver-authentication-reader:cluster-gateway
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
  - kind: ServiceAccount
    name: default
    namespace: default
```