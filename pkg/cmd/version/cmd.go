package version

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/spf13/cobra"
	"strings"
)

var (
	Raw     = "v0.0.1"
	Version = semver.MustParse(strings.TrimLeft(Raw, "v"))
	String  = fmt.Sprintf("ConsoleOperator %s", Raw)
)

func NewVersion() *cobra.Command {
	// TODO:
	// update & use the pkg/version/version.go to pull
	// git information & present here.
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display the Operator Version",
		Run: func(command *cobra.Command, args []string) {
			fmt.Println(String)
		},
	}
	return cmd
}
