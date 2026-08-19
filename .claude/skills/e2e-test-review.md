---
name: e2e-test-review
description: Review e2e tests for console-operator framework patterns, best practices, and anti-patterns
tags: [e2e, testing, review]
---

# E2E Test Review Skill

Review end-to-end tests in `test/e2e/` directory for correctness and best practices.

## When to Use This Skill

- PR modifies `test/e2e/*.go` files
- Reviewing e2e test implementations
- Detecting anti-patterns and missing best practices
- Verifying proper framework usage

---

# Framework Patterns Reference

These are the correct patterns to look for when reviewing e2e tests.

## 1. Setup and Cleanup

**Preferred pattern:**
```go
func TestFeature(t *testing.T) {
    client, operatorConfig := framework.StandardSetup(t)
    defer framework.StandardCleanup(t, client)
    
    // ... test logic
}
```

**Older acceptable pattern:**
```go
client := framework.MustNewClientset(t, nil)
operatorConfig, err := client.Operator.Consoles().Get(
    context.TODO(), api.ConfigResourceName, metav1.GetOptions{},
)
```

**Anti-pattern (flag in reviews):**
```go
// BAD - manual setup without framework
config, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
client, _ := kubernetes.NewForConfig(config)
```

## 2. Wait Patterns

Use `wait.Poll` for eventual consistency (never `time.Sleep`):

```go
err := wait.Poll(5*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
    deployment, err := client.Apps.Deployments(ns).Get(
        ctx, name, metav1.GetOptions{},
    )
    if err != nil {
        return false, err
    }
    return deployment.Status.ReadyReplicas > 0, nil
})
if err != nil {
    t.Fatalf("deployment never became ready: %v", err)
}
```

**Poll interval guidance:**
- Use **5 seconds** for most checks (balances responsiveness with API load)
- Adjust based on what you're waiting for (faster for local state, slower for external resources)

**Anti-pattern (flag in reviews):**
```go
// BAD - time.Sleep instead of wait.Poll
time.Sleep(30*time.Second)
deployment, _ := client.Apps.Deployments(ns).Get(ctx, name, metav1.GetOptions{})
```

## 3. Retry Patterns

Use `retry.RetryOnConflict` for updates:

```go
err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
    config, err := client.Operator.Consoles().Get(
        ctx, api.ConfigResourceName, metav1.GetOptions{},
    )
    if err != nil {
        return err
    }
    config.Spec.SomeField = "newValue"
    _, err = client.Operator.Consoles().Update(
        ctx, config, metav1.UpdateOptions{},
    )
    return err
})
```

## 4. Context Usage

Always use context with timeout:

```go
ctx, cancel := context.WithTimeout(context.TODO(), framework.AsyncOperationTimeout)
defer cancel()
```

**Anti-pattern (flag in reviews):**
```go
// BAD - no timeout, context.Background instead of TODO
ctx := context.Background()
```

## 5. Cleanup and Error Handling

### Cleanup with defer
```go
defer func() {
    err := client.ConsolePluginV1.Delete(
        ctx, pluginName, metav1.DeleteOptions{},
    )
    if err != nil && !apierrors.IsNotFound(err) {
        t.Fatalf("failed to delete plugin: %v", err)
    }
}()
```

**Anti-pattern (flag in reviews):**
```go
// BAD - missing cleanup, or ignoring errors
defer func() {
    client.Delete(ctx, name, metav1.DeleteOptions{})
}()
```

### Error handling
```go
// GOOD - helpful error message
if deployment.Status.ReadyReplicas == 0 {
    t.Fatalf("deployment %s/%s has 0 ready replicas after %v", 
        ns, name, framework.AsyncOperationTimeout)
}

// BAD - vague error
if deployment.Status.ReadyReplicas == 0 {
    t.Fatal("not ready")
}
```

---

# Review Checklist

When reviewing e2e tests, verify:

- [ ] Uses `framework.StandardSetup(t)` for test setup (not `MustNewClientset`)
- [ ] Uses `framework.StandardCleanup(t, client)` with defer for cleanup
- [ ] Creates context with timeout (`context.WithTimeout`)
- [ ] Uses `retry.RetryOnConflict` for config updates
- [ ] Uses `wait.Poll` for async operations (not `time.Sleep`)
  - 5-second intervals for most checks (balances responsiveness with API load)
  - Interval adjusted based on what's being polled
