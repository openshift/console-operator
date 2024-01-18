package e2e

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/api"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupDeploymentsReplicasTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupDeploymentsReplicasTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

func TestDeploymentsReplicas(t *testing.T) {
	client, _ := setupDeploymentsReplicasTestCase(t)
	defer cleanupDeploymentsReplicasTestCase(t, client)

	infrastructureConfig, err := client.Infrastructure.Infrastructures().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	var expectedReplicas int32
	if infrastructureConfig.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		expectedReplicas = int32(deploymentsub.SingleNodeConsoleReplicas)
	} else {
		expectedReplicas = int32(deploymentsub.DefaultConsoleReplicas)
	}

	consoleDeployment, err := framework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	downloadsDeployment, err := framework.GetDownloadsDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	for _, deployment := range []*appsv1.Deployment{consoleDeployment, downloadsDeployment} {
		if !reflect.DeepEqual(*deployment.Spec.Replicas, expectedReplicas) {
			t.Fatalf("error: expected %d replicas for %q deployment but has %d", expectedReplicas, deployment.ObjectMeta.Name, *deployment.Spec.Replicas)
		}
	}

}
