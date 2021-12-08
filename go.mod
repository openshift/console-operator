module github.com/openshift/console-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/open-cluster-management/api v0.0.0-20210527013639-a6845f2ebcb1
	github.com/openshift/api v0.0.0-20211103080632-8981c8822dfa
	github.com/openshift/build-machinery-go v0.0.0-20210712174854-1bb7fd1518d3
	github.com/openshift/client-go v0.0.0-20211104174419-390ab1a408da
	github.com/openshift/library-go v0.0.0-20210330121117-68dd4a4c9d9e
	github.com/pkg/profile v1.4.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1 // indirect
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/component-base v0.22.1
	k8s.io/klog/v2 v2.9.0
)

replace (
	github.com/openshift/api => github.com/jhadvig/api v0.0.0-20211101154927-473eacc76bdf
	github.com/openshift/client-go => github.com/jhadvig/client-go v0.0.0-20211101145210-04457ae71f20
)
