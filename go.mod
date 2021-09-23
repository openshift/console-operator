module github.com/openshift/console-operator

go 1.15

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20210729133217-6a2354a560cb
	github.com/openshift/build-machinery-go v0.0.0-20210423112049-9415d7ebd33e
	github.com/openshift/client-go v0.0.0-20210112160336-8889f8b15bd6
	github.com/openshift/library-go v0.0.0-20210330121117-68dd4a4c9d9e
	github.com/pkg/profile v1.4.0 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.21.0 // indirect
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.0
	k8s.io/component-base v0.21.0
	k8s.io/klog/v2 v2.8.0
)

// points to temporary-watch-reduction-patch-1.21 to pick up k/k/pull/101102 - please remove it once the pr merges and a new Z release is cut
replace k8s.io/apiserver => github.com/openshift/kubernetes-apiserver v0.0.0-20210419140141-620426e63a99
