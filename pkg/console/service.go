package console

import (
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// this annotation should generate us a certificate
	serviceServingCertSignerAnnotationKey = "service.alpha.openshift.io/serving-cert-secret-name"
)

func newConsoleService(cr *v1alpha1.Console) *corev1.Service {
	labels := sharedLabels()
	meta := sharedMeta()
	meta.Annotations = map[string]string{
		serviceServingCertSignerAnnotationKey: consoleServingCertName,
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       consolePortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       consolePort,
					TargetPort: intstr.FromInt(consoleTargetPort),
				},
			},
			Selector:        labels,
			Type:            "ClusterIP",
			SessionAffinity: "None",
		},
	}
	addOwnerRef(service, ownerRefFrom(cr))
	logrus.Info("Creating console service manifest")
	return service
}

func CreateService(cr *v1alpha1.Console) {
	svc := newConsoleService(cr)
	if err := sdk.Create(svc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console service : %v", err)
	} else {
		logrus.Info("created console service")
		// logYaml(svc)
	}
}
