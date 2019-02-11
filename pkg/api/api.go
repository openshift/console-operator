package api

const (
	// TargetNamespace could be made configurable if desired
	TargetNamespace = "openshift-console"
	// ResourceName could be made configurable if desired
	// all resources share the same name to make it easier to reason about and to configure single item watches
	// NOTE: this must match metadata.name in the CR.yaml else the CR will be ignored
	ResourceName = "console-operator-resource"
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
