package testframework

import (
	"fmt"
	"testing"

	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"

	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	clientconsolev1alpha1 "github.com/openshift/console-operator/pkg/generated/clientset/versioned/typed/console/v1alpha1"
)

// Clientset is a set of Kubernetes clients.
type Clientset struct {
	clientcorev1.CoreV1Interface
	clientappsv1.AppsV1Interface
	clientconfigv1.ConfigV1Interface
	clientconsolev1alpha1.ConsoleV1alpha1Interface
}

// NewClientset creates a set of Kubernetes clients. The default kubeconfig is
// used if not provided.
func NewClientset(kubeconfig *restclient.Config) (*Clientset, error) {
	var err error
	if kubeconfig == nil {
		kubeconfig, err = GetConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to get kubeconfig: %s", err)
		}
	}

	clientset := &Clientset{}
	clientset.CoreV1Interface, err = clientcorev1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.AppsV1Interface, err = clientappsv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.ConfigV1Interface, err = clientconfigv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.ConsoleV1alpha1Interface, err = clientconsolev1alpha1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

// MustNewClientset is like NewClienset but aborts the test if clienset cannot
// be constructed.
func MustNewClientset(t *testing.T, kubeconfig *restclient.Config) *Clientset {
	clientset, err := NewClientset(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	return clientset
}
