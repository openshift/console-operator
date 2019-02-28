package configmapcabundle

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-ca-operator/pkg/controller/configmapcainjector/starter"
	"github.com/openshift/service-ca-operator/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-cabundle-injector"
	componentNamespace = "openshift-service-cert-signer"
)

func NewController() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig(componentName, version.Get(), starter.StartConfigMapCABundleInjector).
		NewCommand()
	cmd.Use = "configmap-cabundle-injector"
	cmd.Short = "Start the ConfigMap CA Bundle Injection controller"
	return cmd
}
