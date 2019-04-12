package controller

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"

	ocontroller "github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-ca-operator/pkg/boilerplate/controller"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
	"github.com/openshift/service-ca-operator/pkg/controller/servingcert/cryptoextensions"
)

type serviceServingCertController struct {
	serviceClient kcoreclient.ServicesGetter
	secretClient  kcoreclient.SecretsGetter

	serviceLister listers.ServiceLister
	secretLister  listers.SecretLister

	ca         *crypto.CA
	dnsSuffix  string
	maxRetries int

	// standard controller loop
	// services that need to be checked
	controller.Runner

	// syncHandler does the work. It's factored out for unit testing
	syncHandler controller.SyncFunc
}

func NewServiceServingCertController(services informers.ServiceInformer, secrets informers.SecretInformer, serviceClient kcoreclient.ServicesGetter, secretClient kcoreclient.SecretsGetter, ca *crypto.CA, dnsSuffix string) controller.Runner {
	sc := &serviceServingCertController{
		serviceClient: serviceClient,
		secretClient:  secretClient,

		serviceLister: services.Lister(),
		secretLister:  secrets.Lister(),

		ca:         ca,
		dnsSuffix:  dnsSuffix,
		maxRetries: 10,
	}

	sc.syncHandler = sc.syncService

	sc.Runner = controller.New("ServiceServingCertController", sc,
		controller.WithInformer(services, controller.FilterFuncs{
			AddFunc: func(obj metav1.Object) bool {
				return true // TODO we should filter these based on annotations
			},
			UpdateFunc: func(oldObj, newObj metav1.Object) bool {
				return true // TODO we should filter these based on annotations
			},
			// TODO we may want to best effort handle deletes and clean up the secrets
		}),
		controller.WithInformer(secrets, controller.FilterFuncs{
			ParentFunc: func(obj metav1.Object) (namespace, name string) {
				secret := obj.(*corev1.Secret)
				serviceName, _ := toServiceName(secret)
				return secret.Namespace, serviceName
			},
			DeleteFunc: sc.deleteSecret,
		}),
	)

	return sc
}

// deleteSecret handles the case when the service certificate secret is manually removed.
// In that case the secret will be automatically recreated.
func (sc *serviceServingCertController) deleteSecret(obj metav1.Object) bool {
	secret := obj.(*corev1.Secret)
	serviceName, ok := toServiceName(secret)
	if !ok {
		return false
	}
	service, err := sc.serviceLister.Services(secret.Namespace).Get(serviceName)
	if kapierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get service %s/%s: %v", secret.Namespace, serviceName, err))
		return false
	}
	glog.V(4).Infof("recreating secret for service %s/%s", service.Namespace, service.Name)
	return true
}

func (sc *serviceServingCertController) Key(namespace, name string) (metav1.Object, error) {
	return sc.serviceLister.Services(namespace).Get(name)
}

func (sc *serviceServingCertController) Sync(obj metav1.Object) error {
	// need another layer of indirection so that tests can stub out syncHandler
	return sc.syncHandler(obj)
}

func (sc *serviceServingCertController) syncService(obj metav1.Object) error {
	sharedService := obj.(*corev1.Service)

	if !sc.requiresCertGeneration(sharedService) {
		return nil
	}

	// make a copy to avoid mutating cache state
	serviceCopy := sharedService.DeepCopy()
	return sc.generateCert(serviceCopy)
}

