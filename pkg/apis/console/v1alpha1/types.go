package v1alpha1

import (
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

const (
	defaultBaseImage = "quay.io/openshift/origin-console"
	defaultVersion   = "latest"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Console `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Console struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConsoleSpec   `json:"spec"`
	Status            ConsoleStatus `json:"status,omitempty"`
}

// if/when changed, be sure to regenerate generated code:
// 	operator-sdk generate k8s
type ConsoleSpec struct {
	Paused bool `json:"paused"`
	// Count is the number of Console replicas
	Count     int32  `json:"count,omitempty"`
	BaseImage string `json:"baseImage"`
	Version   string `json:"version"`
	// 0 error, 1 warn 2 info? default debug
	// aiming at consistency with this for now:
	// https://github.com/openshift/cluster-image-registry-operator/blob/master/pkg/generate/podtemplatespec.go#L21
	Logging *LoggingConfig `json:"logging"`
}
type ConsoleStatus struct {
	Paused bool `json:"paused"`
	// the hostname assigned by the cluster after the route is created
	Host string `json:"host"`
}

// TODO: decide on values for this, consider
// consistency with cluster-image-registry-operator,
// though it doesn't seem to set a default level if empty
// https://github.com/openshift/cluster-image-registry-operator/blob/80976754e1467f2303a3ff352fe5955cf58d12f7/pkg/generate/podtemplatespec.go#L21
type LoggingConfig struct {
	Level int `json:"level,omitempty"`
}

// good idea to set the defaults
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/apis/vault/v1alpha1/types.go#L39
func (c *Console) SetDefaults() bool {
	changed := false
	if len(c.Spec.BaseImage) == 0 {
		c.Spec.BaseImage = "docker.io/openshift/origin-console"
		changed = true
	}
	if c.Spec.Count == 0 {
		c.Spec.Count = 1
		changed = true
	}
	if len(c.Spec.BaseImage) == 0 {
		c.Spec.BaseImage = defaultBaseImage
		changed = true
	}
	if len(c.Spec.Version) == 0 {
		c.Spec.Version = defaultVersion
		changed = true
	}
	if c.Spec.Logging == nil {
		c.Spec.Logging = &LoggingConfig{
			Level: 0,
		}
	}
	return changed
}

func (c *Console) UpdateHost(route *routev1.Route) {
	logrus.Infof("updating console status with host %s", route.Spec.Host)
	c.Status.Host = route.Spec.Host
	err := sdk.Update(c)
	if err != nil {
		logrus.Errorf("Failed to update console status with host %s", err)
	}
}

// may want to create secret names here, etc.
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/apis/vault/v1alpha1/types.go#L65

// Required actions
// https://github.com/openshift/elasticsearch-operator/blob/master/pkg/apis/elasticsearch/v1alpha1/types.go#L97
type ConsoleRequiredAction string

const (
	ConsoleActionRestartNeeded      ConsoleRequiredAction = "RestartNeeded"
	ConsoleActionInterventionNeeded ConsoleRequiredAction = "InterventionNeeded"
	ConsoleActionNone               ConsoleRequiredAction = "ConsoleOK"
)
