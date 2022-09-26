package converter

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/go-test/deep"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConverter(t *testing.T) {
	cases := []struct {
		originalObject string
		wantedObject   string
	}{
		{
			originalObject: `apiVersion: console.openshift.io/v1alpha1
kind: ConsolePlugin
metadata:
  annotations:
    console.openshift.io/test-annotation: "true"
    console.openshift.io/use-i18n: "true"
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  displayName: plugin
  service:
    name: console-demo-plugin
    namespace: console-demo-plugin
    port: 9001
    basePath: /
  proxy:
  - type: Service
    alias: thanos-querier
    authorize: true
    caCertificate: certContent
    service:
      name: thanos-querier
      namespace: openshift-monitoring
      port: 9091
`,
			wantedObject: `apiVersion: console.openshift.io/v1
kind: ConsolePlugin
metadata:
  annotations:
    console.openshift.io/test-annotation: "true"
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  backend:
    service:
      basePath: /
      name: console-demo-plugin
      namespace: console-demo-plugin
      port: 9001
    type: Service
  displayName: plugin
  i18n:
    loadType: Preload
  proxy:
  - alias: thanos-querier
    authorization: UserToken
    caCertificate: certContent
    endpoint:
      service:
        name: thanos-querier
        namespace: openshift-monitoring
        port: 9091
      type: Service
`,
		},
		{
			originalObject: `apiVersion: console.openshift.io/v1alpha1
kind: ConsolePlugin
metadata:
  annotations:
    console.openshift.io/use-i18n: "false"
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  displayName: plugin
  service:
    name: console-demo-plugin
    namespace: console-demo-plugin
    port: 9001
    basePath: /
  proxy:
  - type: Service
    alias: thanos-querier
    authorize: true
    caCertificate: certContent
    service:
      name: thanos-querier
      namespace: openshift-monitoring
      port: 9091
`,
			wantedObject: `apiVersion: console.openshift.io/v1
kind: ConsolePlugin
metadata:
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  backend:
    service:
      basePath: /
      name: console-demo-plugin
      namespace: console-demo-plugin
      port: 9001
    type: Service
  displayName: plugin
  i18n:
    loadType: Lazy
  proxy:
  - alias: thanos-querier
    authorization: UserToken
    caCertificate: certContent
    endpoint:
      service:
        name: thanos-querier
        namespace: openshift-monitoring
        port: 9091
      type: Service
`,
		},
		{
			originalObject: `apiVersion: console.openshift.io/v1
kind: ConsolePlugin
metadata:
  annotations:
    console.openshift.io/test-annotation: "true"
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  backend:
    service:
      basePath: /
      name: console-demo-plugin
      namespace: console-demo-plugin
      port: 9001
    type: Service
  displayName: plugin
  i18n:
    loadType: Preload
  proxy:
  - alias: thanos-querier
    authorization: UserToken
    caCertificate: certContent
    endpoint:
      service:
        name: thanos-querier
        namespace: openshift-monitoring
        port: 9091
      type: Service
`,
			wantedObject: `apiVersion: console.openshift.io/v1alpha1
kind: ConsolePlugin
metadata:
  annotations:
    console.openshift.io/test-annotation: "true"
    console.openshift.io/use-i18n: "true"
  creationTimestamp: null
  generation: 1
  name: console-plugin
spec:
  displayName: plugin
  proxy:
  - alias: thanos-querier
    authorize: true
    caCertificate: certContent
    service:
      name: thanos-querier
      namespace: openshift-monitoring
      port: 9091
    type: Service
  service:
    basePath: /
    name: console-demo-plugin
    namespace: console-demo-plugin
    port: 9001
`,
		},
	}
	for _, tc := range cases {
		t.Run("ConsolePlugin version convertion test", func(t *testing.T) {
			unstructuredOriginalObject := getUnstructuredObject(t, tc.originalObject)
			unstructuredWantedObject := getUnstructuredObject(t, tc.wantedObject)

			convertedCR, status := convertCRD(unstructuredOriginalObject, unstructuredWantedObject.GetAPIVersion())
			if status.Status == metav1.StatusFailure {
				t.Errorf("error converting object: %s", status.Message)
			}

			rawConvertedCR, err := convertedCR.MarshalJSON()
			if err != nil {
				t.Errorf("error converting marshaling object: %q", err)
			}

			rawYaml, err := yaml.JSONToYAML(rawConvertedCR)
			if err != nil {
				t.Errorf("error converting parsing object: %q", err)
			}

			if diff := deep.Equal(string(rawYaml), tc.wantedObject); diff != nil {
				t.Error(diff)
			}

		})
	}
}

func getUnstructuredObject(t *testing.T, obj string) *unstructured.Unstructured {
	unstructuredObj := &unstructured.Unstructured{}
	jsonObj, _ := yaml.YAMLToJSON([]byte(obj))
	if err := unstructuredObj.UnmarshalJSON(jsonObj); err != nil {
		t.Errorf("error unmarshalling object: %q", err)
	}
	return unstructuredObj
}
