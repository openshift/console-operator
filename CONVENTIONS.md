# Conventions

This document outlines coding standards and patterns for the OpenShift Console Operator.

See the main OpenShift Go style guidelines for base rules. This document provides project-specific conventions.

## Go Style

- Use `gofmt` for formatting
- Follow standard Go naming conventions
- Group imports: standard lib, 3rd party, kube/openshift, internal (marked with comments)
- Use meaningful error messages with context

## Import Organization

Imports are grouped with comments:

```go
import (
    // standard lib
    "context"

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

## Controller Pattern

Controllers use the OpenShift library-go factory pattern:

```go
return factory.New().
    WithFilteredEventsInformers(
        configNameFilter,
        informers...,
    ).
    WithSync(c.Sync).
    ToController("ControllerName", recorder)
```

## Sync Loop Pattern

The main sync loop (`sync_v400`) works incrementally:
- Start from zero, work through requirements
- If anything is missing, create/update it and return
- Next loop continues from where previous left off
- Ensures simple, non-coordinated logic

## Status Conditions

Set conditions using `status.Handle*` functions with type prefixes:

```go
// For errors that may be transient (SyncError) vs permanent
statusHandler.AddConditions(status.HandleProgressingOrDegraded("ConfigMapSync", reason, err))

// For single conditions
statusHandler.AddCondition(status.HandleDegraded("ConsoleConfig", "FailedUpdate", err))
statusHandler.AddCondition(status.HandleProgressing("SyncLoopRefresh", "InProgress", err))
statusHandler.AddCondition(status.HandleAvailable("Deployment", reason, err))
```

Condition types (appended to prefix):
- `*Degraded` - Errors occurred (true when error present)
- `*Progressing` - Changes are being applied (true when error present)
- `*Available` - Resource is functioning (true when NO error)
- `*Upgradeable` - Safe to upgrade (true when NO error)

## Error Handling

- Use typed errors with meaningful context
- Wrap errors to preserve stack context
- Use `status.Handle*` functions to report conditions

## Branch Naming

- Feature work: `CONSOLE-####` (Jira story number)
- Bug fixes: `OCPBUGS-####` (Jira bug number)

## Commit Strategy

- Separate vendor folder changes into their own commit to isolate core logic changes
- Keep commits focused and atomic
- Write clear commit messages that explain the "why"

