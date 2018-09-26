package operator

import (
	"fmt"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	OpenShiftConsoleName      = "openshift-console"
	OpenShiftConsoleShortName = "console"
)

// This should return the public url provided for us by the ROUTE or Ingress...
func consoleURL() string {
	// This will need to do some work to generate the real path
	logrus.Warn("Console container BRIDGE_DEVELOPER_CONSOLE_URL is HARD-CODED value for now")
	return fmt.Sprintf("https://%s/console/", "api.ui-preserve.origin-gce.dev.openshift.com")
}

func sharedLabels() map[string]string {
	return map[string]string{
		"app": OpenShiftConsoleName,
	}
}

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
		Name: OpenShiftConsoleName, // ATM no configuration, stable name
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
	logrus.Infof("Deploying: %v", string(deployYAML))
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

// objects can have more than one ownerRef, potentially
func addOwnerRef(obj metav1.Object, ownerRef *metav1.OwnerReference) {
	if obj != nil {
		if ownerRef != nil {
			obj.SetOwnerReferences(append(obj.GetOwnerReferences(), *ownerRef))
		}
	}
}

func ownerRefFrom(cr *v1alpha1.Console) *metav1.OwnerReference {
	if cr != nil {
		truthy := true
		return &metav1.OwnerReference{
			APIVersion: cr.APIVersion,
			Kind:       cr.Kind,
			Name:       cr.Name,
			UID:        cr.UID,
			Controller: &truthy,
		}
	}
	return nil
}
