# Cluster Gateway

"Cluster-Gateway" is a gateway apiserver for routing kubernetes api traffic
to multiple kubernetes clusters. Additionally, the gateway is completely 
pluggable for a running kubernetes cluster natively because it is developed
based on the native api extensibility named [apiserver-aggregation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/).
A new extended resource "cluster.core.oam.dev/ClusterGateway" will be registered into the
hosting cluster after properly applying corresponding `APIService` objects, 
and the new subresource named "proxy" will be available for every existing 
"ClusterGateway" resource which is inspired by the original kubernetes 
"service/proxy", "pod/proxy" subresource.

Overall our "Cluster-Gateway" also has the following merits as a multi-cluster 
api-gateway solution:

- Zero-Dependency: Normally an aggregated apiserver must be deployed along with a
  dedicated etcd cluster which is bringing extra costs for the admins. While
  our "Cluster-Gateway" can be running completely without etcd instances,
  because the extended "ClusterGateway" resource are physically stored as
  secret resources in the hosting kubernetes cluster.
  
- Scalability: We can scale out the gateway instances in arbitrary replicas
  freely. Also it's proven stably working in production for years.

## Image

```shell
$ docker pull oamdev/cluster-gateway:v1.1.4 # Or other newer tags
```

## Documentation

__Run Local__: https://github.com/oam-dev/cluster-gateway/blob/master/docs/non-etcd-apiserver/local-run.md


#### Resource Example

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

