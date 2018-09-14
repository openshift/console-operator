package console

import (
	// "encoding/json" think ill stick with yaml
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"

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

// Deploy Vault Example:
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L39
func deployConsole(cr *v1alpha1.Console) error {
	newConsoleNamespace()
	svc := newConsoleService(cr)
	rt := newConsoleRoute(cr)
	cm := newConsoleConfigMap(cr, rt)
	oauthc, oauths := newConsoleOauthClient(cr, rt)
	d := newConsoleDeployment(cr)

	// logrus.Info("Created stubs", n, cm, svc, rt, oauth)
	logrus.Info("Created", svc.Kind, svc.ObjectMeta.Name, d.Kind, d.ObjectMeta.Name)

	if err := sdk.Create(cm); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console configmap : %v", err)
		return err
	} else {
		logrus.Info("created console configmap")
		// logYaml(cm)
	}

	if err := sdk.Create(svc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console service : %v", err)
		return err
	} else {
		logrus.Info("created console service")
		// logYaml(svc)
	}

	if err := sdk.Create(rt); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console route : %v", err)
		return err
	} else {
		logrus.Info("created console route")
		// logYaml(rt)
	}

	if err := sdk.Create(d); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console deployment : %v", err)
		return err
	} else {
		logrus.Info("created console deployment")
		// logYaml(d)
	}

	if err := sdk.Create(oauthc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console oauth client : %v", err)
		return err
	} else {
		logrus.Info("created console oauth client")
		// logYaml(oauthc)
	}

	if err := sdk.Create(oauths); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console oauth client secret : %v", err)
		return err
	} else {
		logrus.Info("created console oauth client secret")
		// logYaml(oauths)
	}


	return nil
}
