package util

import (
	"fmt"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	proxyv1alpha1 "open-cluster-management.io/cluster-proxy/pkg/apis/proxy/v1alpha1"
	proxyscheme "open-cluster-management.io/cluster-proxy/pkg/generated/clientset/versioned/scheme"
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

func LogYaml(obj runtime.Object) {
	// REALLY NOISY, but handy for debugging:
	// deployJSON, err := json.Marshal(d)
	objYaml, err := yaml.Marshal(obj)
	if err != nil {
		klog.V(4).Infoln("failed to show yaml in log")
	} else {
		klog.V(4).Infof("%v", string(objYaml))
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

// TODO remove when we update library-go to a version that includes this
// borrowed from library-go
// https://github.com/openshift/library-go/blob/master/pkg/operator/resource/resourceread/unstructured.go
func ReadUnstructuredOrDie(objBytes []byte) *unstructured.Unstructured {
	udi, _, err := scheme.Codecs.UniversalDecoder().Decode(objBytes, nil, &unstructured.Unstructured{})
	if err != nil {
		panic(err)
	}
	return udi.(*unstructured.Unstructured)
}

func ReadManagedProxyServiceResolverOrDie(objBytes []byte) *proxyv1alpha1.ManagedProxyServiceResolver {
	groupVersionKind := proxyv1alpha1.GroupVersion.WithKind("ManagedProxyServiceResolver")
	resource, _, err := proxyscheme.Codecs.UniversalDecoder().Decode(objBytes, &groupVersionKind, &proxyv1alpha1.ManagedProxyServiceResolver{})
	if err != nil {
		panic(err)
	}
	return resource.(*proxyv1alpha1.ManagedProxyServiceResolver)
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
