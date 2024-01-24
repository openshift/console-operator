package authentication

import (
	"fmt"
	"strings"

	config "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	"golang.org/x/exp/slices"
)

func GetOIDCOCLoginCommand(authConfig *config.Authentication, apiServerURL string) string {
	clientConfig := GetOIDCClientConfig(authConfig, api.TargetNamespace, api.OCOIDCClientComponentName)
	if clientConfig == nil {
		return ""
	}

	extraScopes := ""
	if len(clientConfig.ExtraScopes) > 0 {
		extraScopes = fmt.Sprintf(" --extra-scopes %s", strings.Join(clientConfig.ExtraScopes, ","))
	}

	return fmt.Sprintf("oc login %s --exec-plugin oc-oidc --client-id %s%s", apiServerURL, clientConfig.ClientID, extraScopes)
}

func GetOIDCClientConfig(authnConfig *config.Authentication, componentNamespace, componentName string) *config.OIDCClientConfig {
	if len(authnConfig.Spec.OIDCProviders) == 0 {
		return nil
	}

	var oidcClientConfig *config.OIDCClientConfig
	slices.IndexFunc[config.OIDCClientConfig](authnConfig.Spec.OIDCProviders[0].OIDCClients, func(oc config.OIDCClientConfig) bool {
		if oc.ComponentNamespace == componentNamespace && oc.ComponentName == componentName {
			oidcClientConfig = &oc
			return true
		}
		return false
	})

	return oidcClientConfig
}
