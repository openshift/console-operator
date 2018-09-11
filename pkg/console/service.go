package console

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// this annotation should generate us a certificate
	serviceServingCertSignerAnnotationKey = "service.alpha.openshift.io/serving-cert-secret-name"
)


func newConsoleService() *corev1.Service {
	labels := sharedLabels()
	meta := sharedMeta()
	meta.Annotations = map[string]string{
		serviceServingCertSignerAnnotationKey: consoleServingCertName,
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind: "Service",
		},
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: consolePortName,
					Protocol: corev1.ProtocolTCP,
					Port: consolePort,
					TargetPort: intstr.FromInt(consoleTargetPort),
				},
			},
			Selector: labels,
			Type: "ClusterIP",
			SessionAffinity: "None",
		},
	}
	logrus.Info("Creating console service manifest")
	return service
}