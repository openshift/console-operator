package api

const (
	TargetNamespace    = "openshift-console"
	ConfigResourceName = "cluster"
)

// consts to maintain existing names of various sub-resources
const (
	OpenShiftConsoleName              = "console"
	OpenShiftConsoleNamespace         = TargetNamespace
	OpenShiftConsoleOperatorNamespace = "openshift-console-operator"
	OpenShiftConsoleOperator          = "console-operator"
	OpenShiftConsoleConfigMapName     = "console-config"
	OpenShiftConsoleDeploymentName    = OpenShiftConsoleName
	OpenShiftConsoleServiceName       = OpenShiftConsoleName
	OpenShiftConsoleRouteName         = OpenShiftConsoleName
	OAuthClientName                   = OpenShiftConsoleName
)
