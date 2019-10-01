package e2e

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupProxyTest(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	clientSet, operatorConfig := framework.StandardSetup(t)
	return clientSet, operatorConfig
}

func cleanupProxyTest(t *testing.T, clientSet *framework.ClientSet) {
	framework.ResetClusterProxyConfig(clientSet)
	framework.StandardCleanup(t, clientSet)
}

func TestProxy(t *testing.T) {
	client, _ := setupProxyTest(t)
	defer cleanupProxyTest(t, client)

	clusterProxyConfig := configv1.ProxySpec{
		NoProxy:    "clusternoproxy.example.com",
		HTTPProxy:  "http://clusterhttpproxy.example.com",
		HTTPSProxy: "https://clusterhttpsproxy.example.com",
	}

	proxyEnvVars := map[string][]corev1.EnvVar{
		"emtpyVars": {
			{Name: "NO_PROXY", Value: "", ValueFrom: nil},
			{Name: "HTTP_PROXY", Value: "", ValueFrom: nil},
			{Name: "HTTPS_PROXY", Value: "", ValueFrom: nil},
		},
		"clusterVars": {
			{Name: "NO_PROXY", Value: clusterProxyConfig.NoProxy, ValueFrom: nil},
			{Name: "HTTP_PROXY", Value: clusterProxyConfig.HTTPProxy, ValueFrom: nil},
			{Name: "HTTPS_PROXY", Value: clusterProxyConfig.HTTPSProxy, ValueFrom: nil},
		},
	}

	consoleDeployment, err := framework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	for _, err = range framework.CheckEnvVars(proxyEnvVars["emptyVars"], consoleDeployment.Spec.Template.Spec.Containers[0].Env, true) {
		t.Errorf("%v", err)
	}

	if err := framework.SetClusterProxyConfig(clusterProxyConfig, client); err != nil {
		t.Errorf("unable to patch cluster proxy instance: %v", err)
	}

	t.Logf("waiting for the new console deployment with proxy environment variables...")
	err = wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (stop bool, err error) {
		newConsoleDeployment, err := framework.GetConsoleDeployment(client)
		if err != nil {
			return false, err
		}
		if errors.IsNotFound(err) {
			return false, nil
		}
		if newConsoleDeployment.Status.ObservedGeneration != consoleDeployment.Status.ObservedGeneration {
			return true, nil
		}
		return false, nil
	})
	err = pollDeploymentForEnv(client, proxyEnvVars["clusterVars"])

	if err != nil {
		t.Fatal(err)
	}
}

func pollDeploymentForEnv(client *framework.ClientSet, clusterVars []corev1.EnvVar) error {
	counter := 0
	maxCount := 60
	err := wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		newConsoleDeployment, err := framework.GetConsoleDeployment(client)
		if err != nil {
			return false, err
		}
		for _, err = range framework.CheckEnvVars(clusterVars, newConsoleDeployment.Spec.Template.Spec.Containers[0].Env, true) {
			if counter == maxCount {
				if err != nil {
					return false, err
				}
			}
			counter++
			return false, nil
		}
		return true, nil
	})

	return err
}