- [ ] Uses framework helpers (`GetConsoleDeployment`, `GetConsoleConfigMap`, etc.)
- [ ] Has helpful error messages with context
- [ ] Handles `IsNotFound` errors in cleanup (`if err != nil && !apierrors.IsNotFound(err)`)
- [ ] Uses table-driven tests for multiple similar cases (when appropriate)

## Anti-Patterns to Flag

### Critical Issues

1. **Missing cleanup**
   - No `defer` for resource cleanup
   - Not handling cleanup errors properly

2. **Using `time.Sleep` instead of `wait.Poll`**
   - Causes flaky tests and wastes time

3. **Missing context timeouts**
   - Tests can hang indefinitely

4. **Ignoring errors with `_`**
   - No error assertions in tests

5. **Tests without assertions**
   - Test runs but doesn't verify anything

### Warnings

1. **Manual client setup instead of framework.StandardSetup**
   - Misses pristine state setup and cleanup

2. **Vague error messages**
   - `t.Fatal("failed")` instead of contextual errors

3. **Not using framework helpers**
   - Manual `client.Apps.Deployments().Get()` instead of `framework.GetConsoleDeployment()`

4. **Missing table-driven structure**
   - Testing multiple similar cases without using table-driven pattern

### Suggestions

1. **Consider grouping imports**
   - Standard lib, 3rd party, kube, openshift, internal

2. **Use framework constants**
   - `framework.AsyncOperationTimeout` instead of hardcoded durations

## Review Output Format

For each issue found, report:
- **File:Line**: Location of the issue
- **Issue**: What pattern is wrong or missing
- **Fix**: How to correct it  
- **Priority**: Critical / Warning / Suggestion

### Example Review Comments

**Critical**: `test/e2e/feature_test.go:45` - Missing cleanup for created ConsolePlugin resource. Add defer with Delete() and IsNotFound check.

**Warning**: `test/e2e/feature_test.go:67` - Using `time.Sleep(30*time.Second)` instead of `wait.Poll`. Replace with `wait.Poll(5*time.Second, framework.AsyncOperationTimeout, ...)` for eventual consistency.

**Suggestion**: `test/e2e/feature_test.go:23` - Consider using `framework.StandardSetup(t)` instead of manual client setup for automatic pristine state management.

---

# Framework API Reference

## Setup and Cleanup

- `framework.StandardSetup(t)` - Returns `(*ClientSet, *operatorv1.Console)`. Sets up client and pristine operator config.
- `framework.StandardCleanup(t, client)` - Restores operator to pristine state after test.

## Resource Helpers

Prefer these helpers over manual client access:

- `framework.GetConsoleDeployment(client)` - Returns `(*appv1.Deployment, error)`
- `framework.GetDownloadsDeployment(client)` - Returns `(*appv1.Deployment, error)`
- `framework.GetConsoleConfigMap(client)` - Returns `(*corev1.ConfigMap, error)`
- `framework.GetCustomLogoConfigMap(client, name)` - Returns `(*corev1.ConfigMap, error)`
- `framework.GetConsoleService(client)` - Returns `(*corev1.Service, error)`
- `framework.GetConsoleRoute(client)` - Returns `(*routev1.Route, error)`
- `framework.GetConsoleCLIDownloads(client, name)` - Returns `(*consolev1.ConsoleCLIDownload, error)`
- `framework.GetConsolePodDisruptionBudget(client, pdbName)` - Returns `(*policyv1.PodDisruptionBudget, error)`

## ClientSet Interfaces

When helper functions don't exist, use `framework.ClientSet` typed interfaces:

**Core Kubernetes:**
- `client.Core` - CoreV1 resources (ConfigMaps, Services, Secrets)
- `client.Apps` - AppsV1 resources (Deployments, DaemonSets, StatefulSets)
- `client.PodDisruptionBudget` - PodDisruptionBudgets

**OpenShift Routing:**
- `client.Routes` - OpenShift Routes

**Console Operator:**
- `client.Operator` - Console operator configs (operatorv1.Console)

**Console Config Resources:**
- `client.Console` - Cluster console configs (configv1.Console)
- `client.ConsolePluginV1` - Console plugins
- `client.ConsoleCliDownloads` - Console CLI downloads
- `client.ConsoleLink` - Console links
- `client.ConsoleNotification` - Console notifications
- `client.ConsoleExternalLogLink` - External log links
- `client.ConsoleYAMLSample` - YAML samples

**Cluster Config:**
- `client.ClusterOperator` - ClusterOperator status
- `client.Infrastructure` - Cluster infrastructure config
- `client.Proxy` - Cluster proxy config
- `client.Ingress` - Cluster ingress config
- `client.FeatureGate` - Feature gates
