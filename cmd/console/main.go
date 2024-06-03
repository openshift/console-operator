package main

import (
	// standard lib
	"os"

	// 3rd party
	"github.com/spf13/cobra"

	// kube / openshift
	"k8s.io/component-base/cli"

	// us
	"github.com/openshift/console-operator/pkg/cmd/operator"
	"github.com/openshift/console-operator/pkg/cmd/version"
)

func main() {
	// build a new cobra command
	command := NewOperatorCommand()
	code := cli.Run(command)
	os.Exit(code)
}

// create the root "console" command
// we will add subcommands to this
func NewOperatorCommand() *cobra.Command {
	// "console" just prints help, then exists.  It doesn't start
	// the operator.
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Top level command",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(operator.NewOperator())
	cmd.AddCommand(version.NewVersion())

	return cmd
}
