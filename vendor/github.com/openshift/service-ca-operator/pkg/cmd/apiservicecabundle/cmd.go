package apiservicecabundle

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/service-ca-operator/pkg/controller/apiservicecabundle/starter"
	"github.com/openshift/service-ca-operator/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-apiservice-injector"
	componentNamespace = "openshift-service-ca"
)

func NewController() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig(componentName, version.Get(), starter.StartAPIServiceCABundleInjector).
		NewCommand()
	cmd.Use = "apiservice-cabundle-injector"
	cmd.Short = "Start the APIService CA Bundle Injection controller"
	return cmd
}
