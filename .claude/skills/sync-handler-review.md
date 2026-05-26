---
name: sync-handler-review
description: Review operator sync handler logic for incremental reconciliation patterns
tags: [review, sync, reconciliation, operator]
---

# Sync Handler Review Skill

Review sync handler implementations (especially `sync_v400.go` and controller Sync methods).

## What to Check

### 1. Incremental Sync Pattern
The operator uses an incremental sync pattern where each loop picks up where the previous left off:

**Good Pattern:**
```go
func (c *OperatorController) Sync(ctx context.Context, ...) error {
    // 1. Check prerequisite resources
    requiredConfig, err := c.getRequiredConfig()
    if err != nil {
        return err // Will retry
    }

    // 2. Sync first dependency
    if err := c.syncConfigMaps(ctx, requiredConfig); err != nil {
        return err // Stop here, retry next loop
    }

    // 3. Sync second dependency (only if step 2 succeeded)
    if err := c.syncSecrets(ctx); err != nil {
        return err
    }

    // 4. Sync final resource
    return c.syncDeployment(ctx, requiredConfig)
}
```

**Bad Pattern:**
```go
func (c *OperatorController) Sync(ctx context.Context, ...) error {
    // DON'T collect all errors and continue
    var errs []error
    errs = append(errs, c.syncConfigMaps(ctx))
    errs = append(errs, c.syncSecrets(ctx))
    errs = append(errs, c.syncDeployment(ctx))
    return errors.Join(errs...)
}
```

### 2. Dependency Ordering
Resources should be synced in dependency order:
1. ConfigMaps and Secrets (prerequisites)
2. Service Accounts
3. RBAC (Roles, RoleBindings)
4. Services
5. Deployments (depend on above)
6. Routes (depend on Services)

### 3. Status Updates
Status should reflect actual reconciliation state:
```go
statusHandler := status.NewStatusHandler(c.operatorClient)

// Add conditions as operations complete
if err := c.syncResource(ctx); err != nil {
    statusHandler.AddConditions(
        status.HandleProgressingOrDegraded("ResourceSync", "FailedApply", err)
    )
    return statusHandler.FlushAndReturn(err)
}

// Mark available when fully synced
statusHandler.AddCondition(
    status.HandleAvailable("DeploymentAvailable", "", nil)
)
return statusHandler.FlushAndReturn(nil)
```

### 4. Early Returns
Return early on errors to maintain incremental behavior:
```go
// GOOD - returns immediately
if err := c.doSomething(); err != nil {
    return err
}

// BAD - continues after error
if err := c.doSomething(); err != nil {
    klog.Errorf("failed: %v", err)
}
```

### 5. Resource Creation vs Update
Use `resourceapply.Apply*()` functions - they handle both create and update:
```go
_, _, err = resourceapply.ApplyConfigMap(
    ctx,
    c.configMapClient,
    recorder,
    requiredConfigMap,
)
```

### 6. Deleted Resource Detection
When resources are removed from config, ensure they're deleted from cluster:
```go
if !shouldExist {
    err := c.client.Delete(ctx, name, metav1.DeleteOptions{})
    if apierrors.IsNotFound(err) {
        return nil // Already gone, success
    }
    return err
}
```

### 7. Feature Gate Handling
Check feature gates before syncing gated resources:
```go
if featureGates.Enabled(features.ExternalOIDC) {
    if err := c.syncOIDCResources(ctx); err != nil {
        return err
    }
}
```

## Anti-Patterns

- Syncing all resources regardless of errors
- Continuing after failure instead of returning
- Not respecting ManagementState
- Missing cleanup logic for Removed state
- Mutating live objects instead of building desired state
- Lack of status condition updates

## Output Format

For each issue:
- **Location**: Function or line range
- **Pattern**: Which pattern is violated
- **Risk**: What could go wrong
- **Fix**: How to correct it

## Example Review Comments

**Critical**: Sync continues after configmap error. This violates incremental sync pattern and could deploy with incomplete config.

**Warning**: No cleanup logic for ManagementState.Removed. Resources will leak when console is removed.

**Suggestion**: Consider extracting this sync logic into a separate controller for better separation of concerns.
