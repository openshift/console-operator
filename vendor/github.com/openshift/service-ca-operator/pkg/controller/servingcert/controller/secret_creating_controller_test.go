package controller

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
	"github.com/openshift/service-ca-operator/pkg/controller/servingcert/cryptoextensions"
)

func controllerSetup(startingObjects []runtime.Object, t *testing.T) ( /*caName*/ string, *fake.Clientset, *watch.RaceFreeFakeWatcher, *watch.RaceFreeFakeWatcher, *serviceServingCertController, informers.SharedInformerFactory) {
	certDir, err := ioutil.TempDir("", "serving-cert-unit-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	signerName := fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
	ca, err := crypto.MakeSelfSignedCA(
		path.Join(certDir, "service-signer.crt"),
		path.Join(certDir, "service-signer.key"),
		path.Join(certDir, "service-signer.serial"),
		signerName,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeclient := fake.NewSimpleClientset(startingObjects...)
	fakeWatch := watch.NewRaceFreeFake()
	fakeSecretWatch := watch.NewRaceFreeFake()
	kubeclient.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(clientgotesting.CreateAction).GetObject(), nil
	})
	kubeclient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(clientgotesting.UpdateAction).GetObject(), nil
	})
	kubeclient.PrependWatchReactor("services", clientgotesting.DefaultWatchReactor(fakeWatch, nil))
	kubeclient.PrependWatchReactor("secrets", clientgotesting.DefaultWatchReactor(fakeSecretWatch, nil))

	informerFactory := informers.NewSharedInformerFactory(kubeclient, 0)

	controller := NewServiceServingCertController(
		informerFactory.Core().V1().Services(),
		informerFactory.Core().V1().Secrets(),
		kubeclient.Core(), kubeclient.Core(), ca, "cluster.local",
	)

	return signerName, kubeclient, fakeWatch, fakeSecretWatch, controller.(*serviceServingCertController), informerFactory
}

func checkGeneratedCertificate(t *testing.T, certData []byte, service *v1.Service) {
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Errorf("PEM block not found in secret")
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Errorf("expected valid certificate in first position: %v", err)
		return
	}

	if len(cert.DNSNames) != 2 {
		t.Errorf("unexpected DNSNames: %v", cert.DNSNames)
	}
	for _, s := range cert.DNSNames {
		switch s {
		case fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace):
		default:
			t.Errorf("unexpected DNSNames: %v", cert.DNSNames)
		}
	}

	found := true
	for _, ext := range cert.Extensions {
		if cryptoextensions.OpenShiftServerSigningServiceUIDOID.Equal(ext.Id) {
			var value string
			if _, err := asn1.Unmarshal(ext.Value, &value); err != nil {
				t.Errorf("unable to parse certificate extension: %v", ext.Value)
				continue
			}
			if value != string(service.UID) {
				t.Errorf("unexpected extension value: %v", value)
				continue
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unable to find service UID certificate extension in cert: %#v", cert)
	}
}

func TestBasicControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedServiceAnnotations := map[string]string{
		api.AlphaServingCertSecretAnnotation:    expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}
	expectedSecretAnnotations := map[string]string{
		api.AlphaServiceUIDAnnotation:  serviceUID,
		api.AlphaServiceNameAnnotation: serviceName,
	}
	namespace := "ns"

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't created.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestBasicControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedServiceAnnotations := map[string]string{
		api.ServingCertSecretAnnotation:         expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}
	expectedSecretAnnotations := map[string]string{
		api.ServiceUIDAnnotation:  serviceUID,
		api.ServiceNameAnnotation: serviceName,
	}
	namespace := "ns"

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't created.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestAlreadyExistingSecretControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedSecretAnnotations := map[string]string{api.AlphaServiceUIDAnnotation: serviceUID, api.AlphaServiceNameAnnotation: serviceName}
	namespace := "ns"

	existingSecret := &v1.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = v1.SecretTypeTLS
	existingSecret.Annotations = expectedSecretAnnotations

	caName, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{existingSecret}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewAlreadyExists(v1.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.AlphaServingCertSecretAnnotation:    expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}

}

func TestAlreadyExistingSecretControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedSecretAnnotations := map[string]string{api.AlphaServiceUIDAnnotation: serviceUID, api.AlphaServiceNameAnnotation: serviceName}
	namespace := "ns"

	existingSecret := &v1.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = v1.SecretTypeTLS
	existingSecret.Annotations = expectedSecretAnnotations

	caName, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{existingSecret}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewAlreadyExists(v1.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.ServingCertSecretAnnotation:         expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}

}

