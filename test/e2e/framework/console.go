package framework

import (
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/console-operator/pkg/api"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

const (
	pollFrequency   = 1 * time.Second
	pollStandardMax = 30 * time.Second
	pollLongMax     = 120 * time.Second
)

// Similar to the console-operator needing to reach a settled state,
// we want to know when the console itself is settled.
// This means that the console pods are not in flux, the expected number
// of replicas have been created and are running and healthy.
func ConsolePodsMustSettle(t *testing.T, clientset *ClientSet) {
	count := 0
	t.Log("ConsolePodsMustSettle()")
	err := wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		count++
		t.Log(fmt.Sprintf("running %d time(s)", count))
		err, ok := ConsolePodsRunning(t, clientset)
		t.Logf(fmt.Sprintf("is ok: %t, because:%s", ok, err))
		if err != nil {
			t.Log("continue due to error")
			return false, err
		}
		t.Log("stop, no error")
		// Try until timeout... can we just return both?
		return true, nil
	})

	if err != nil {
		t.Log(fmt.Sprintf("ran %d time(s)", count))
		t.Fatalf("console pods have not settled (%s)", err)
	}
}

func ConsolePodsRunning(t *testing.T, clientset *ClientSet) (err error, ok bool) {
	deployment, err := clientset.Apps.Deployments(api.OpenShiftConsoleNamespace).Get(api.OpenShiftConsoleName, metav1.GetOptions{})
	if err != nil {
		t.Fatal("GET deployment error")
		return err, false
	}

	// if all is well, exit early
	if deploymentsub.AllPodsHealthy(deployment) {
		t.Log("All pods healthy")
		return nil, true
	}
	// otherwise, lets report whats wrong
	fmt.Printf("%d/%d replicas ready of expected %d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas, deployment.Status.Replicas)

	pods, err := clientset.Core.Pods(api.OpenShiftConsoleNamespace).List(metav1.ListOptions{
		LabelSelector: "app=console,component=ui",
	})
	if err != nil {
		t.Fatal("GET pods error")
		return err, false
	}

	for _, pod := range pods.Items {
		t.Log(fmt.Sprintf("pod %s is %s", pod.Name, pod.Status.Phase))
		if pod.Status.Phase == corev1.PodRunning {
			continue
		}
		t.Log(fmt.Sprintf(">> Pod %s is %s\n", pod.Name, pod.Status.Phase))

		for _, status := range pod.Status.ContainerStatuses {
			// TODO: remove, just logging for checks
			if status.State.Running != nil {
				t.Log(fmt.Sprintf(">>> container %s is running\n", status.Name))
			}
			// TODO: remove, just logging for checks
			if status.State.Terminated != nil {
				t.Log(fmt.Sprintf(">>> container %s is terminated\n", status.Name))
			}
			if status.State.Waiting != nil {
				if len(status.State.Waiting.Message) != 0 {
					t.Log(fmt.Sprintf(">>> container %s is %s because %s\n", status.Name, status.State.Waiting.Reason, status.State.Waiting.Message))
				} else {
					t.Log(fmt.Sprintf(">>> container %s is %s\n", status.Name, status.State.Waiting.Reason))
				}
				return errors.New(status.State.Waiting.Message), false
			}
		}

	}
	return nil, true
}
