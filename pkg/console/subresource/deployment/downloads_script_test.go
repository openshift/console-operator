package deployment

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

func TestDownloadsArchiveScript(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required to test the embedded downloads server")
	}
	deployment := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/deployments/downloads-deployment.yaml"))
	command := deployment.Spec.Template.Spec.Containers[0].Args[1]
	script, ok := strings.CutPrefix(command, "cat <<EOF >/tmp/serve.py\n")
	if !ok {
		t.Fatal("could not find embedded Python script")
	}
	script, _, ok = strings.Cut(script, "\nEOF\n")
	if !ok {
		t.Fatal("could not find end of embedded Python script")
	}
	cmd := exec.Command(python, "testdata/downloads_script_test.py")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("downloads script tests failed: %v\n%s", err, output)
	}
}
