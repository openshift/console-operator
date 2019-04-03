package servingcertsigner

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/service-ca-operator/pkg/controller/servingcert/starter"
	"github.com/openshift/service-ca-operator/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-serving-ca"
	componentNamespace = "openshift-service-ca"
)

func NewController() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig(componentName, version.Get(), starter.StartServiceServingCertSigner).
		NewCommand()
	cmd.Use = "serving-cert-signer"
	cmd.Short = "Start the Service Serving Cert Signer controller"
	return cmd

}
