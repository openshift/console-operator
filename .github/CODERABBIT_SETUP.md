# CodeRabbit and Claude Code Integration Setup

This document describes the AI tooling integration for the console-operator repository.

## Overview

The console-operator uses two complementary AI tools:
- **Claude Code**: Interactive development assistant with custom operator skills
- **CodeRabbit**: Automated PR review bot that references Claude skills

## Claude Code Skills

Custom skills are defined in `.claude/skills/`:

| Skill | Purpose | Use When |
|-------|---------|----------|
| `controller-review` | Review controller patterns | Reviewing controller implementations |
| `sync-handler-review` | Review sync/reconciliation logic | Reviewing operator sync handlers |
| `manifest-review` | Review RBAC/CVO manifests | Reviewing manifest changes |
| `unit-test-review` | Review unit tests | Reviewing pkg/**/*_test.go files |
| `e2e-test-review` | Review e2e tests | Reviewing or creating test/e2e/**/*_test.go files |
| `go-quality-review` | Review code quality | Checking for deprecated APIs, code smells |

### Using Skills

In Claude Code CLI or IDE extensions:
```bash
# Review existing code
/controller-review
/e2e-test-review

# When generating new e2e tests, reference patterns from the review skill:
# "Reference /e2e-test-review skill for framework patterns"
```

In PR reviews, CodeRabbit will reference these automatically based on code patterns, not just file paths.

## CodeRabbit Configuration

Configuration is in `.coderabbit.yaml`. It:
- References project documentation (ARCHITECTURE.md, CONVENTIONS.md, etc.)
- Applies review guidelines from skills based on **code patterns** (function signatures, types, YAML kinds) not just file paths
- Integrates with openshift/console repository context
- Provides operator-specific review guidelines

> **Note**: CodeRabbit embeds skill content as reviewer instructions during PR review.
> Skills are also usable interactively via Claude Code (e.g., `/controller-review`).

### Pattern-Based Review Guidelines

Review guidelines from skills are applied when specific code patterns are detected:

**controller-review** applies when code contains:
- `type *Controller struct` definitions
- `factory.New().WithFilteredEventsInformers()` pattern
- `Sync(ctx context.Context, controllerContext factory.SyncContext)` methods
- `status.NewStatusHandler` usage

**sync-handler-review** applies when code contains:
- Sequential `resourceapply.Apply*()` calls with early returns
- Feature gate conditional logic
- Incremental reconciliation patterns

**manifest-review** applies when YAML contains:
- `kind: Role` or `kind: ClusterRole`
- Missing cluster profile annotations in `manifests/`
- Wildcard permissions (`verbs: ["*"]`)

**e2e-test-review** applies when tests contain:
- `framework.StandardSetup(t)` or `framework.MustNewClientset(t, nil)`
- `wait.Poll` or `retry.RetryOnConflict`
- Console/operator CR manipulations

**go-quality-review** applies to all Go code to check:
- Deprecated APIs (`ioutil.*`, `Dial` without `DialContext`)
- Error handling patterns (missing `%w` in `fmt.Errorf`)
- Code smells (deep nesting, god functions, magic values)

This approach is more reliable than path-based matching, as it recognizes what the code actually does.

## CI/CD Integration

### Skip E2E Tests for Tooling Changes

Changes to AI tooling configuration should not trigger e2e test runs.

**This repository (console-operator):**
- OWNERS file documents the intent (see comments)
- `.coderabbit.yaml` contains the configuration
- `.claude/skills/` contains Claude skills

**openshift/release repository:**

To skip e2e tests for tooling changes, add to the ci-operator configuration for console-operator:

```yaml
skip_if_only_changed: "^(\\.coderabbit\\.yaml|\\.claude/|.*\\.md|OWNERS)$"
```

Or in Prow configuration:
```yaml
- name: pull-ci-openshift-console-operator-main-e2e
  skip_if_only_changed: "^(\\.coderabbit\\.yaml|\\.claude/|.*\\.md|OWNERS)$"
```

**Action Required:** Update openshift/release configuration to implement this skip pattern.

## Updating Skills

To add or modify Claude skills:

1. Edit or create skill files in `.claude/skills/`
2. Update `.coderabbit.yaml` path_instructions to reference new skills
3. Test the skill locally with Claude Code: `/skill-name`
4. Commit changes (will not trigger e2e tests once openshift/release is updated)

## Syncing with openshift/console

The console repository has similar AI tooling. When adding patterns here, consider:
- Are there equivalent patterns for TypeScript/React code?
- Should frontend-specific skills be added to console repo?
- Do both repos need to reference the same conventions?

Keep tooling configuration aligned between repos where patterns overlap.

## Reference

- [Claude Code Documentation](https://docs.anthropic.com/claude/docs)
- [CodeRabbit Documentation](https://coderabbit.ai/docs)
- [OpenShift CI/CD Docs](https://docs.ci.openshift.org/)
