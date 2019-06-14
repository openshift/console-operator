package version

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/openshift/console-operator/pkg/console/version"
	"github.com/spf13/cobra"
	"strings"

	cm "github.com/openshift/console-operator/pkg/console/configmap"
)

var (
	Raw        = "v0.0.1"
	VerInfo    = version.Get()
	GitCommit  = VerInfo.GitCommit
	BuildDate  = VerInfo.BuildDate
	Version    = semver.MustParse(strings.TrimLeft(Raw, "v"))
	BrandValue = cm.DEFAULT_BRAND
	String     = fmt.Sprintf("ConsoleOperator %s\nGit Commit: %s\nBuild Date: %s\nCurrent Brand Setting: %s", Raw, GitCommit, BuildDate, BrandValue)
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
