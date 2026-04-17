---
name: e2e-test-review
description: Review e2e tests for best practices, proper cleanup, and wait patterns
tags: [review, testing, e2e]
---

# E2E Test Review Skill

Review end-to-end tests in `test/e2e/` directory for correctness and best practices.

## What to Check

### 1. Test Structure
Tests should follow this pattern:
```go
func TestFeatureName(t *testing.T) {
    client := framework.MustNewClientset(t, nil)
    
    // Setup
    defer cleanupFunction(t, client)
    
    // Test logic with subtests
    t.Run("subtest1", func(t *testing.T) {
        // test code
    })
}
```

### 2. Table-Driven Tests
For testing multiple scenarios, use table-driven pattern:
```go
func TestMultipleScenarios(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {name: "scenario1", input: "a", expected: "b"},
        {name: "scenario2", input: "c", expected: "d"},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result := doSomething(tc.input)
            if result != tc.expected {
                t.Errorf("expected %v, got %v", tc.expected, result)
            }
        })
    }
}
```

### 3. Cleanup and Teardown
Always clean up created resources:
```go
// Use defer for cleanup
defer func() {
    err := client.Delete(ctx, name, metav1.DeleteOptions{})
    if err != nil && !apierrors.IsNotFound(err) {
        t.Logf("cleanup failed: %v", err)
    }
}()
```

### 4. Wait Patterns
Use `wait.Poll` for eventual consistency:
```go
err := wait.Poll(1*time.Second, 5*time.Minute, func() (bool, error) {
    deployment, err := client.AppsV1().Deployments(ns).Get(
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

### 5. Retry Patterns
Use `retry.RetryOnConflict` for updates:
```go
err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
    config, err := client.Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return err
    }
    config.Spec.SomeField = "newValue"
    _, err = client.Update(ctx, config, metav1.UpdateOptions{})
    return err
})
```

### 6. Context Usage
Always use context with timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

### 7. Framework Usage
Use test framework helpers:
```go
// Good - uses framework
client := framework.MustNewClientset(t, nil)
operatorConfig := framework.GetOperatorConfig(t, client)

// Bad - manual setup
config, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
client, _ := kubernetes.NewForConfig(config)
```

### 8. Error Assertions
Be specific about expected errors:
```go
// Good - checks specific error
if !apierrors.IsNotFound(err) {
    t.Fatalf("expected NotFound, got %v", err)
}

// Bad - vague
if err != nil {
    t.Fatal("got error")
}
```

### 9. Test Isolation
Tests should not depend on each other:
- Each test creates its own resources
- Cleanup removes all test resources
- No shared mutable state between tests

### 10. Assertion Quality
```go
// Good - clear message with context
if actual != expected {
    t.Errorf("plugin config mismatch: expected %v, got %v", expected, actual)
}

// Bad - no context
if actual != expected {
    t.Error("mismatch")
}
```

## Red Flags

- Tests without cleanup (resource leaks)
- Missing `wait.Poll` when checking async operations
- Direct sleeps instead of wait conditions
- Tests that modify global state
- Tests without timeout contexts
- Assertions without helpful error messages
- Tests that require specific ordering to pass

## E2E-Specific Patterns

### Console Config Updates
```go
err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
    config, err := client.ConfigV1().Consoles().Get(
        ctx, "cluster", metav1.GetOptions{},
    )
    if err != nil {
        return err
    }
    config.Spec.Customization = &configv1.ConsoleCustomization{
        Brand: configv1.BrandOKD,
    }
    _, err = client.ConfigV1().Consoles().Update(
        ctx, config, metav1.UpdateOptions{},
    )
    return err
})
```

### Plugin Testing
```go
// Create plugin
plugin := &consolev1.ConsolePlugin{
    ObjectMeta: metav1.ObjectMeta{Name: "test-plugin"},
    Spec: consolev1.ConsolePluginSpec{
        DisplayName: "Test Plugin",
        Service: consolev1.ConsolePluginService{
            Name:      "plugin-service",
            Namespace: "plugin-ns",
            Port:      8443,
        },
    },
}
defer cleanupPlugin(t, client, plugin.Name)
```

## Output Format

For each issue:
- **Test**: Test function name
- **Issue**: What's wrong
- **Risk**: Why it matters
- **Fix**: How to improve

## Example Review Comments

**Critical**: Test has no cleanup - will leak ConsolePlugin resource. Add defer with cleanup.

**Warning**: Using time.Sleep instead of wait.Poll. This makes test flaky and slower than necessary.

**Suggestion**: Consider using table-driven tests to cover multiple plugin configurations.

**Info**: Good use of retry.RetryOnConflict for config updates.
