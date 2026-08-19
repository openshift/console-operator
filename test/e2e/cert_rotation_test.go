package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

const servingCertAnnotation = "console.openshift.io/serving-cert-secret-version"

// TestCertRotationTriggersRollout verifies that deleting the console-serving-cert
// secret (simulating a service-CA cert rotation) causes the operator to detect
// the new secret and roll out a new console deployment with the updated
// resource version annotation.
func TestCertRotationTriggersRollout(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer framework.StandardCleanup(t, client)

	// 1. Record the current state of the deployment and secret.
	deployment, err := framework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("failed to get console deployment: %v", err)
	}
	oldAnnotation := deployment.Spec.Template.ObjectMeta.Annotations[servingCertAnnotation]
	oldGeneration := deployment.ObjectMeta.Generation
	t.Logf("before rotation: annotation=%q, generation=%d", oldAnnotation, oldGeneration)

	oldSecret, err := client.Core.Secrets(api.TargetNamespace).Get(context.TODO(), api.ConsoleServingCertName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get console-serving-cert secret: %v", err)
	}
	oldSecretRV := oldSecret.ResourceVersion
	t.Logf("before rotation: secret resourceVersion=%q", oldSecretRV)

	// 2. Delete the secret to trigger service-CA to regenerate it with a new resourceVersion.
	t.Log("deleting console-serving-cert secret to simulate cert rotation...")
	err = client.Core.Secrets(api.TargetNamespace).Delete(context.TODO(), api.ConsoleServingCertName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("failed to delete console-serving-cert secret: %v", err)
	}

	// 3. Wait for the secret to be recreated by service-CA with a new resourceVersion.
	t.Log("waiting for service-CA to recreate the secret...")
	var newSecretRV string
	err = wait.PollImmediate(2*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		secret, err := client.Core.Secrets(api.TargetNamespace).Get(context.TODO(), api.ConsoleServingCertName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if secret.ResourceVersion != oldSecretRV {
			newSecretRV = secret.ResourceVersion
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for console-serving-cert secret to be recreated: %v", err)
	}
	t.Logf("secret recreated with new resourceVersion=%q", newSecretRV)

	// 4. Wait for the operator to reconcile and update the deployment annotation.
	t.Log("waiting for operator to update deployment annotation...")
	err = wait.PollImmediate(2*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		dep, err := framework.GetConsoleDeployment(client)
		if err != nil {
			return false, nil
		}
		currentAnnotation := dep.Spec.Template.ObjectMeta.Annotations[servingCertAnnotation]
		return currentAnnotation != oldAnnotation && currentAnnotation != "", nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for deployment annotation to update after cert rotation: %v", err)
	}

	// 5. Verify the new annotation matches the new secret's resourceVersion.
	updatedDeployment, err := framework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("failed to get updated console deployment: %v", err)
	}
	newAnnotation := updatedDeployment.Spec.Template.ObjectMeta.Annotations[servingCertAnnotation]
	t.Logf("after rotation: annotation=%q, generation=%d", newAnnotation, updatedDeployment.ObjectMeta.Generation)

	if newAnnotation != newSecretRV {
		t.Errorf("expected deployment annotation %q to match new secret resourceVersion %q", newAnnotation, newSecretRV)
	}

	if updatedDeployment.ObjectMeta.Generation <= oldGeneration {
		t.Errorf("expected deployment generation to increase after cert rotation: old=%d, new=%d", oldGeneration, updatedDeployment.ObjectMeta.Generation)
	}

	// 6. Wait for operator to settle after the rollout.
	t.Log("waiting for operator to reach settled state...")
	settled, err := framework.WaitForSettledState(t, client, "cert-rotation")
	if err != nil {
		t.Fatalf("operator did not settle after cert rotation: %v", err)
	}
	if !settled {
		t.Error("operator did not reach settled state after cert rotation")
	}
}
