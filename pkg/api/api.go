package api

const (
	TargetNamespace    = "openshift-console"
	ConfigResourceName = "cluster"
)

// consts to maintain existing names of various sub-resources
const (
	OpenShiftConsoleName                = "console"
	OpenShiftConsoleNamespace           = TargetNamespace
	OpenShiftConsoleOperatorNamespace   = "openshift-console-operator"
	OpenShiftConsoleOperator            = "console-operator"
	OpenShiftConsoleConfigMapName       = "console-config"
	OpenShiftConsolePublicConfigMapName = "console-public"
	ServiceCAConfigMapName              = "service-ca"
	OpenShiftConsoleDeploymentName      = OpenShiftConsoleName
	OpenShiftConsoleServiceName         = OpenShiftConsoleName
	OpenShiftConsoleRouteName           = OpenShiftConsoleName
	OAuthClientName                     = OpenShiftConsoleName
	OpenShiftConfigManagedNamespace     = "openshift-config-managed"
	OpenShiftConfigNamespace            = "openshift-config"
	OpenShiftCustomLogoConfigMapName    = "custom-logo"
)
