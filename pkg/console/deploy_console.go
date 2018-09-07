package console

import (
	"k8s.io/apimachinery/pkg/runtime"

	// "encoding/json" think ill stick with yaml
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	// "github.com/ose/Godeps/_workspace/src/k8s.io/kubernetes/pkg/api/v1"
)

// not sure that the operator is responsible for creating its own
// namespace?  doesn't hurt to ensure we have it, but can comment out
// if this is not necessary
func newConsoleNamespace() string {
	// the console-operator should be installed somewhere.
	// then the CONSOLE_NAMESPACE must exist
	// so that we can install the console into it.
	// should the operator live within the same namespace?
	logrus.Info("TODO: create Namespace `openshift-console`?")
	return ""
}

func newConsoleConfigMap() string {
	logrus.Info("TODO: create ConfigMap `console-config`")
	return ""
}

// the oauth client can be created after the route, once we have a hostname
// - will create a client secret
//   - reference by configmap/deployment
func newConsoleOauthClient() string {
	logrus.Info("TODO: create cluster scoped OAuth client `openshift-console`")
	return ""
}

func logYaml(obj runtime.Object) {
	// REALLY NOISY, but handy for debugging:
	// deployJSON, err := json.Marshal(d)
	deployYAML, err := yaml.Marshal(obj)
	if err != nil {
		logrus.Info("failed to show deployment yaml in log")
	}
	// TODO: dev toggle, make this useful if kept :)
	if(false) {
		logrus.Infof("Deploying:", string(deployYAML))
	}
}

// Deploy Vault Example:
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L39
func deployConsole(cr *v1alpha1.Console) error {
	newConsoleNamespace()
	newConsoleConfigMap()
	svc := newConsoleService()
	d := newConsoleDeployment(cr)
	rt := newConsoleRoute()
	newConsoleOauthClient()
	// logrus.Info("Created stubs", n, cm, svc, rt, oauth)
	logrus.Info("Created", svc.Kind, svc.ObjectMeta.Name, d.Kind, d.ObjectMeta.Name)

	logYaml(d)
	logYaml(svc)
	logYaml(rt)

	if err := sdk.Create(svc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console service : %v", err)
		return err
	} else {
		logrus.Info("created console service")
	}

	if err := sdk.Create(rt); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console route : %v", err)
		return err
	} else {
		logrus.Info("created console route")
	}

	if err := sdk.Create(d); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console deployment : %v", err)
		return err
	} else {
		logrus.Info("created console deployment")
	}


	return nil
}
