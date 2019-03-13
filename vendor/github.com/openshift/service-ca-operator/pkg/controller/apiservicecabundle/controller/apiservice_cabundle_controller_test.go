package controller

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	apiregistrationapiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiserviceclientfake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	apiservicelister "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"

	"github.com/openshift/service-ca-operator/pkg/controller/api"
)

func validateActions(t *testing.T, expectedActionsNum int, actions []clienttesting.Action) {
	if len(actions) != expectedActionsNum {
		t.Fatal(spew.Sdump(actions))
	}
	if expectedActionsNum == 0 {
		return
	}
	if !actions[0].Matches("update", "apiservices") {
		t.Error(spew.Sdump(actions))
	}
	actual := actions[0].(clienttesting.UpdateAction).GetObject().(*apiregistrationapiv1.APIService)
	if expected := "content"; string(actual.Spec.CABundle) != expected {
		t.Error(diff.ObjectDiff(expected, actual))
	}
}

func TestSyncAPIService(t *testing.T) {
	tests := []struct {
		name                string
		startingAPIServices []runtime.Object
		key                 string
		caBundle            []byte
		expectedActionsNum  int
	}{
		{
			name:               "missing",
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 0,
		},
		{
			name: "requested and empty",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.AlphaInjectCABundleAnnotationName: "true"}},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 1,
		},
		{
			name: "requested and nochange",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.AlphaInjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("content"),
					},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 0,
		},
		{
			name: "requested and different",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.AlphaInjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("old"),
					},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 1,
		},
		{
			name: "requested and empty beta",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"}},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 1,
		},
		{
			name: "requested and nochange beta",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("content"),
					},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 0,
		},
		{
			name: "requested and different beta",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("old"),
					},
				},
			},
			key:                "foo",
			caBundle:           []byte("content"),
			expectedActionsNum: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := apiserviceclientfake.NewSimpleClientset(tc.startingAPIServices...)
			index := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, apiService := range tc.startingAPIServices {
				index.Add(apiService)
			}

			c := &serviceServingCertUpdateController{
				apiServiceLister: apiservicelister.NewAPIServiceLister(index),
				apiServiceClient: fakeClient.ApiregistrationV1(),
				caBundle:         tc.caBundle,
			}

			obj, err := c.Key("", tc.key)
			if err == nil {
				if err := c.Sync(obj); err != nil {
					t.Fatal(err)
				}
			}

			validateActions(t, tc.expectedActionsNum, fakeClient.Actions())
		})
	}
}
