package api

const (
	// AuthProxyCAConfigMapName is the destination name used for the auth proxy trustedCA ConfigMap
	// when copying it into local namespaces.
	AuthProxyCAConfigMapName = "v4-0-config-system-auth-proxy-ca"
	// AuthProxyCAMountDir is the mount point where the auth proxy trustedCA ConfigMap is mounted in containers.
	AuthProxyCAMountDir = "/var/auth-proxy-ca"
	// AuthProxyCAFileName is the auth proxy trustedCA ConfigMap key that is expected to contain the CA bundle.
	// It's also the local filename that the bundle is accessible as under AuthProxyCAMountDir.
	AuthProxyCAFileName = "ca-bundle.crt"

	AuthServerCAMountDir                = "/var/auth-server-ca"
	AuthServerCAFileName                = "ca-bundle.crt"
	CLIOIDCClientComponentName          = "cli"
	ClusterOperatorName                 = "console"
	ConfigResourceName                  = "cluster"
	ConsoleContainerPort                = 443
	ConsoleContainerPortName            = "https"
	ConsoleContainerTargetPort          = 8443
	ConsoleServingCertName              = "console-serving-cert"
	DefaultIngressCertConfigMapName     = "default-ingress-cert"
	DownloadsPort                       = 8080
	DownloadsPortName                   = "http"
	DownloadsResourceName               = "downloads"
	NodeArchitectureLabel               = "kubernetes.io/arch"
	NodeOperatingSystemLabel            = "kubernetes.io/os"
	OAuthConfigMapName                  = "oauth-openshift"
	OAuthServingCertConfigMapName       = "oauth-serving-cert"
	OCCLIDownloadsCustomResourceName    = "oc-cli-downloads"
	OLMConfigGroup                      = "operators.coreos.com"
	OLMConfigResource                   = "olmconfigs"
	OLMConfigVersion                    = "v1"
	OpenShiftConfigManagedNamespace     = "openshift-config-managed"
	OpenShiftConfigNamespace            = "openshift-config"
	OpenShiftConsoleConfigMapName       = "console-config"
	OpenShiftConsoleName                = "console"
	OpenShiftConsoleOperator            = "console-operator"
	OpenShiftConsoleOperatorNamespace   = "openshift-console-operator"
	OpenShiftConsolePublicConfigMapName = "console-public"
	OpenShiftCustomLogoConfigMapName    = "custom-logo"
	OpenShiftMonitoringConfigMapName    = "monitoring-shared-config"
	OpenshiftConsoleCustomRouteName     = "console-custom"
	OpenshiftDownloadsCustomRouteName   = "downloads-custom"
	OpenshiftConsoleRedirectServiceName = "console-redirect"
	RedirectContainerPort               = 8444
	RedirectContainerPortName           = "custom-route-redirect"
	ServiceCAConfigMapName              = "service-ca"
	SessionSecretName                   = "session-secret"
	TargetNamespace                     = "openshift-console"
	TrustedCABundleKey                  = "ca-bundle.crt"
	TrustedCABundleMountDir             = "/etc/pki/ca-trust/extracted/pem"
	TrustedCABundleMountFile            = "tls-ca-bundle.pem"
	TrustedCAConfigMapName              = "trusted-ca-bundle"
	UpgradeConsoleNotification          = "cluster-upgrade"
	V1Alpha1PluginI18nAnnotation        = "console.openshift.io/use-i18n"
	VersionResourceName                 = "version"

	HelmChartreposViewerRoleName             = "helm-chartrepos-viewer"
	ProjectHelmChartrepositoryEditorRoleName = "project-helm-chartrepository-editor"
	ConsoleExtensionsReaderRoleName          = "console-extensions-reader"

	// ingress instance named "default" is the OOTB ingresscontroller
	// this is an implicit stable API
	DefaultIngressController   = "default"
	IngressControllerNamespace = "openshift-ingress-operator"

	OAuthClientName                             = OpenShiftConsoleName
	OpenShiftConsoleDeploymentName              = OpenShiftConsoleName
	OpenShiftConsoleDownloadsDeploymentName     = DownloadsResourceName
	OpenShiftConsoleDownloadsPDBName            = DownloadsResourceName
	OpenShiftConsoleDownloadsRouteName          = DownloadsResourceName
	OpenShiftConsoleNamespace                   = TargetNamespace
	OpenShiftConsolePDBName                     = OpenShiftConsoleName
	OpenShiftConsoleRouteName                   = OpenShiftConsoleName
	OpenShiftConsoleServiceName                 = OpenShiftConsoleName
	OpenShiftConsoleDownloadsServiceAccountName = OpenShiftConsoleDownloadsDeploymentName
	OpenShiftConsoleServiceAccountName          = OpenShiftConsoleDeploymentName
	RedirectContainerTargetPort                 = RedirectContainerPort
)
