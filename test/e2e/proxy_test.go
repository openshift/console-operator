package e2e

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/test/e2e/framework"

	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func setupProxyTest(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	clientSet, operatorConfig := framework.StandardSetup(t)
	// If we don't pause the mco, it will actually apply this fake proxy config to nodes
	pauseAllMachineConfigPools(t, clientSet)
	return clientSet, operatorConfig
}

func cleanupProxyTest(t *testing.T, clientSet *framework.ClientSet) {
	framework.ResetClusterProxyConfig(clientSet)
	// Make sure mco gets to react to us resetting the proxy config, and then unpause
	unpauseAllMachineConfigPools(t, clientSet)
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

// pauseAllMachineConfigPools pauses all the MachineConfigPools so they can't apply the bogus proxy config
// while we're doing this test. See: https://issues.redhat.com/browse/OCPBUGS-5780
func pauseAllMachineConfigPools(t *testing.T, clientSet *framework.ClientSet) error {

	pools, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pool := range pools.Items {
		t.Logf("pausing pool %s...", pool.Name)
		pausePatch := []byte("{\"spec\":{\"paused\":true}}")
		if _, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().Patch(context.Background(), pool.Name, types.MergePatchType, pausePatch, metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// pauseAllMachineConfigPools unpauses all the MachineConfigPools once the pool config has settled back to the original after
// the example proxy configuration has been removed. See: https://issues.redhat.com/browse/OCPBUGS-5780
func unpauseAllMachineConfigPools(t *testing.T, clientSet *framework.ClientSet) error {

	pools, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// The MCO didn't roll out the proxy config because it was paused, but it did probably render a new machineconfig that got selected for the pool,
	// we need to wait for controllerconfig to get updated again (the MCO to react to the proxy config removal) and move back to the previous config
	for _, pool := range pools.Items {
		t.Logf("waiting on pool %s to revert the proxy change...", pool.Name)

		if err := wait.PollImmediate(2*time.Second, 2*time.Minute, func() (bool, error) {
			mcp, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// paused == not updated, not updating, not degraded
			// safe to unpause without changes == updated, not updating, not degraded
			if mcfgv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) &&
				mcfgv1.IsMachineConfigPoolConditionFalse(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdating) &&
				mcfgv1.IsMachineConfigPoolConditionFalse(mcp.Status.Conditions, mcfgv1.MachineConfigPoolDegraded) {
				return true, nil
			}
			return false, nil
		}); err != nil {
			t.Errorf("Machine config pool never went back to normal after config revert: %v", err)
		}
	}

	// Unpause the pools
	for _, pool := range pools.Items {
		t.Logf("unpausing pool %s...", pool.Name)

		pausePatch := []byte("{\"spec\":{\"paused\":false}}")
		if _, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().Patch(context.Background(), pool.Name, types.MergePatchType, pausePatch, metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	return nil
}
