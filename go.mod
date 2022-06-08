module github.com/openshift/console-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20210831091943-07e756545ac1
	github.com/openshift/build-machinery-go v0.0.0-20210806203541-4ea9b6da3a37
	github.com/openshift/client-go v0.0.0-20210831095141-e19a065e79f7
	github.com/openshift/library-go v0.0.0-20220119132903-b5557aacc264
	github.com/pkg/profile v1.4.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/klog/v2 v2.9.0
)

replace (
	google.golang.org/grpc => google.golang.org/grpc v1.40.0
	k8s.io/apiserver => github.com/openshift/kubernetes-apiserver v0.0.0-20211019154525-d47792cfd13b // points to openshift-apiserver-4.9-kubernetes-1.22.2
)
