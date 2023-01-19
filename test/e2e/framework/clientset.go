package framework

import (
	"fmt"
	"testing"

	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	restclient "k8s.io/client-go/rest"

	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	consoleclientv1 "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"
	consoleclientv1alpha1 "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1alpha1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	clientroutev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	clientmachineconfigv1 "github.com/openshift/machine-config-operator/pkg/generated/clientset/versioned"
)

// ClientSet is a set of Kubernetes clients.
type ClientSet struct {
	// embedded
	Core                   clientcorev1.CoreV1Interface
	Apps                   clientappsv1.AppsV1Interface
	Routes                 clientroutev1.RouteV1Interface
	MachineConfig          clientmachineconfigv1.Interface
	ConsoleCliDownloads    consoleclientv1.ConsoleCLIDownloadInterface
	ConsoleExternalLogLink consoleclientv1.ConsoleExternalLogLinkInterface
	ConsoleLink            consoleclientv1.ConsoleLinkInterface
	ConsoleNotification    consoleclientv1.ConsoleNotificationInterface
	ConsolePluginV1Alpha1  consoleclientv1alpha1.ConsolePluginInterface
	ConsolePluginV1        consoleclientv1.ConsolePluginInterface
	ConsoleYAMLSample      consoleclientv1.ConsoleYAMLSampleInterface
	Operator               operatorclientv1.ConsolesGetter
	Console                configv1.ConsolesGetter
	ClusterOperator        configv1.ClusterOperatorsGetter
	Proxy                  configv1.ProxiesGetter
	Infrastructure         configv1.InfrastructuresGetter
	Ingress                configv1.IngressesGetter
	PodDisruptionBudget    policyv1client.PodDisruptionBudgetsGetter
}

// NewClientset creates a set of Kubernetes clients. The default kubeconfig is
// used if not provided.
func NewClientset(kubeconfig *restclient.Config) (*ClientSet, error) {
	var err error
	if kubeconfig == nil {
		kubeconfig, err = GetConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to get kubeconfig: %s", err)
		}
	}

	clientset := &ClientSet{}
	clientset.Core, err = clientcorev1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.Apps, err = clientappsv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.Routes, err = clientroutev1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.MachineConfig, err = clientmachineconfigv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	operatorsClient, err := operatorclientv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.Operator = operatorsClient

	configClient, err := configv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset.Console = configClient
	clientset.Proxy = configClient
	clientset.Infrastructure = configClient
	clientset.Ingress = configClient
	clientset.ClusterOperator = configClient

	consoleClient, err := consoleclientv1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	consoleClientAlpha1, err := consoleclientv1alpha1.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset.ConsoleCliDownloads = consoleClient.ConsoleCLIDownloads()
	clientset.ConsoleExternalLogLink = consoleClient.ConsoleExternalLogLinks()
	clientset.ConsoleLink = consoleClient.ConsoleLinks()
	clientset.ConsoleNotification = consoleClient.ConsoleNotifications()
	clientset.ConsoleYAMLSample = consoleClient.ConsoleYAMLSamples()
	clientset.ConsolePluginV1Alpha1 = consoleClientAlpha1.ConsolePlugins()
	clientset.ConsolePluginV1 = consoleClient.ConsolePlugins()

	policyClient, err := policyv1client.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset.PodDisruptionBudget = policyClient

	return clientset, nil
}

// MustNewClientset is like NewClienset but aborts the test if clienset cannot
// be constructed.
func MustNewClientset(t *testing.T, kubeconfig *restclient.Config) *ClientSet {
	clientset, err := NewClientset(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	return clientset
}