func (sc *serviceServingCertController) generateCert(serviceCopy *corev1.Service) error {
	glog.V(4).Infof("generating new cert for %s/%s", serviceCopy.GetNamespace(), serviceCopy.GetName())
	if serviceCopy.Annotations == nil {
		serviceCopy.Annotations = map[string]string{}
	}

	secret := toBaseSecret(serviceCopy)
	if err := toRequiredSecret(sc.dnsSuffix, sc.ca, serviceCopy, secret); err != nil {
		return err
	}

	_, err := sc.secretClient.Secrets(serviceCopy.Namespace).Create(secret)
	if err != nil && !kapierrors.IsAlreadyExists(err) {
		return sc.updateServiceFailure(serviceCopy, err)
	}
	if kapierrors.IsAlreadyExists(err) {
		actualSecret, err := sc.secretClient.Secrets(serviceCopy.Namespace).Get(secret.Name, metav1.GetOptions{})
		if err != nil {
			return sc.updateServiceFailure(serviceCopy, err)
		}

		if !uidsEqual(actualSecret, serviceCopy) {
			uidErr := fmt.Errorf("secret %s/%s does not have corresponding service UID %v", actualSecret.GetNamespace(), actualSecret.GetName(), serviceCopy.UID)
			return sc.updateServiceFailure(serviceCopy, uidErr)
		}
		glog.V(4).Infof("renewing cert in existing secret %s/%s", secret.GetNamespace(), secret.GetName())
		// Actually update the secret in the regeneration case (the secret already exists but we want to update to a new cert).
		_, updateErr := sc.secretClient.Secrets(secret.GetNamespace()).Update(secret)
		if updateErr != nil {
			return sc.updateServiceFailure(serviceCopy, updateErr)
		}
	}

	sc.resetServiceAnnotations(serviceCopy)
	_, err = sc.serviceClient.Services(serviceCopy.Namespace).Update(serviceCopy)

	return err
}

func getNumFailures(service *corev1.Service) int {
	numFailuresString := service.Annotations[api.ServingCertErrorNumAnnotation]
	if len(numFailuresString) == 0 {
		numFailuresString = service.Annotations[api.AlphaServingCertErrorNumAnnotation]
		if len(numFailuresString) == 0 {
			return 0
		}
	}

	numFailures, err := strconv.Atoi(numFailuresString)
	if err != nil {
		return 0
	}

	return numFailures
}

func (sc *serviceServingCertController) requiresCertGeneration(service *corev1.Service) bool {
	// check the secret since it could not have been created yet
	secretName := service.Annotations[api.ServingCertSecretAnnotation]
	if len(secretName) == 0 {
		secretName = service.Annotations[api.AlphaServingCertSecretAnnotation]
		if len(secretName) == 0 {
			return false
		}
	}

	_, err := sc.secretLister.Secrets(service.Namespace).Get(secretName)
	if kapierrors.IsNotFound(err) {
		// we have not created the secret yet
		return true
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get the secret %s/%s: %v", service.Namespace, secretName, err))
		return false
	}

	// check to see if the service was updated by us
	if service.Annotations[api.ServingCertCreatedByAnnotation] == sc.commonName() || service.Annotations[api.AlphaServingCertCreatedByAnnotation] == sc.commonName() {
		return false
	}
	// we have failed too many times on this service, give up
	if getNumFailures(service) >= sc.maxRetries {
		return false
	}

	// the secret exists but the service was either not updated to include the correct created
	// by annotation or it does not match what we expect (i.e. the certificate has been rotated)
	return true
}

func (sc *serviceServingCertController) commonName() string {
	return sc.ca.Config.Certs[0].Subject.CommonName
}

// updateServiceFailure updates the service's error annotations with err.
// Returns the passed in err normally, or nil if the amount of failures has hit the max. This is so it can act as a
// return to the sync method.
func (sc *serviceServingCertController) updateServiceFailure(service *corev1.Service, err error) error {
	setErrAnnotation(service, err)
	incrementFailureNumAnnotation(service)
	_, updateErr := sc.serviceClient.Services(service.Namespace).Update(service)
	if updateErr != nil {
		glog.V(4).Infof("warning: failed to update failure annotations on service %s: %v", service.Name, updateErr)
	}
	// Past the max retries means we've handled this failure enough, so forget it from the queue.
	if updateErr == nil && getNumFailures(service) >= sc.maxRetries {
		return nil
	}

	// Return the original error.
	return err
}

