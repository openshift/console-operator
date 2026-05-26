---
name: controller-review
description: Review controller code for OpenShift operator patterns and library-go conventions
tags: [review, controller, operator]
---

# Controller Review Skill

Review controller implementation code following OpenShift console-operator patterns.

## What to Check

### 1. Controller Factory Pattern
- Uses `factory.New().WithFilteredEventsInformers()` pattern
- Proper informer filtering with `util.IncludeNamesFilter()`
- ToController() call with descriptive name and recorder

### 2. ManagementState Handling
All controllers should handle three states:
```go
switch operatorConfig.Spec.ManagementState {
case operatorv1.Managed:
    // sync logic
case operatorv1.Unmanaged:
    // skip sync, return nil
case operatorv1.Removed:
    // cleanup/removal logic
default:
    return fmt.Errorf("unknown state: %v", ...)
}
```

### 3. Status Condition Handling
- Use `status.NewStatusHandler(c.operatorClient)`
- Set conditions with appropriate types:
  - `status.HandleProgressingOrDegraded()` for transient errors
  - `status.HandleDegraded()` for degraded conditions
  - `status.HandleProgressing()` for in-progress state
  - `status.HandleAvailable()` for availability
- Always `FlushAndReturn()` at end of sync

### 4. Import Organization
Imports must be grouped with comments:
```go
import (
    // standard lib
    "context"
    "fmt"

    // 3rd party
    "github.com/spf13/cobra"

    // kube
    "k8s.io/client-go/kubernetes"

    // openshift
    "github.com/openshift/api/config/v1"

    // operator (internal)
    "github.com/openshift/console-operator/pkg/api"
)
```

### 5. Error Handling
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Check for `apierrors.IsNotFound()` when appropriate:
  - **Delete operations** - NotFound means already deleted (success):
    ```go
    err := c.client.Delete(ctx, name, metav1.DeleteOptions{})
    if apierrors.IsNotFound(err) {
        return nil // Already deleted, success
    }
    return err
    ```
  - **Optional resources** - Resource might not exist yet:
    ```go
    config, err := c.lister.Get(name)
    if err != nil && !apierrors.IsNotFound(err) {
        return fmt.Errorf("failed to get config: %w", err)
    }
    if apierrors.IsNotFound(err) {
        config = &corev1.ConfigMap{} // Use default
    }
    ```
  - **Get before create** - Check existence before creating:
    ```go
    existing, err := c.lister.Get(name)
    if apierrors.IsNotFound(err) {
        // Resource doesn't exist, create it
        return c.client.Create(ctx, required, metav1.CreateOptions{})
    } else if err != nil {
        return fmt.Errorf("failed to check existing: %w", err)
    }
    ```
- Return meaningful error messages

### 6. Resource Application
- Use `resourceapply.Apply*()` functions from library-go
- Pass controller recorder for events
- Handle returned error properly

### 7. OwnerReference Management
- Set owner references using `util.OwnerRefFrom(cr)` (from `pkg/console/subresource/util`)
- Be careful with multiple owner references (only one controller=true)
- Handle OwnerRef cleanup when replacing existing resources

## Output Format

For each issue found, report:
- **Location**: File:line or function name
- **Issue**: What pattern is missing or incorrect
- **Fix**: How to correct it
- **Severity**: Critical / Warning / Suggestion

## Example Review Comments

**Critical**: Missing ManagementState.Removed handling in Sync() function. Controller will not clean up resources when console is removed.

**Warning**: Status conditions not flushed. Add `statusHandler.FlushAndReturn(err)` at end of Sync().

**Suggestion**: Consider grouping imports with standard comments for better readability.
