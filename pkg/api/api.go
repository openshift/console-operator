package api

const (
	TargetNamespace    = "openshift-console"
	ConfigResourceName = "cluster"
)

// consts to maintain existing names of various sub-resources
const (
	ClusterOperatorName                     = "console"
	OpenShiftConsoleName                    = "console"
	OpenShiftConsoleNamespace               = TargetNamespace
	OpenShiftConsoleOperatorNamespace       = "openshift-console-operator"
	OpenShiftConsoleOperator                = "console-operator"
	OpenShiftConsoleConfigMapName           = "console-config"
	OpenShiftConsolePublicConfigMapName     = "console-public"
	ServiceCAConfigMapName                  = "service-ca"
	DefaultIngressCertConfigMapName         = "default-ingress-cert"
	OAuthServingCertConfigMapName           = "oauth-serving-cert"
	OAuthConfigMapName                      = "oauth-openshift"
	OpenShiftConsoleDeploymentName          = OpenShiftConsoleName
	OpenShiftConsoleServiceName             = OpenShiftConsoleName
	OpenshiftConsoleRedirectServiceName     = "console-redirect"
	OpenShiftConsoleRouteName               = OpenShiftConsoleName
	OpenshiftConsoleCustomRouteName         = "console-custom"
	DownloadsResourceName                   = "downloads"
	OpenShiftConsoleDownloadsRouteName      = DownloadsResourceName
	OpenShiftConsoleDownloadsDeploymentName = DownloadsResourceName
	OAuthClientName                         = OpenShiftConsoleName
	OpenShiftConfigManagedNamespace         = "openshift-config-managed"
	OpenShiftConfigNamespace                = "openshift-config"
	OpenShiftCustomLogoConfigMapName        = "custom-logo"
	TrustedCAConfigMapName                  = "trusted-ca-bundle"
	TrustedCABundleKey                      = "ca-bundle.crt"
	TrustedCABundleMountDir                 = "/etc/pki/ca-trust/extracted/pem"
	TrustedCABundleMountFile                = "tls-ca-bundle.pem"
	OCCLIDownloadsCustomResourceName        = "oc-cli-downloads"
	ODOCLIDownloadsCustomResourceName       = "odo-cli-downloads"
	HubClusterName                          = "local-cluster"
	ManagedClusterLabel                     = "managed-cluster"
	ManagedClusterConfigMapName             = "managed-clusters"
	ManagedClusterConfigMountDir            = "/var/managed-cluster-config"
	ManagedClusterConfigKey                 = "managed-clusters.yaml"
	ManagedClusterAPIServerCAMountDir       = "/var/managed-cluster-certs"
	ManagedClusterAPIServerCAName           = "managed-cluster-api-server-ca"
	ManagedClusterAPIServerCAKey            = "ca-bundle.crt"
	ManagedClusterIngressCertName           = "managed-cluster-ingress-cert"
	ManagedClusterIngressCertKey            = "ca-bundle.crt"

	ConsoleContainerPortName    = "https"
	ConsoleContainerPort        = 443
	ConsoleContainerTargetPort  = 8443
	RedirectContainerPortName   = "custom-route-redirect"
	RedirectContainerPort       = 8444
	RedirectContainerTargetPort = RedirectContainerPort
	ConsoleServingCertName      = "console-serving-cert"
	DownloadsPortName           = "http"
	DownloadsPort               = 8080
)
