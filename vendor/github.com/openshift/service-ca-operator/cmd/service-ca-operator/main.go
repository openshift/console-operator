package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/service-ca-operator/pkg/cmd/apiservicecabundle"
	"github.com/openshift/service-ca-operator/pkg/cmd/configmapcabundle"
	"github.com/openshift/service-ca-operator/pkg/cmd/operator"
	"github.com/openshift/service-ca-operator/pkg/cmd/servingcertsigner"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	goflag.CommandLine.Parse([]string{})

	logs.InitLogs()
	defer logs.FlushLogs()

	command := NewSSCSCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewSSCSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-ca-operator",
		Short: "OpenShift Service CA Operator",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(operator.NewOperator())
	cmd.AddCommand(servingcertsigner.NewController())
	cmd.AddCommand(apiservicecabundle.NewController())
	cmd.AddCommand(configmapcabundle.NewController())

	return cmd
}
