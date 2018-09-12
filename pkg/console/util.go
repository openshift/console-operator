package console

import (
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

const (
	openshiftConsoleName = "openshift-console"
)

// This should return the public url provided for us by the ROUTE or Ingress...
func consoleURL() string {
	// This will need to do some work to generate the real path
	logrus.Info("Console container BRIDGE_DEVELOPER_CONSOLE_URL is HARD-CODED value for now")
	return fmt.Sprintf("https://%s/console/", "api.ui-preserve.origin-gce.dev.openshift.com")
}

func sharedLabels() map[string]string {
	return map[string]string{
		"app": openshiftConsoleName,
	}
}

// similar to how I did this with the helm chart
func labelsForConsole() map[string]string {
	baseLabels := sharedLabels()

	extraLabels := map[string]string{
		"component": "ui",
	}
	// we want to deduplicate, so doing these two loops.
	allLabels := map[string]string{}

	for key, value := range baseLabels {
		allLabels[key] = value
	}
	for key, value := range extraLabels {
		allLabels[key] = value
	}
	return allLabels
}

func sharedMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		// TODO: will we always have one console?
		// if not, then shouldn't our name be more specific?
		Name: openshiftConsoleName, // ATM no configuration, stable name
		// NOTE:
		// namepsace shouldn't be here. it should
		// create with whatever namespace is set via
		// the --namespace flag
		Namespace: "openshift-console-operator-test", // ATM no configuration, "openshift-"
		// these can be overridden/mutated
		Labels: sharedLabels(),
	}
}


func logYaml(obj runtime.Object) {
	// REALLY NOISY, but handy for debugging:
	// deployJSON, err := json.Marshal(d)
	deployYAML, err := yaml.Marshal(obj)
	if err != nil {
		logrus.Info("failed to show deployment yaml in log")
	}
	logrus.Infof("Deploying:", string(deployYAML))
}

func generateLogLevel(cr *v1alpha1.Console) string {
	switch cr.Spec.Logging.Level {
	case 0:
		return "error"
	case 1:
		return "warn"
	case 2, 3:
		return "info"
	}
	return "debug"
}