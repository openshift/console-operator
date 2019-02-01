package operator

import (
	"github.com/openshift/console-operator/pkg/console/operator"
	// 3rd party
	"github.com/spf13/cobra"
	// kube / openshift
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	// us
	"github.com/openshift/console-operator/pkg/console/starter"
	"github.com/openshift/console-operator/pkg/console/version"
)

func NewOperator() *cobra.Command {

	cmd := controllercmd.
		NewControllerCommandConfig(
			"console-operator",
			version.Get(),
			starter.RunOperator).
		NewCommand()
	cmd.Use = "operator"
	cmd.Short = "Start the Console Operator"
	// TODO: better docs on this
	// should probably give example usage, etc
	// https://github.com/spf13/cobra#create-rootcmd
	cmd.Long = `An Operator for a web console for OpenShift.
				`
	cmd.Flags().BoolVarP(
		&operator.CreateDefaultConsoleFlag,
		"create-default-console",
		"d",
		false,
		`Instructs the operator to create a console
        custom resource on startup if one does not exist. 
        `,
	)

	cmd.Flags().StringVarP(
		&operator.Brand,
		"brand",
		"b",
		"okd",
		"Defines what branding the console will show. Defaults to OKD.",
	)

	return cmd
}