func TestAlreadyExistingSecretForDifferentUIDControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := "secret/new-secret references serviceUID wrong-uid, which does not match some-uid"
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	existingSecret := &v1.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = v1.SecretTypeTLS
	existingSecret.Annotations = map[string]string{
		api.AlphaServiceUIDAnnotation:  "wrong-uid",
		api.ServiceUIDAnnotation:       "wrong-uid",
		api.AlphaServiceNameAnnotation: serviceName,
	}

	_, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{existingSecret}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewAlreadyExists(v1.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.AlphaServingCertSecretAnnotation:   expectedSecretName,
		api.AlphaServingCertErrorAnnotation:    expectedError,
		api.ServingCertErrorAnnotation:         expectedError,
		api.AlphaServingCertErrorNumAnnotation: "1",
		api.ServingCertErrorNumAnnotation:      "1",
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestAlreadyExistingSecretForDifferentUIDControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := "secret/new-secret references serviceUID wrong-uid, which does not match some-uid"
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	existingSecret := &v1.Secret{}
	existingSecret.Name = expectedSecretName
	existingSecret.Namespace = namespace
	existingSecret.Type = v1.SecretTypeTLS
	existingSecret.Annotations = map[string]string{
		api.AlphaServiceUIDAnnotation:  "wrong-uid",
		api.ServiceUIDAnnotation:       "wrong-uid",
		api.AlphaServiceNameAnnotation: serviceName,
	}

	_, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{existingSecret}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewAlreadyExists(v1.Resource("secrets"), "new-secret")
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.ServingCertSecretAnnotation:        expectedSecretName,
		api.AlphaServingCertErrorAnnotation:    expectedError,
		api.ServingCertErrorAnnotation:         expectedError,
		api.AlphaServingCertErrorNumAnnotation: "1",
		api.ServingCertErrorNumAnnotation:      "1",
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("get", "secrets"):
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't retrieved.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestSecretCreationErrorControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := `secrets "new-secret" is forbidden: any reason`
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	_, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("secrets"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.AlphaServingCertSecretAnnotation:   expectedSecretName,
		api.AlphaServingCertErrorAnnotation:    expectedError,
		api.ServingCertErrorAnnotation:         expectedError,
		api.AlphaServingCertErrorNumAnnotation: "1",
		api.ServingCertErrorNumAnnotation:      "1",
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestSecretCreationErrorControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedError := `secrets "new-secret" is forbidden: any reason`
	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	_, kubeclient, fakeWatch, _, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	kubeclient.PrependReactor("create", "secrets", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("secrets"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil && err.Error() != expectedError {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedServiceAnnotations := map[string]string{
		api.ServingCertSecretAnnotation:        expectedSecretName,
		api.AlphaServingCertErrorAnnotation:    expectedError,
		api.ServingCertErrorAnnotation:         expectedError,
		api.AlphaServingCertErrorNumAnnotation: "1",
		api.ServingCertErrorNumAnnotation:      "1",
	}

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}
}

func TestSkipGenerationControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	caName, kubeclient, fakeWatch, fakeSecretWatch, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	kubeclient.PrependReactor("update", "service", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Service{}, kapierrors.NewForbidden(v1.Resource("fdsa"), "new-service", fmt.Errorf("any service reason"))
	})
	kubeclient.PrependReactor("create", "secret", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	kubeclient.PrependReactor("update", "secret", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}

	secretToAdd := &v1.Secret{}
	secretToAdd.Name = expectedSecretName
	secretToAdd.Namespace = namespace
	fakeSecretWatch.Add(secretToAdd)

	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName, api.AlphaServingCertErrorAnnotation: "any-error", api.AlphaServingCertErrorNumAnnotation: "11"}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}

	kubeclient.ClearActions()
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName, api.AlphaServingCertCreatedByAnnotation: caName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}
}

func TestSkipGenerationControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	namespace := "ns"

	caName, kubeclient, fakeWatch, fakeSecretWatch, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	kubeclient.PrependReactor("update", "service", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Service{}, kapierrors.NewForbidden(v1.Resource("fdsa"), "new-service", fmt.Errorf("any service reason"))
	})
	kubeclient.PrependReactor("create", "secret", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	kubeclient.PrependReactor("update", "secret", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Secret{}, kapierrors.NewForbidden(v1.Resource("asdf"), "new-secret", fmt.Errorf("any reason"))
	})
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}

	secretToAdd := &v1.Secret{}
	secretToAdd.Name = expectedSecretName
	secretToAdd.Namespace = namespace
	fakeSecretWatch.Add(secretToAdd)

	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName, api.ServingCertErrorAnnotation: "any-error", api.ServingCertErrorNumAnnotation: "11"}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}

	kubeclient.ClearActions()
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName, api.ServingCertCreatedByAnnotation: caName}
	fakeWatch.Add(serviceToAdd)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch action.GetVerb() {
		case "update", "create":
			t.Errorf("no mutation expected, but we got %v", action)
		}
	}
}

