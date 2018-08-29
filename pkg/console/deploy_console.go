package console

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	// "github.com/ose/Godeps/_workspace/src/k8s.io/kubernetes/pkg/api/v1"
)


func sharedLabels() map[string]string {
	return map[string]string{
		"app": "openshift-console",
	}
}

// similar to how I did this with the helm chart
func labelsForConsole() map[string]string {
	baseLabels := sharedLabels()

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

func sharedMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: "openshift-console", // config? or nah
		Namespace: "bens-project", // config? or nah
		// these can be overridden/mutated
		Labels: sharedLabels(),
	}
}

// might do something with this, if it is meaningful.
// perhaps reconcile() will need to fiddle? maybe not,
// cuz pods should be handled by their deployments...
//func newConsolePod() *corev1.Pod {
//	labels := sharedLabels() // Didn't add them all here...
//	meta := sharedMeta()
//	meta.Labels = labels
//
//	pod := &corev1.Pod{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "apps/v1",
//			Kind:       "Pod",
//		},
//		ObjectMeta: meta,
//		Spec: corev1.PodSpec{
//			Containers: []corev1.Container{
//				{
//					//Name:    "busybox",
//					//Image:   "busybox",
//					//Command: []string{"sleep", "3600"},
//				},
//			},
//		}
//	}
//	return pod
//}

func newConsoleDeployment(cr *v1alpha1.Console) *appsv1.Deployment {
	labels := labelsForConsole()
	meta := sharedMeta()
	replicas := cr.Spec.Count
	image := cr.Spec.Image
	// tack on the deployment specific labels
	meta.Labels = labels

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas, // requires pointer
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					// NOTE: I have no idea why double curlies...
					Containers: []corev1.Container{{
						Image: image,
						Name: "openshift-console",
						Command: []string{
							"/opt/bridge/bin/bridge",
							"--public-dir=/opt/bridge/static",
							"--config=/var/console-config/console-config.yaml",
						},
						// NOTE:
						// resources, limits, cpu, all that good stuff
						// goes here, but lets get something that deploys first!
					}},
				},
			},
		},
	}

	return deployment
}

// Deploy Vault Example:
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L39
func deployConsole(cr *v1alpha1.Console) error {
	// What to do to actually deploy a console?
	d := newConsoleDeployment(cr)
	// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L98
	err := sdk.Create(d)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console deployment : %v", err)
		return err
	}
	return nil
}