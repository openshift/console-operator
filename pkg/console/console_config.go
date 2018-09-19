package console

const (
	consolePortName        = "http"
	consolePort            = 443
	consoleTargetPort      = 8443
	publicURLName          = "BRIDGE_DEVELOPER_CONSOLE_URL"
	consoleServingCertName = "console-serving-cert"
	consoleOauthConfigName = "console-oauth-config"
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
		name:     consoleServingCertName,
		readOnly: true,
		path:     "/var/serving-cert",
		isSecret: true,
		// defaultMode: 288,
	},
	{
		name:     consoleOauthConfigName,
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
