package e2e_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeset "k8s.io/client-go/kubernetes"

	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/testframework"
)

var (
	kubeClient *kubeset.Clientset
)

func TestMain(m *testing.M) {

	kubeconfig, err := testframework.GetConfig()
	if err != nil {
		fmt.Printf("unable to get kubeconfig: %s", err)
		os.Exit(1)
	}

	kubeClient, err = kubeset.NewForConfig(kubeconfig)
	if err != nil {
		fmt.Printf("%#v", err)
		os.Exit(1)
	}

	// e2e test job does not guarantee our operator is up before
	// launching the test, so we need to do so.
	fmt.Println("checking for console-operator availability")
	err = waitForOperator()
	if err != nil {
		fmt.Println("failed waiting for operator to start")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func waitForOperator() error {
	depClient := kubeClient.AppsV1().Deployments(consoleapi.OpenShiftConsoleOperatorNamespace)
	err := wait.PollImmediate(1*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := depClient.Get(consoleapi.OpenShiftConsoleOperator, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("error waiting for operator deployment to exist: %v\n", err)
			return false, nil
		}
		fmt.Println("found operator deployment")
		return true, nil
	})
	return err
}
