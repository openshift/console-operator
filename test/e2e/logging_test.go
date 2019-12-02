package e2e

import (
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupLoggingTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanUpLoggingTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

// TestDebugLogLevel sets 'Debug' LogLevel on the console operator and tests
// if '--log-level=*=DEBUG' flag is set on the console deployment
func TestDebugLogLevel(t *testing.T) {
	client, _ := setupLoggingTestCase(t)
	defer cleanUpLoggingTestCase(t, client)

	err := framework.SetLogLevel(t, client, operatorsv1.Debug)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := framework.GetConsoleDeployment(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	flagToTest := deploymentsub.GetLogLevelFlag(operatorsv1.Debug)
	if !isFlagInCommand(t, deployment.Spec.Template.Spec.Containers[0].Command, flagToTest) {
		t.Fatalf("error: flag (%s) not found in command %v \n", flagToTest, deployment.Spec.Template.Spec.Containers[0].Command)
	}

	t.Log("made it this far, should be ok?")
	framework.ConsolePodsMustSettle(t, client)
}

// TestTraceLogLevel sets 'Trace' LogLevel on the console operator and tests
// if '--log-level=*=TRACE' flag is set on the console deployment
func TestTraceLogLevel(t *testing.T) {
	client, _ := setupLoggingTestCase(t)
	defer cleanUpLoggingTestCase(t, client)
	err := framework.SetLogLevel(t, client, operatorsv1.Trace)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := framework.GetConsoleDeployment(client)
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
	client, _ := setupLoggingTestCase(t)
	defer cleanUpLoggingTestCase(t, client)

	err := framework.SetLogLevel(t, client, operatorsv1.TraceAll)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	deployment, err := framework.GetConsoleDeployment(client)
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