// Sets the service CA common name and clears any errors.
func (sc *serviceServingCertController) resetServiceAnnotations(service *corev1.Service) {
	service.Annotations[api.AlphaServingCertCreatedByAnnotation] = sc.commonName()
	service.Annotations[api.ServingCertCreatedByAnnotation] = sc.commonName()
	delete(service.Annotations, api.AlphaServingCertErrorAnnotation)
	delete(service.Annotations, api.AlphaServingCertErrorNumAnnotation)
	delete(service.Annotations, api.ServingCertErrorAnnotation)
	delete(service.Annotations, api.ServingCertErrorNumAnnotation)
}

func ownerRef(service *corev1.Service) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       service.Name,
		UID:        service.UID,
	}
}

func toBaseSecret(service *corev1.Service) *corev1.Secret {
	// Use beta annotations
	if _, ok := service.Annotations[api.ServingCertSecretAnnotation]; ok {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Annotations[api.ServingCertSecretAnnotation],
				Namespace: service.Namespace,
				Annotations: map[string]string{
					api.ServiceUIDAnnotation:  string(service.UID),
					api.ServiceNameAnnotation: service.Name,
				},
			},
			Type: corev1.SecretTypeTLS,
		}
	}
	// Use alpha annotations
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Annotations[api.AlphaServingCertSecretAnnotation],
			Namespace: service.Namespace,
			Annotations: map[string]string{
				api.AlphaServiceUIDAnnotation:  string(service.UID),
				api.AlphaServiceNameAnnotation: service.Name,
			},
		},
		Type: corev1.SecretTypeTLS,
	}
}

func getServingCert(dnsSuffix string, ca *crypto.CA, service *corev1.Service) (*crypto.TLSCertificateConfig, error) {
	dnsName := service.Name + "." + service.Namespace + ".svc"
	fqDNSName := dnsName + "." + dnsSuffix
	certificateLifetime := 365 * 2 // 2 years
	servingCert, err := ca.MakeServerCert(
		sets.NewString(dnsName, fqDNSName),
		certificateLifetime,
		cryptoextensions.ServiceServerCertificateExtensionV1(service),
	)
	if err != nil {
		return nil, err
	}
	return servingCert, nil
}

func toRequiredSecret(dnsSuffix string, ca *crypto.CA, service *corev1.Service, secretCopy *corev1.Secret) error {
	servingCert, err := getServingCert(dnsSuffix, ca, service)
	if err != nil {
		return err
	}
	certBytes, keyBytes, err := servingCert.GetPEMBytes()
	if err != nil {
		return err
	}
	if secretCopy.Annotations == nil {
		secretCopy.Annotations = map[string]string{}
	}
	// let garbage collector cleanup map allocation, for simplicity
	secretCopy.Data = map[string][]byte{
		corev1.TLSCertKey:       certBytes,
		corev1.TLSPrivateKeyKey: keyBytes,
	}

	secretCopy.Annotations[api.AlphaServingCertExpiryAnnotation] = servingCert.Certs[0].NotAfter.Format(time.RFC3339)
	secretCopy.Annotations[api.ServingCertExpiryAnnotation] = servingCert.Certs[0].NotAfter.Format(time.RFC3339)

	ocontroller.EnsureOwnerRef(secretCopy, ownerRef(service))

	return nil
}

func setErrAnnotation(service *corev1.Service, err error) {
	service.Annotations[api.ServingCertErrorAnnotation] = err.Error()
	service.Annotations[api.AlphaServingCertErrorAnnotation] = err.Error()
}

func incrementFailureNumAnnotation(service *corev1.Service) {
	numFailure := strconv.Itoa(getNumFailures(service) + 1)
	service.Annotations[api.ServingCertErrorNumAnnotation] = numFailure
	service.Annotations[api.AlphaServingCertErrorNumAnnotation] = numFailure
}

func uidsEqual(secret *corev1.Secret, service *corev1.Service) bool {
	suid := string(service.UID)
	return secret.Annotations[api.AlphaServiceUIDAnnotation] == suid ||
		secret.Annotations[api.ServiceUIDAnnotation] == suid
}
