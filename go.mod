module github.com/openshift/console-operator

go 1.13

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/getsentry/raven-go v0.2.1-0.20190513200303-c977f96e1095 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/go-test/deep v1.0.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20201117184740-859beeffd973
	github.com/openshift/build-machinery-go v0.0.0-20200917070002-f171684f77ab
	github.com/openshift/client-go v0.0.0-20200827190008-3062137373b5
	github.com/openshift/library-go v0.0.0-20200907120738-ea57b121ba1a
	github.com/pkg/profile v1.4.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/text v0.3.4 // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.2
	k8s.io/component-base v0.19.2
	k8s.io/klog/v2 v2.4.0
	sigs.k8s.io/structured-merge-diff/v4 v4.0.2 // indirect
)

replace (
	github.com/openshift/api => github.com/jhadvig/api v0.0.0-20201120110223-105453730373
	github.com/openshift/client-go => github.com/jhadvig/client-go v0.0.0-20201120114228-b390d4a67199
)
