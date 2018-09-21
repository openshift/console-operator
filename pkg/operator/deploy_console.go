package operator

import (
	// "encoding/json" think ill stick with yaml
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

// Deploy Vault Example:
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L39
func DeployConsole(cr *v1alpha1.Console) {
	CreateService(cr)
	rt, _ := CreateRoute(cr)
	CreateConsoleConfigMap(cr, rt)
	CreateOAuthClient(cr, rt)
	CreateConsoleDeployment(cr)
}
