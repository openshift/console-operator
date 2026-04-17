---
name: e2e-test-create
description: Scaffold new e2e tests following console-operator patterns and best practices
tags: [create, testing, e2e, scaffold]
---

# E2E Test Creation Skill

Create new end-to-end tests in `test/e2e/` directory following established patterns.

## Test Template

```go
package e2e

import (
    "context"
    "testing"
    "time"

    operatorv1 "github.com/openshift/api/operator/v1"
    "github.com/openshift/console-operator/pkg/api"
    "github.com/openshift/console-operator/test/e2e/framework"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/util/retry"
)

func TestYourFeature(t *testing.T) {
    // Setup client
    client := framework.MustNewClientset(t, nil)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    // Get initial operator config
    operatorConfig := framework.GetOperatorConfig(t, client.OperatorClient)
    if operatorConfig.Spec.ManagementState != operatorv1.Managed {
        t.Skip("Skipping test - operator not in Managed state")
    }

    // Test implementation
    t.Run("SubtestName", func(t *testing.T) {
        // Setup test resources
        defer cleanupResources(t, client)

        // Perform operations
        err := updateOperatorConfig(ctx, client, func(config *operatorv1.Console) {
            // Modify config
        })
        if err != nil {
            t.Fatalf("failed to update config: %v", err)
        }

        // Wait for expected state
        err = waitForCondition(ctx, t, client)
        if err != nil {
            t.Fatalf("expected condition not met: %v", err)
        }

        // Verify result
        verifyExpectedState(t, client)
    })
}

// Helper functions
func updateOperatorConfig(
    ctx context.Context,
    client *framework.Clientset,
    modify func(*operatorv1.Console),
) error {
    return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
        config, err := client.OperatorClient.Consoles().Get(
            ctx, api.ConfigResourceName, metav1.GetOptions{},
        )
        if err != nil {
            return err
        }
        modify(config)
        _, err = client.OperatorClient.Consoles().Update(
            ctx, config, metav1.UpdateOptions{},
        )
        return err
    })
}

func waitForCondition(
    ctx context.Context,
    t *testing.T,
    client *framework.Clientset,
) error {
    return wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
        // Check for expected condition
        deployment, err := client.KubeClient.AppsV1().Deployments(
            api.OpenShiftConsoleNamespace,
        ).Get(ctx, api.OpenShiftConsoleName, metav1.GetOptions{})
        if err != nil {
            return false, err
        }
        return deployment.Status.ReadyReplicas > 0, nil
    })
}

func cleanupResources(t *testing.T, client *framework.Clientset) {
    ctx := context.Background()
    // Clean up in reverse order of creation
    // Ignore NotFound errors
}

func verifyExpectedState(t *testing.T, client *framework.Clientset) {
    // Make assertions about final state
}
```

## Table-Driven Test Template

```go
func TestMultipleScenarios(t *testing.T) {
    testCases := []struct {
        name          string
        setup         func(*operatorv1.Console)
        expectedValue string
        expectError   bool
    }{
        {
            name: "scenario 1",
            setup: func(config *operatorv1.Console) {
                config.Spec.SomeField = "value1"
            },
            expectedValue: "result1",
            expectError:   false,
        },
        {
            name: "scenario 2",
            setup: func(config *operatorv1.Console) {
                config.Spec.SomeField = "value2"
            },
            expectedValue: "result2",
            expectError:   false,
        },
    }

    client := framework.MustNewClientset(t, nil)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Setup
            err := updateOperatorConfig(ctx, client, tc.setup)
            if err != nil {
                t.Fatalf("setup failed: %v", err)
            }
            defer resetConfig(t, client)

            // Execute test
            result, err := performOperation(ctx, client)

            // Verify
            if tc.expectError && err == nil {
                t.Error("expected error but got none")
            }
            if !tc.expectError && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tc.expectedValue {
                t.Errorf("expected %v, got %v", tc.expectedValue, result)
            }
        })
    }
}
```

## Plugin Test Pattern

```go
func TestConsolePlugin(t *testing.T) {
    client := framework.MustNewClientset(t, nil)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    pluginName := "test-plugin"
    
    // Create plugin
    plugin := &consolev1.ConsolePlugin{
        ObjectMeta: metav1.ObjectMeta{
            Name: pluginName,
        },
        Spec: consolev1.ConsolePluginSpec{
            DisplayName: "Test Plugin",
            Service: consolev1.ConsolePluginService{
                Name:      "plugin-service",
                Namespace: "plugin-namespace",
                Port:      8443,
                BasePath:  "/",
            },
        },
    }

    _, err := client.ConsoleClient.ConsoleV1().ConsolePlugins().Create(
        ctx, plugin, metav1.CreateOptions{},
    )
    if err != nil {
        t.Fatalf("failed to create plugin: %v", err)
    }

    defer func() {
        err := client.ConsoleClient.ConsoleV1().ConsolePlugins().Delete(
            ctx, pluginName, metav1.DeleteOptions{},
        )
        if err != nil {
            t.Logf("cleanup warning: %v", err)
        }
    }()

    // Wait for plugin to be processed
    err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
        deployment, err := client.KubeClient.AppsV1().Deployments(
            api.OpenShiftConsoleNamespace,
        ).Get(ctx, api.OpenShiftConsoleName, metav1.GetOptions{})
        if err != nil {
            return false, err
        }
        // Check for plugin in deployment
        return containsPlugin(deployment, pluginName), nil
    })
    if err != nil {
        t.Fatalf("plugin not reflected in deployment: %v", err)
    }
}
```

## Best Practices Checklist

When creating e2e tests:

- [ ] Use `framework.MustNewClientset(t, nil)` for client setup
- [ ] Create context with timeout (`context.WithTimeout`)
- [ ] Add cleanup with `defer`
- [ ] Use `retry.RetryOnConflict` for config updates
- [ ] Use `wait.Poll` for async operations (not `time.Sleep`)
- [ ] Write helpful error messages with context
- [ ] Add subtests for different scenarios
- [ ] Test both success and failure paths
- [ ] Verify operator responds to config changes
- [ ] Check ClusterOperator status conditions
- [ ] Clean up in reverse order of creation
- [ ] Handle `IsNotFound` errors in cleanup
- [ ] Use table-driven tests for multiple similar cases

## Common Imports

```go
import (
    "context"
    "testing"
    "time"

    configv1 "github.com/openshift/api/config/v1"
    consolev1 "github.com/openshift/api/console/v1"
    operatorv1 "github.com/openshift/api/operator/v1"
    "github.com/openshift/console-operator/pkg/api"
    "github.com/openshift/console-operator/test/e2e/framework"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/util/retry"
)
```

## Output

Generate complete, runnable test code with:
1. Proper imports
2. Framework setup
3. Resource creation
4. Cleanup logic
5. Wait patterns
6. Assertions
7. Helpful error messages
8. Comments explaining the test flow
