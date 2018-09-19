package console

import (
	// "encoding/json" think ill stick with yaml
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/sirupsen/logrus"
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
func DeployConsole(cr *v1alpha1.Console) {
	newConsoleNamespace()
	CreateService(cr)
	rt, _ := CreateRoute(cr)
	CreateConsoleConfigMap(cr, rt)
	CreateOAuthClient(cr, rt)
	CreateConsoleDeployment(cr)
}
