package console

//const (
//	servingCertVolName = "serving-cert"
//	// servingCertSecretName = "console-serving-cert"
//	// mount path for the console https serving certificate
//	servingCertPath = "/var/serving-cert"
//)
//
//const (
//	oauthConfigVolName = "oauth-config"
//	// mount path for configuration for oauth client
//	oauthConfigPath = "/var/oauth-config"
//)
//
//const (
//	consoleConfigVolName = "console-config"
//	// path to the mount point for the ConfigMap containing
//	// runtime configuration for the console
//	consoleConfigPath = "/var/console-config"
//)

const (
	consolePortName = "http"
	consolePort     = 443
	consoleTargetPort = 8443
	publicURLName   = "BRIDGE_DEVELOPER_CONSOLE_URL"
)

type volumeConfig struct {
	name     string
	readOnly bool
	path     string
	// defaultMode int
	// will be either secret or configMap
	isSecret    bool
	isConfigMap bool
}

var volumeConfigList = []volumeConfig{
	{
		name:     "serving-cert",
		readOnly: true,
		path:     "/var/serving-cert",
		isSecret: true,
		// defaultMode: 288,
	},
	{
		name:     "oauth-config",
		readOnly: true,
		path:     "/var/oauth-config",
		isSecret: true,
		// defaultMode: 288,
	},
	{
		name:        "console-config",
		readOnly:    true,
		path:        "/var/console-config",
		isConfigMap: true,
		// defaultMode: 288,
	},
}
