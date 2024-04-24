package authentication

import (
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	"golang.org/x/exp/slices"
)

func GetOIDCOCLoginCommand(authConfig *configv1.Authentication, apiServerURL string) string {
	provider, clientConfig := GetOIDCClientConfig(authConfig, api.TargetNamespace, api.CLIOIDCClientComponentName)
	if provider == nil || clientConfig == nil || provider.Issuer.URL == "" {
		return ""
	}

	extraScopes := ""
	if len(clientConfig.ExtraScopes) > 0 {
		extraScopes = fmt.Sprintf(" --extra-scopes %s", strings.Join(clientConfig.ExtraScopes, ","))
	}

	return fmt.Sprintf("oc login %s --issuer-url %s --exec-plugin oc-oidc --client-id %s%s", apiServerURL, provider.Issuer.URL, clientConfig.ClientID, extraScopes)
}

func GetOIDCClientConfig(authnConfig *configv1.Authentication, componentNamespace, componentName string) (*configv1.OIDCProvider, *configv1.OIDCClientConfig) {
	if len(authnConfig.Spec.OIDCProviders) == 0 {
		return nil, nil
	}

	var clientIdx int
	for i := 0; i < len(authnConfig.Spec.OIDCProviders); i++ {
		clientIdx = slices.IndexFunc(authnConfig.Spec.OIDCProviders[i].OIDCClients, func(oc configv1.OIDCClientConfig) bool {
			if oc.ComponentNamespace == componentNamespace && oc.ComponentName == componentName {
				return true
			}
			return false
		})
		if clientIdx != -1 {
			return &authnConfig.Spec.OIDCProviders[i], &authnConfig.Spec.OIDCProviders[i].OIDCClients[clientIdx]
		}
	}

	return nil, nil
}
