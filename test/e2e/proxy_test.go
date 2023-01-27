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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var originalConfig = map[string]string{}

func setupProxyTest(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	clientSet, operatorConfig := framework.StandardSetup(t)
	// If we don't pause the mco, it will actually apply this fake proxy config to nodes
	pauseAllMachineConfigPools(t, clientSet)
	return clientSet, operatorConfig
}

func cleanupProxyTest(t *testing.T, clientSet *framework.ClientSet) {
	waitforMachineConfig(t, clientSet)
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

	originalConfig = make(map[string]string)
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
		originalConfig[pool.Name] = pool.Spec.Configuration.Name
	}

	return nil
}

// waitFormachineConfig waits to make sure the pool receieves the new config containing the proxy. This is necessary because sometimes when
// things happen too fast, we pause and unpause before the config is even rendered, and it still get applied to the nodes.
// See: https://issues.redhat.com/browse/OCPBUGS-5780
func waitforMachineConfig(t *testing.T, clientSet *framework.ClientSet) error {

	pools, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pool := range pools.Items {
		t.Logf("waiting on pool %s to acknowledge the proxy change...", pool.Name)

		if err := wait.PollImmediate(2*time.Second, 2*time.Minute, func() (bool, error) {
			mcp, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// If we're still on the config we started on, we didn't get the new one yet
			if mcp.Spec.Configuration.Name == originalConfig[mcp.Name] {
				return false, nil
			}
			return true, nil
		}); err != nil {
			t.Fatalf("Machine config pool %s never acknowledged the proxy change: %v", pool.Name, err)
		}
	}

	return nil
}

// unpauseAllMachineConfigPools unpauses all the MachineConfigPools once the pool config has settled back to the original after
// the proxy configuration has been removed. See: https://issues.redhat.com/browse/OCPBUGS-5780
func unpauseAllMachineConfigPools(t *testing.T, clientSet *framework.ClientSet) error {

	pools, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pool := range pools.Items {
		t.Logf("waiting on pool %s to revert the proxy change...", pool.Name)

		if err := wait.PollImmediate(2*time.Second, 2*time.Minute, func() (bool, error) {
			mcp, err := clientSet.MachineConfig.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// If we're not back on the old config yet, it hasn't processed it
			// TODO(jkyros): this will break if something else pushes a machineconfig change while we're paused,
			// but that does not happen during this test, so it is okay -- just be aware this might not work
			// well for "parallel" tests, this expects things to happen sequentially.
			if mcp.Spec.Configuration.Name != originalConfig[mcp.Name] {
				return false, nil
			}
			return true, nil
		}); err != nil {
			t.Fatalf("Machine config pool %s never reverted the proxy change: %v", pool.Name, err)
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
