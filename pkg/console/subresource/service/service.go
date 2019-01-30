package service

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
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
)

func DefaultService(cr *v1alpha1.ConsoleOperatorConfig) *v1.Service {
	labels := util.LabelsForConsole()
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleShortName
	meta.Annotations = map[string]string{
		ServingCertSecretAnnotation: ConsoleServingCertName,
	}
	service := Stub()
	service.Spec = v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{
				Name:       consolePortName,
				Protocol:   v1.ProtocolTCP,
				Port:       consolePort,
				TargetPort: intstr.FromInt(consoleTargetPort),
			},
		},
		Selector:        labels,
		Type:            "ClusterIP",
		SessionAffinity: "None",
	}

	util.AddOwnerRef(service, util.OwnerRefFrom(cr))
	return service
}

func Stub() *v1.Service {
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleShortName
	meta.Annotations = map[string]string{
		ServingCertSecretAnnotation: ConsoleServingCertName,
	}
	service := &v1.Service{
		ObjectMeta: meta,
	}
	return service
}
