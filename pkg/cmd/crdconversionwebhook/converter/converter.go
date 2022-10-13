package converter

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	v1 "github.com/openshift/api/console/v1"
	v1alpha1 "github.com/openshift/api/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/api"
)

func convertCRD(object *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, metav1.Status) {
	originalObject := object.DeepCopy()
	fromVersion := object.GetAPIVersion()
	convertedObject := &unstructured.Unstructured{}
	var err error

	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	switch object.GetAPIVersion() {
	case "console.openshift.io/v1alpha1":
		switch toVersion {
		case "console.openshift.io/v1":
			klog.Infof("converting %q object from 'console.openshift.io/v1alpha1' into 'console.openshift.io/v1'", originalObject.GetName())
			convertedObject, err = convertPluginV1alpha1ToV1(originalObject)
			if err != nil {
				return nil, statusErrorWithMessage("error converting %q object from 'console.openshift.io/v1alpha1' into 'console.openshift.io/v1: %v'", originalObject.GetName(), err)
			}
		default:
			return nil, statusErrorWithMessage("unexpected conversion version %q", toVersion)
		}
	case "console.openshift.io/v1":
		switch toVersion {
		case "console.openshift.io/v1alpha1":
			klog.Infof("converting %q object from 'console.openshift.io/v1' into 'console.openshift.io/v1alpha1'", originalObject.GetName())

			convertedObject, err = convertPluginV1ToV1alpha1(originalObject)
			if err != nil {
				return nil, statusErrorWithMessage("error converting %q object from 'console.openshift.io/v1' into 'console.openshift.io/v1alpha1: %v'", originalObject.GetName(), err)
			}
		default:
			return nil, statusErrorWithMessage("unexpected conversion version %q", toVersion)
		}
	default:
		return nil, statusErrorWithMessage("unexpected conversion version %q", fromVersion)
	}

	return convertedObject, statusSucceed()
}

func convertPluginV1ToV1alpha1(convertedObject *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	v1alpha1Plugin := &v1alpha1.ConsolePlugin{
		TypeMeta: metav1.TypeMeta{Kind: "ConsolePlugin", APIVersion: "console.openshift.io/v1alpha1"},
	}
	v1Plugin := &v1.ConsolePlugin{}

	// convert unstructured object to v1 ConsolePlugin
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(convertedObject.Object, v1Plugin)
	if err != nil {
		return nil, err
	}

	// metadata
	v1alpha1Plugin.ObjectMeta = v1Plugin.ObjectMeta

	// displayName
	v1alpha1Plugin.Spec.DisplayName = v1Plugin.Spec.DisplayName

	// i18n
	annotations := v1Plugin.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if v1Plugin.Spec.I18n.LoadType == v1.Preload {
		annotations[api.V1Alpha1PluginI18nAnnotation] = "true"
	} else if v1Plugin.Spec.I18n.LoadType == v1.Lazy {
		annotations[api.V1Alpha1PluginI18nAnnotation] = "false"
	}
	v1alpha1Plugin.SetAnnotations(annotations)

	// backend -> service
	// we only support backend type 'Service' for now, so it's always true
	if v1Plugin.Spec.Backend.Type == v1.Service {
		v1alpha1Plugin.Spec.Service = v1alpha1.ConsolePluginService(*v1Plugin.Spec.Backend.Service)
	}

	//proxy
	v1alpha1Proxies := []v1alpha1.ConsolePluginProxy{}
	for _, proxy := range v1Plugin.Spec.Proxy {
		v1alpha1Proxy := v1alpha1.ConsolePluginProxy{}

		// we only support proxy type 'Service' for now, so it's always true
		if proxy.Endpoint.Type == v1.ProxyTypeService {
			v1alpha1Proxy.Type = v1alpha1.ProxyTypeService
			v1alpha1Proxy.Service = v1alpha1.ConsolePluginProxyServiceConfig(*proxy.Endpoint.Service)
			v1alpha1Proxy.Alias = proxy.Alias
			v1alpha1Proxy.CACertificate = proxy.CACertificate
		}

		if proxy.Authorization == v1.UserToken {
			v1alpha1Proxy.Authorize = true
		}

		if proxy.Authorization == v1.None {
			v1alpha1Proxy.Authorize = false
		}

		v1alpha1Proxies = append(v1alpha1Proxies, v1alpha1Proxy)
	}
	v1alpha1Plugin.Spec.Proxy = v1alpha1Proxies

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(v1alpha1Plugin)
	if err != nil {
		return nil, err
	}

	convertedObject = &unstructured.Unstructured{
		Object: raw,
	}

	return convertedObject, nil
}

func convertPluginV1alpha1ToV1(convertedObject *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	v1Plugin := &v1.ConsolePlugin{
		TypeMeta: metav1.TypeMeta{Kind: "ConsolePlugin", APIVersion: "console.openshift.io/v1"},
	}
	v1alpha1Plugin := &v1alpha1.ConsolePlugin{}

	// convert unstructured object to v1alpha1 ConsolePlugin
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(convertedObject.Object, v1alpha1Plugin)
	if err != nil {
		return nil, err
	}

	// metadata
	v1Plugin.ObjectMeta = v1alpha1Plugin.ObjectMeta

	// displayName
	v1Plugin.Spec = v1.ConsolePluginSpec{
		DisplayName: v1alpha1Plugin.Spec.DisplayName,
	}

	// i18n
	updatedV1alpha1PluginAnnotations := v1alpha1Plugin.GetAnnotations()
	if v := updatedV1alpha1PluginAnnotations[api.V1Alpha1PluginI18nAnnotation]; v == "true" {
		v1Plugin.Spec.I18n.LoadType = v1.Preload
	} else if v == "false" {
		v1Plugin.Spec.I18n.LoadType = v1.Lazy
	}
	delete(updatedV1alpha1PluginAnnotations, api.V1Alpha1PluginI18nAnnotation)
	v1Plugin.SetAnnotations(updatedV1alpha1PluginAnnotations)

	// service -> backend
	// v1alpha1 can only be type 'Service'
	v1Plugin.Spec.Backend = v1.ConsolePluginBackend{
		Service: (*v1.ConsolePluginService)(&v1alpha1Plugin.Spec.Service),
		Type:    v1.Service,
	}

	// proxy
	// v1alpha1 endpoint can only be type 'Service'
	v1Proxies := []v1.ConsolePluginProxy{}
	for _, proxy := range v1alpha1Plugin.Spec.Proxy {
		v1Proxy := v1.ConsolePluginProxy{
			Alias:         proxy.Alias,
			CACertificate: proxy.CACertificate,
			Endpoint: v1.ConsolePluginProxyEndpoint{
				Service: (*v1.ConsolePluginProxyServiceConfig)(&proxy.Service),
				Type:    v1.ProxyTypeService,
			},
		}

		if proxy.Authorize == true {
			v1Proxy.Authorization = v1.UserToken
		}
		if proxy.Authorize == false {
			v1Proxy.Authorization = v1.None
		}

		v1Proxies = append(v1Proxies, v1Proxy)
	}
	v1Plugin.Spec.Proxy = v1Proxies

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(v1Plugin)
	if err != nil {
		return nil, err
	}

	convertedObject = &unstructured.Unstructured{
		Object: raw,
	}

	return convertedObject, nil
}
