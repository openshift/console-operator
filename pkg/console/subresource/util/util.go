package util

import (
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
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

func LabelsForDownloads() map[string]string {
	return map[string]string{
		"app":       api.OpenShiftConsoleName,
		"component": api.DownloadsResourceName,
	}
}

func SharedMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        api.OpenShiftConsoleName,
		Namespace:   api.OpenShiftConsoleNamespace,
		Labels:      SharedLabels(),
		Annotations: map[string]string{},
	}
}

// objects can have more than one ownerRef, potentially
func AddOwnerRef(obj metav1.Object, ownerRef *metav1.OwnerReference) {
	if obj != nil && ownerRef != nil {
		obj.SetOwnerReferences(append(obj.GetOwnerReferences(), *ownerRef))
	}
}

// func RemoveOwnerRef
func OwnerRefFrom(cr *operatorv1.Console) *metav1.OwnerReference {
	if cr == nil {
		return nil
	}

	truthy := true
	return &metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "Console",
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &truthy,
	}
}

// borrowed from library-go
// https://github.com/openshift/library-go/blob/master/pkg/operator/v1alpha1helpers/helpers.go
func GetImageEnv(envName string) string {
	return os.Getenv(envName)
}

// TODO: technically, this should take targetPort from route.spec.port.targetPort
func HTTPS(host string) string {
	protocol := "https://"
	if host == "" {
		klog.V(4).Infoln("util.HTTPS() cannot accept an empty string.")
		return ""
	}
	if strings.HasPrefix(host, protocol) {
		return host
	}
	secured := fmt.Sprintf("%s%s", protocol, host)
	return secured
}

func RemoveDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func PluginNamesToStrings(pluginNames []operatorv1.PluginName) []string {
	var pluginNameStrings []string
	for _, pluginName := range pluginNames {
		pluginNameStrings = append(pluginNameStrings, string(pluginName))
	}
	return RemoveDuplicateStr(pluginNameStrings)
}
