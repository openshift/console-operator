package operator

import (
	// golang
	"context"
	// 3rd party
	"github.com/spf13/cobra"
	"k8s.io/utils/clock"

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
			starter.RunOperator,
			clock.RealClock{}).
		NewCommandWithContext(context.TODO())
	cmd.Use = "operator"
	cmd.Short = "Start the Console Operator"
	// TODO: better docs on this
	// should probably give example usage, etc
	// https://github.com/spf13/cobra#create-rootcmd
	cmd.Long = `An Operator for a web console for OpenShift.
				`
	return cmd
}
