package e2e

import (
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	deploymentsub "github.com/openshift/console-operator/pkg/console/deployment"
	"github.com/openshift/console-operator/pkg/testframework"
)

func setupLoggingTestCase(t *testing.T) *testframework.Clientset {
	client := testframework.MustNewClientset(t, nil)
	testframework.MustManageConsole(t, client)
	testframework.MustNormalLogLevel(t, client)
	return client
}

// TestDebugLogLevel sets 'Debug' LogLevel on the console operator and tests
// if '--log-level=*=DEBUG' flag is set on the console deployment
func TestDebugLogLevel(t *testing.T) {
	client := setupLoggingTestCase(t)
	defer testframework.SetLogLevel(t, client, operatorsv1.Normal)

	err := testframework.SetLogLevel(t, client, operatorsv1.Debug)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := testframework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	flagToTest := deploymentsub.GetLogLevelFlag(operatorsv1.Debug)
	if !isFlagInCommand(t, deployment.Spec.Template.Spec.Containers[0].Command, flagToTest) {
		t.Fatalf("error: flag (%s) not found in command %v \n", flagToTest, deployment.Spec.Template.Spec.Containers[0].Command)
	}
}

// TestTraceLogLevel sets 'Trace' LogLevel on the console operator and tests
// if '--log-level=*=TRACE' flag is set on the console deployment
func TestTraceLogLevel(t *testing.T) {
	client := setupLoggingTestCase(t)
	defer testframework.SetLogLevel(t, client, operatorsv1.Normal)

	err := testframework.SetLogLevel(t, client, operatorsv1.Trace)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := testframework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	flagToTest := deploymentsub.GetLogLevelFlag(operatorsv1.Trace)
	if !isFlagInCommand(t, deployment.Spec.Template.Spec.Containers[0].Command, flagToTest) {
		t.Fatalf("error: flag (%s) not found in command %v \n", flagToTest, deployment.Spec.Template.Spec.Containers[0].Command)
	}
}

// TestTraceLogLevel sets 'TraceAll' LogLevel on the console operator and tests
// if '--log-level=*=TRACE' flag is set on the console deployment
func TestTraceAllLogLevel(t *testing.T) {
	client := setupLoggingTestCase(t)
	defer testframework.SetLogLevel(t, client, operatorsv1.Normal)

	err := testframework.SetLogLevel(t, client, operatorsv1.TraceAll)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := testframework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	flagToTest := deploymentsub.GetLogLevelFlag(operatorsv1.TraceAll)
	if !isFlagInCommand(t, deployment.Spec.Template.Spec.Containers[0].Command, flagToTest) {
		t.Fatalf("error: flag (%s) not found in command %v \n", flagToTest, deployment.Spec.Template.Spec.Containers[0].Command)
	}
}

func isFlagInCommand(t *testing.T, command []string, loggingFlag string) bool {
	t.Logf("checking if '%s' flag is set on the console deployment container command...", loggingFlag)
	for _, flag := range command {
		if flag == loggingFlag {
			return true
		}
	}
	return false
}
