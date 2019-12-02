package framework

import (
	"errors"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	err := wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		count++
		t.Log(fmt.Sprintf("running %d time(s)", count))
		err, ok := ConsolePodsRunning(clientset)
		t.Logf(fmt.Sprintf("is ok: %t, why? %s", ok, err))
		if err != nil {
			return ok, err
		}
		// Try until timeout... can we just return both?
		return ok, nil
	})

	if err != nil {
		t.Log(fmt.Sprintf("ran %d time(s)", count))
		t.Fatalf("console pods have not settled (%s)", err)
	}
}

func ConsolePodsRunning(clientset *ClientSet) (err error, ok bool) {
	deployment, err := clientset.Apps.Deployments(api.OpenShiftConsoleNamespace).Get(api.OpenShiftConsoleName, v1.GetOptions{})
	if err != nil {
		return err, false
	}

	// if all is well, exit early
	if deploymentsub.AllPodsHealthy(deployment) {
		return nil, true
	}

	fmt.Printf("something wrong, %d/%d replicas ready of expected %d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas, deployment.Status.Replicas)

	if deployment.Status.Replicas > 2 {
		// todo: may or may not be a bad thing, we need to investigate further
		//    this may simply indicate churn, new pods are rolling out, old pods being terminated
		//    do we need to consider this?
	}
	pods, err := clientset.Core.Pods(api.OpenShiftConsoleNamespace).List(v1.ListOptions{
		LabelSelector: "app=console,component=ui",
	})
	if err != nil {
		return err, false
	}

	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Running != nil {
				fmt.Printf("container %s is running\n", status.Name)
			}
			if status.State.Terminated != nil {
				fmt.Printf("container %s is terminated\n", status.Name)
			}
			if status.State.Waiting != nil {
				fmt.Printf("container %s is %s because %s\n", status.Name, status.State.Waiting.Reason, status.State.Waiting.Message)
				return errors.New(status.State.Waiting.Message), false
			}
		}

	}
	return nil, true
}
