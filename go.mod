module github.com/openshift/console-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20211209135129-c58d9f695577
	github.com/openshift/build-machinery-go v0.0.0-20211213093930-7e33a7eb4ce3
	github.com/openshift/client-go v0.0.0-20211209144617-7385dd6338e3
	github.com/openshift/library-go v0.0.0-20211220195323-eca2c467c492
	github.com/pkg/profile v1.4.0 // indirect
	github.com/spf13/cobra v1.2.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/component-base v0.23.0
	k8s.io/klog/v2 v2.30.0
)

replace (
	github.com/openshift/api => github.com/jhadvig/api v0.0.0-20211101154927-473eacc76bdf
	github.com/openshift/client-go => github.com/jhadvig/client-go v0.0.0-20211101145210-04457ae71f20
)
