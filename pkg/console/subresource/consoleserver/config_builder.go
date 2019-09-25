package consoleserver

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/klog"
)

const (
	clientSecretFilePath    = "/var/oauth-config/clientSecret"
	oauthEndpointCAFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath  = "/var/serving-cert/tls.key"
)

// ConsoleServerCLIConfigBuilder
// Director will be DefaultConfigMap()
//
// b := ConsoleYamlConfigBuilder{}
// return the default config value immediately:
//   b.Config()
//   b.ConfigYAML()
// set all the values:
//   b.Host(host).LogoutURL("").Brand("").DocURL("").APIServerURL("").Config()
// set only some values:
//   b.Host().Brand("").Config()
type ConsoleServerCLIConfigBuilder struct {
	host              string
	logoutRedirectURL string
	brand             operatorv1.Brand
	docURL            string
	apiServerURL      string
	statusPageID      string
	customProductName string
	customLogoFile    string
	cliDownloadURLs   *CLIDownloadURLs
}

func (b *ConsoleServerCLIConfigBuilder) Host(host string) *ConsoleServerCLIConfigBuilder {
	b.host = host
	return b
}
func (b *ConsoleServerCLIConfigBuilder) LogoutURL(logoutRedirectURL string) *ConsoleServerCLIConfigBuilder {
	b.logoutRedirectURL = logoutRedirectURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) Brand(brand operatorv1.Brand) *ConsoleServerCLIConfigBuilder {
	b.brand = brand
	return b
}
func (b *ConsoleServerCLIConfigBuilder) DocURL(docURL string) *ConsoleServerCLIConfigBuilder {
	b.docURL = docURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) APIServerURL(apiServerURL string) *ConsoleServerCLIConfigBuilder {
	b.apiServerURL = apiServerURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CLIDownloadURL(cliDownloadURL string) *ConsoleServerCLIConfigBuilder {
	if cliDownloadURL != "" {
		b.cliDownloadURLs = &CLIDownloadURLs{
			LinuxDownloadURL:   util.GetDownloadURL(cliDownloadURL, api.LinuxPlatformKey, "oc.tar"),
			MacDownloadURL:     util.GetDownloadURL(cliDownloadURL, api.MacPlatformKey, "oc.zip"),
			WindowsDownloadURL: util.GetDownloadURL(cliDownloadURL, api.WindowsPlatformKey, "oc.zip"),
		}
		return b
	}
	b.cliDownloadURLs = nil
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomProductName(customProductName string) *ConsoleServerCLIConfigBuilder {
	b.customProductName = customProductName
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomLogoFile(customLogoFile string) *ConsoleServerCLIConfigBuilder {
	if customLogoFile != "" {
		b.customLogoFile = "/var/logo/" + customLogoFile // append path here to prevent customLogoFile from always being just /var/logo/
	}
	return b
}

func (b *ConsoleServerCLIConfigBuilder) StatusPageID(id string) *ConsoleServerCLIConfigBuilder {
	b.statusPageID = id
	return b
}

func (b *ConsoleServerCLIConfigBuilder) Config() Config {
	return Config{
		Kind:          "ConsoleConfig",
		APIVersion:    "console.openshift.io/v1",
		Auth:          b.authServer(),
		ClusterInfo:   b.clusterInfo(),
		Customization: b.customization(),
		ServingInfo:   b.servingInfo(),
		Providers:     b.providers(),
	}
}

func (b *ConsoleServerCLIConfigBuilder) ConfigYAML() (consoleConfigYAML []byte, marshallError error) {
	conf := b.Config()
	yml, err := yaml.Marshal(conf)
	if err != nil {
		klog.V(4).Infof("could not create config yaml %v", err)
		return nil, err
	}
	return yml, nil
}

func (b *ConsoleServerCLIConfigBuilder) servingInfo() ServingInfo {
	return ServingInfo{
		BindAddress: "https://0.0.0.0:8443",
		CertFile:    certFilePath,
		KeyFile:     keyFilePath,
	}
}

func (b *ConsoleServerCLIConfigBuilder) clusterInfo() ClusterInfo {
	conf := ClusterInfo{
		ConsoleBasePath: "",
	}

	if len(b.apiServerURL) > 0 {
		conf.MasterPublicURL = b.apiServerURL
	}
	if len(b.host) > 0 {
		conf.ConsoleBaseAddress = util.HTTPS(b.host)
	}
	if b.cliDownloadURLs != nil {
		conf.CLIDownloadURLs = b.cliDownloadURLs
	}
	return conf
}

func (b *ConsoleServerCLIConfigBuilder) authServer() Auth {
	conf := Auth{
		ClientID:            api.OpenShiftConsoleName,
		ClientSecretFile:    clientSecretFilePath,
		OAuthEndpointCAFile: oauthEndpointCAFilePath,
	}
	if len(b.logoutRedirectURL) > 0 {
		conf.LogoutRedirect = b.logoutRedirectURL
	}
	return conf
}

func (b *ConsoleServerCLIConfigBuilder) customization() Customization {
	conf := Customization{}
	if len(b.brand) > 0 {
		conf.Branding = string(b.brand)
	}
	if len(b.docURL) > 0 {
		conf.DocumentationBaseURL = b.docURL
	}
	if len(b.customProductName) > 0 {
		conf.CustomProductName = b.customProductName
	}
	if len(b.customLogoFile) > 0 {
		conf.CustomLogoFile = b.customLogoFile
	}
	return conf

}

func (b *ConsoleServerCLIConfigBuilder) providers() Providers {
	if len(b.statusPageID) > 0 {
		return Providers{
			StatuspageID: b.statusPageID,
		}
	}
	return Providers{}
}
