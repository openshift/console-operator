module github.com/openshift/console-operator

go 1.18

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/open-cluster-management/api v0.0.0-20210527013639-a6845f2ebcb1
	github.com/openshift/api v0.0.0-20220404140913-04e1813ebb11
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
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
)
