package api

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	ClusterOperatorName                       = "console"
	ConfigResourceName                        = "cluster"
	ConsoleContainerPort                      = 443
	ConsoleContainerPortName                  = "https"
	ConsoleContainerTargetPort                = 8443
	ConsoleServingCertName                    = "console-serving-cert"
	CreateOAuthClientManagedClusterActionName = "console-create-oauth-client"
	DefaultIngressCertConfigMapName           = "default-ingress-cert"
	DownloadsPort                             = 8080
	DownloadsPortName                         = "http"
	DownloadsResourceName                     = "downloads"
	HubClusterName                            = "local-cluster"
	ManagedClusterAPIServerCertKey            = "ca-bundle.crt"
	ManagedClusterAPIServerCertMountDir       = "/var/managed-cluster-api-server-certs"
	ManagedClusterAPIServerCertName           = "managed-cluster-api-server-cert"
	ManagedClusterActionAPIGroup              = "action.open-cluster-management.io"
	ManagedClusterActionAPIVersion            = "v1beta1"
	ManagedClusterActionResource              = "managedclusteractions"
	ManagedClusterConfigKey                   = "managed-clusters.yaml"
	ManagedClusterConfigMapName               = "managed-clusters"
	ManagedClusterConfigMountDir              = "/var/managed-cluster-config"
	ManagedClusterLabel                       = "managed-cluster"
	ManagedClusterOAuthClientName             = "console-managed-cluster-oauth-client"
	ManagedClusterOAuthServerCertKey          = "ca-bundle.crt"
	ManagedClusterOAuthServerCertMountDir     = "/var/managed-cluster-oauth-server-certs"
	ManagedClusterOAuthServerCertName         = "managed-cluster-oauth-server-cert"
	ManagedClusterProductClaim                = "product.open-cluster-management.io"
	ManagedClusterVersionClaim                = "version.openshift.io"
	ManagedClusterViewAPIGroup                = "view.open-cluster-management.io"
	ManagedClusterViewAPIVersion              = "v1beta1"
	ManagedClusterViewResource                = "managedclusterviews"
	ManagedProxyServiceResolverCRDName        = "managedproxyserviceresolvers.proxy.open-cluster-management.io"
	NodeArchitectureLabel                     = "kubernetes.io/arch"
	NodeOperatingSystemLabel                  = "kubernetes.io/os"
	OAuthClientManagedClusterViewName         = "console-oauth-client"
	OAuthConfigMapName                        = "oauth-openshift"
	OAuthServerCertManagedClusterViewName     = "console-oauth-server-cert"
	OAuthServingCertConfigMapName             = "oauth-serving-cert"
	OCCLIDownloadsCustomResourceName          = "oc-cli-downloads"
	ODOCLIDownloadsCustomResourceName         = "odo-cli-downloads"
	OLMConfigGroup                            = "operators.coreos.com"
	OLMConfigManagedClusterViewName           = "olm-config"
	OLMConfigResource                         = "olmconfigs"
	OLMConfigVersion                          = "v1"
	OpenShiftConfigManagedNamespace           = "openshift-config-managed"
	OpenShiftConfigNamespace                  = "openshift-config"
	OpenShiftConsoleConfigMapName             = "console-config"
	OpenShiftConsoleName                      = "console"
	OpenShiftConsoleOperator                  = "console-operator"
	OpenShiftConsoleOperatorNamespace         = "openshift-console-operator"
	OpenShiftConsolePublicConfigMapName       = "console-public"
	OpenShiftCustomLogoConfigMapName          = "custom-logo"
	OpenShiftMonitoringConfigMapName          = "monitoring-shared-config"
	OpenshiftConsoleCustomRouteName           = "console-custom"
	OpenshiftConsoleRedirectServiceName       = "console-redirect"
	RedirectContainerPort                     = 8444
	RedirectContainerPortName                 = "custom-route-redirect"
	ServiceCAConfigMapName                    = "service-ca"
	TargetNamespace                           = "openshift-console"
	TrustedCABundleKey                        = "ca-bundle.crt"
	TrustedCABundleMountDir                   = "/etc/pki/ca-trust/extracted/pem"
	TrustedCABundleMountFile                  = "tls-ca-bundle.pem"
	TrustedCAConfigMapName                    = "trusted-ca-bundle"
	UpgradeConsoleNotification                = "cluster-upgrade"
	V1Alpha1PluginI18nAnnotation              = "console.openshift.io/use-i18n"
	VersionResourceName                       = "version"

	OAuthClientName                         = OpenShiftConsoleName
	OpenShiftConsoleDeploymentName          = OpenShiftConsoleName
	OpenShiftConsoleDownloadsDeploymentName = DownloadsResourceName
	OpenShiftConsoleDownloadsPDBName        = DownloadsResourceName
	OpenShiftConsoleDownloadsRouteName      = DownloadsResourceName
	OpenShiftConsoleNamespace               = TargetNamespace
	OpenShiftConsolePDBName                 = OpenShiftConsoleName
	OpenShiftConsoleRouteName               = OpenShiftConsoleName
	OpenShiftConsoleServiceName             = OpenShiftConsoleName
	RedirectContainerTargetPort             = RedirectContainerPort
)

var (
	ManagedClusterViewGroupVersionResource = schema.GroupVersionResource{
		Group:    ManagedClusterViewAPIGroup,
		Version:  ManagedClusterViewAPIVersion,
		Resource: ManagedClusterViewResource,
	}
	ManagedClusterActionGroupVersionResource = schema.GroupVersionResource{
		Group:    ManagedClusterActionAPIGroup,
		Version:  ManagedClusterActionAPIVersion,
		Resource: ManagedClusterActionResource,
	}

	// List of products we support for managed clusters (under the claims "product.open-cluster-management.io")
	// Use a map for trivial lookup
	SupportedClusterProducts = map[string]struct{}{
		"OpenShift":          {},
		"ROSA":               {},
		"ARO":                {},
		"ROKS":               {},
		"OpenShiftDedicated": {},
	}
)