func TestRecreateSecretControllerFlow(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, fakeSecretWatch, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedServiceAnnotations := map[string]string{
		api.AlphaServingCertSecretAnnotation:    expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}
	expectedSecretAnnotations := map[string]string{api.AlphaServiceUIDAnnotation: serviceUID, api.AlphaServiceNameAnnotation: serviceName}
	expectedOwnerRef := []metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: serviceName, UID: types.UID(serviceUID)}}
	namespace := "ns"

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.AlphaServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	secretToDelete := &v1.Secret{}
	secretToDelete.Name = expectedSecretName
	secretToDelete.Namespace = namespace
	secretToDelete.Annotations = map[string]string{api.AlphaServiceNameAnnotation: serviceName}

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}
			if !equality.Semantic.DeepEqual(expectedOwnerRef, newSecret.OwnerReferences) {
				t.Errorf("expected %v, got %v", expectedOwnerRef, newSecret.OwnerReferences)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't created.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}

	kubeclient.ClearActions()
	fakeSecretWatch.Add(secretToDelete)
	fakeSecretWatch.Delete(secretToDelete)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}
}

func TestRecreateSecretControllerFlowBetaAnnotation(t *testing.T) {
	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	caName, kubeclient, fakeWatch, fakeSecretWatch, controller, informerFactory := controllerSetup([]runtime.Object{}, t)
	controller.syncHandler = func(obj metav1.Object) error {
		defer func() { received <- true }()

		err := controller.syncService(obj)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}
	informerFactory.Start(stopChannel)
	go controller.Run(1, stopChannel)

	expectedSecretName := "new-secret"
	serviceName := "svc-name"
	serviceUID := "some-uid"
	expectedServiceAnnotations := map[string]string{
		api.ServingCertSecretAnnotation:         expectedSecretName,
		api.AlphaServingCertCreatedByAnnotation: caName,
		api.ServingCertCreatedByAnnotation:      caName,
	}
	expectedSecretAnnotations := map[string]string{api.ServiceUIDAnnotation: serviceUID, api.ServiceNameAnnotation: serviceName}
	expectedOwnerRef := []metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: serviceName, UID: types.UID(serviceUID)}}
	namespace := "ns"

	serviceToAdd := &v1.Service{}
	serviceToAdd.Name = serviceName
	serviceToAdd.Namespace = namespace
	serviceToAdd.UID = types.UID(serviceUID)
	serviceToAdd.Annotations = map[string]string{api.ServingCertSecretAnnotation: expectedSecretName}
	fakeWatch.Add(serviceToAdd)

	secretToDelete := &v1.Secret{}
	secretToDelete.Name = expectedSecretName
	secretToDelete.Namespace = namespace
	secretToDelete.Annotations = map[string]string{api.AlphaServiceNameAnnotation: serviceName}

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	foundSecret := false
	foundServiceUpdate := false
	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}
			if !equality.Semantic.DeepEqual(expectedOwnerRef, newSecret.OwnerReferences) {
				t.Errorf("expected %v, got %v", expectedOwnerRef, newSecret.OwnerReferences)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}

	if !foundSecret {
		t.Errorf("secret wasn't created.  Got %v\n", kubeclient.Actions())
	}
	if !foundServiceUpdate {
		t.Errorf("service wasn't updated.  Got %v\n", kubeclient.Actions())
	}

	kubeclient.ClearActions()
	fakeSecretWatch.Add(secretToDelete)
	fakeSecretWatch.Delete(secretToDelete)

	t.Log("waiting to reach syncHandler")
	select {
	case <-received:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to call into syncService")
	}

	for _, action := range kubeclient.Actions() {
		switch {
		case action.Matches("create", "secrets"):
			createSecret := action.(clientgotesting.CreateAction)
			newSecret := createSecret.GetObject().(*v1.Secret)
			if newSecret.Name != expectedSecretName {
				t.Errorf("expected %v, got %v", expectedSecretName, newSecret.Name)
				continue
			}
			if newSecret.Namespace != namespace {
				t.Errorf("expected %v, got %v", namespace, newSecret.Namespace)
				continue
			}
			delete(newSecret.Annotations, api.AlphaServingCertExpiryAnnotation)
			delete(newSecret.Annotations, api.ServingCertExpiryAnnotation)
			if !reflect.DeepEqual(newSecret.Annotations, expectedSecretAnnotations) {
				t.Errorf("expected %v, got %v", expectedSecretAnnotations, newSecret.Annotations)
				continue
			}

			checkGeneratedCertificate(t, newSecret.Data["tls.crt"], serviceToAdd)
			foundSecret = true

		case action.Matches("update", "services"):
			updateService := action.(clientgotesting.UpdateAction)
			service := updateService.GetObject().(*v1.Service)
			if !reflect.DeepEqual(service.Annotations, expectedServiceAnnotations) {
				t.Errorf("expected %v, got %v", expectedServiceAnnotations, service.Annotations)
				continue
			}
			foundServiceUpdate = true

		}
	}
}
