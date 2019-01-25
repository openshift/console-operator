package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/apis/console/v1"
)

func SharedLabels() map[string]string {
	return map[string]string{
		"app": api.OpenShiftConsoleName,
	}
}

func LabelsForConsole() map[string]string {
	baseLabels := SharedLabels()

	extraLabels := map[string]string{
		"component": "ui",
	}
	// we want to deduplicate, so doing these two loops.
	allLabels := map[string]string{}

	for key, value := range baseLabels {
		allLabels[key] = value
	}
	for key, value := range extraLabels {
		allLabels[key] = value
	}
	return allLabels
}

func SharedMeta() v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      api.OpenShiftConsoleName,
		Namespace: api.OpenShiftConsoleName,
		Labels:    SharedLabels(),
	}
}

func LogYaml(obj runtime.Object) {
	// REALLY NOISY, but handy for debugging:
	// deployJSON, err := json.Marshal(d)
	objYaml, err := yaml.Marshal(obj)
	if err != nil {
		logrus.Info("failed to show yaml in log")
	}
	logrus.Infof("%v", string(objYaml))
}

// objects can have more than one ownerRef, potentially
func AddOwnerRef(obj v1.Object, ownerRef *v1.OwnerReference) {
	// TODO: find the library-go equivalent of this and replace
	// currently errs out with something like:
	// failed with: ConfigMap "console-config" is invalid: [metadata.ownerReferences.apiVersion: Invalid value: "": version must not be empty, metadata.ownerReferences.kind: Invalid value: "": kind must not be empty]
	//if obj != nil {
	//	if ownerRef != nil {
	//		obj.SetOwnerReferences(append(obj.GetOwnerReferences(), *ownerRef))
	//	}
	//}
}

// func RemoveOwnerRef
func OwnerRefFrom(cr *v1.Console) *v1.OwnerReference {

	if cr != nil {
		truthy := true
		return &v1.OwnerReference{
			APIVersion: cr.APIVersion,
			Kind:       cr.Kind,
			Name:       cr.Name,
			UID:        cr.UID,
			Controller: &truthy,
		}
	}
	return nil
}

// borrowed from library-go
// https://github.com/openshift/library-go/blob/master/pkg/operator/v1alpha1helpers/helpers.go
func GetImageEnv() string {
	return os.Getenv("IMAGE")
}

// TODO: technically, this should take targetPort from route.spec.port.targetPort
func HTTPS(host string) string {
	protocol := "https://"
	if host == "" {
		logrus.Infof("util.HTTPS() cannot accept an empty string.")
		return ""
	}
	if strings.HasPrefix(host, protocol) {
		return host
	}
	secured := fmt.Sprintf("%s%s", protocol, host)
	return secured
}
