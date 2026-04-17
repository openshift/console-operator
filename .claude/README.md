# Claude Code Configuration for Console Operator

This directory contains Claude Code configuration and custom skills for the OpenShift console-operator.

## Directory Structure

```
.claude/
├── README.md           # This file
└── skills/             # Custom operator-specific skills
    ├── controller-review.md      # Controller pattern review
    ├── sync-handler-review.md    # Sync handler review
    ├── manifest-review.md        # RBAC/manifest review
    ├── e2e-test-review.md        # E2E test review
    ├── e2e-test-create.md        # E2E test scaffolding
    └── go-quality-review.md      # Code quality review
```

## Skills Overview

### Operator Development Skills

**controller-review** - Review controller implementations
- Checks library-go factory pattern usage
- Validates ManagementState handling (Managed/Unmanaged/Removed)
- Verifies status condition handling
- Ensures proper import organization
- Validates error handling and resource application

**sync-handler-review** - Review sync/reconciliation handlers
- Validates incremental sync pattern
- Checks dependency ordering
- Verifies status updates
- Ensures early returns on errors
- Validates resource creation vs update logic
- Checks feature gate handling

**manifest-review** - Review Kubernetes manifests
- Validates cluster profile annotations
- Checks RBAC least privilege
- Verifies namespace consistency
- Ensures proper YAML formatting
- Validates service account references

### Testing Skills

**e2e-test-review** - Review end-to-end tests
- Validates test structure and cleanup
- Checks for proper wait patterns (wait.Poll, not time.Sleep)
- Verifies retry patterns for config updates
- Ensures context usage with timeouts
- Validates framework usage
- Checks assertion quality

**e2e-test-create** - Scaffold new e2e tests
- Provides complete test templates
- Includes proper framework setup
- Shows cleanup patterns
- Demonstrates wait and retry patterns
- Includes table-driven test examples

### Code Quality Skills

**go-quality-review** - General Go code quality
- Detects deprecated APIs (ioutil.ReadFile, etc.)
- Validates error handling and wrapping
- Checks context propagation
- Identifies code smells (god functions, magic values, deep nesting)
- Reviews performance issues
- Checks concurrency safety

## Using Skills

### In Claude Code

```bash
# Review a controller
/controller-review

# Review sync handler
/sync-handler-review

# Review e2e tests
/e2e-test-review

# Create new e2e test
/e2e-test-create

# Review manifests
/manifest-review

# General code quality review
/go-quality-review
```

### In CodeRabbit PR Reviews

CodeRabbit automatically references these skills based on code patterns (configured in `.coderabbit.yaml`):
- Controller factory patterns (`factory.New()`, `ToController()`) → controller-review
- Sequential sync logic with early returns → sync-handler-review
- E2E framework usage (`framework.MustNewClientset`, `wait.Poll`) → e2e-test-review
- YAML with RBAC kinds or missing annotations → manifest-review
- All Go code → go-quality-review (deprecated APIs, code smells)

## Project Context

These skills reference and enforce patterns documented in:
- `ARCHITECTURE.md` - System architecture and components
- `CONVENTIONS.md` - Go coding standards and patterns
- `TESTING.md` - Testing patterns and commands
- `AGENTS.md` - AI context hub

## Extending Skills

To add a new skill:

1. Create a new `.md` file in `.claude/skills/`
2. Use this frontmatter:
   ```yaml
   ---
   name: skill-name
   description: What this skill does
   tags: [relevant, tags]
   ---
   ```
3. Document what to check, patterns to follow, and output format
4. Update `.coderabbit.yaml` to reference the skill in path_instructions
5. Update this README with the new skill

## Integration with CodeRabbit

The `.coderabbit.yaml` configuration:
- References all skills in `knowledge_base.code_guidelines.filePatterns`
- Maps file paths to relevant skills in `reviews.path_instructions`
- Provides operator-specific review guidelines
- Links to openshift/console repository for cross-repo context

See `.github/CODERABBIT_SETUP.md` for complete integration documentation.

## Common Review Patterns

Based on recent PRs, watch for:
- OwnerReference management (ensure only one controller=true)
- Service account lifecycle and RBAC changes
- API version bumps and deprecated field usage
- Feature flag addition/removal
- Manifest cluster profile annotations
- Cleanup logic for Removed state
- Status condition updates

## Questions?

See project documentation:
- Main docs: `ARCHITECTURE.md`, `CONVENTIONS.md`, `TESTING.md`
- Integration: `.github/CODERABBIT_SETUP.md`
- Skills: Individual `.md` files in this directory
