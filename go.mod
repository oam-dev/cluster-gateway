module github.com/oam-dev/cluster-gateway

go 1.16

require (
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/spec v0.19.8 // indirect
	github.com/go-openapi/swag v0.19.11 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.6 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/apiserver v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/component-base v0.21.3
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/apiserver-runtime v1.0.3-0.20210906132642-810075b08b5f
	sigs.k8s.io/controller-runtime v0.9.5
)

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
