package service

import (
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// this annotation should generate us a serving certificate
	ServingCertSecretAnnotation = "service.alpha.openshift.io/serving-cert-secret-name"
)

const (
	ConsoleServingCertName = "console-serving-cert"
	consolePortName        = "https"
	consolePort            = 443
	consoleTargetPort      = 8443
	sessionAffinity        = "None"
	serviceType            = "ClusterIP"
)

func DefaultService(cr *v1alpha1.Console) *v1.Service {
	labels := util.LabelsForConsole()
	service := Stub()
	service.Spec = v1.ServiceSpec{
		Ports:           ports(),
		Selector:        labels,
		Type:            serviceType,
		SessionAffinity: sessionAffinity,
	}

	util.AddOwnerRef(service, util.OwnerRefFrom(cr))
	return service
}

func Stub() *v1.Service {
	service := &v1.Service{
		ObjectMeta: meta(),
	}
	return service
}

func Validate(svc *v1.Service) (*v1.Service, bool) {
	changed := false

	if svc.ObjectMeta.Annotations[ServingCertSecretAnnotation] != ConsoleServingCertName {
		changed = true
		svc.ObjectMeta.Annotations[ServingCertSecretAnnotation] = ConsoleServingCertName
	}

	if !equality.Semantic.DeepEqual(svc.Spec.Selector, util.LabelsForConsole()) {
		changed = true
		svc.Spec.Selector = util.LabelsForConsole()
	}

	if !equality.Semantic.DeepEqual(svc.Spec.Ports, ports()) {
		changed = true
		svc.Spec.Ports = ports()
	}

	if svc.Spec.SessionAffinity != sessionAffinity {
		changed = true
		svc.Spec.SessionAffinity = sessionAffinity
	}

	if svc.Spec.Type != serviceType {
		changed = true
		svc.Spec.Type = serviceType
	}

	return svc, changed
}

func meta() metav1.ObjectMeta {
	meta := util.SharedMeta()
	meta.Name = controller.OpenShiftConsoleShortName
	meta.Annotations = map[string]string{
		ServingCertSecretAnnotation: ConsoleServingCertName,
	}
	return meta
}

func ports() []v1.ServicePort {
	return []v1.ServicePort{
		{
			Name:       consolePortName,
			Protocol:   v1.ProtocolTCP,
			Port:       consolePort,
			TargetPort: intstr.FromInt(consoleTargetPort),
		},
	}
}
