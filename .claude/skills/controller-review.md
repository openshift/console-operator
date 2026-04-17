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
- Check for `apierrors.IsNotFound()` when appropriate
- Return meaningful error messages

### 6. Resource Application
- Use `resourceapply.Apply*()` functions from library-go
- Pass controller recorder for events
- Handle returned error properly

### 7. OwnerReference Management
- Set owner references using `subresourceutil.OwnerRefFrom(cr)`
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
