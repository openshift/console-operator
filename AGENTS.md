# OpenShift Console Operator - AI Context Hub

This file serves as the central AI documentation hub for the OpenShift Console Operator project. AI assistants (Claude, Cursor, Copilot, CodeRabbit, etc.) use this and the linked documents to understand project context.

## Quick Reference

### This Repository

| Document | Purpose |
|----------|---------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | System architecture, components, repository structure |
| [CONVENTIONS.md](./CONVENTIONS.md) | Go coding standards, patterns, import organization |
| [TESTING.md](./TESTING.md) | Testing patterns, commands, debugging |
| [README.md](./README.md) | Project README with setup instructions |

### Console Repository (openshift/console)

For frontend-related guidelines, see the [openshift/console](https://github.com/openshift/console) repository:

| Document | Purpose |
|----------|---------|
| [STYLEGUIDE.md](https://github.com/openshift/console/blob/master/STYLEGUIDE.md) | Frontend code style guidelines |
| [INTERNATIONALIZATION.md](https://github.com/openshift/console/blob/master/INTERNATIONALIZATION.md) | i18n patterns and translation guidelines |
| [CONTRIBUTING.md](https://github.com/openshift/console/blob/master/CONTRIBUTING.md) | Contribution guidelines for the console project |


## Project Summary

The **console-operator** is an OpenShift operator that installs and maintains the OpenShift web console on a cluster. It manages:

- Console deployment and configuration
- OAuth client configuration for console authentication
- CLI downloads (oc, kubectl) deployment
- Routes, services, and secrets
- Console plugins and quickstarts
- Pod disruption budgets
- Health checks and monitoring

## Essential Commands

```bash
# Build
make                           # Build the operator binary
make build                     # Same as above

# Test
make test-unit                 # Run unit tests
make test-e2e                  # Run e2e tests (requires cluster)
make check                     # Run verify + test-unit

# Code Quality
gofmt -w ./pkg ./cmd           # Format code
go vet ./pkg/... ./cmd/...     # Run vet checks
make verify                    # Run all verification
```

## Key Namespaces

| Namespace | Purpose |
|-----------|---------|
| `openshift-console` | Console deployment and resources |
| `openshift-console-operator` | Operator deployment |
| `openshift-config` | Cluster configuration |
| `openshift-config-managed` | Managed configuration |
| `openshift-ingress-operator` | Ingress controller (default ingress) |

## Branch Naming

- Feature work: `CONSOLE-####` (Jira story number)
- Bug fixes: `OCPBUGS-####` (Jira bug number)

## Related Repositories

- [openshift/console](https://github.com/openshift/console) - Web console frontend and backend (see STYLEGUIDE.md, INTERNATIONALIZATION.md, CONTRIBUTING.md)
- [openshift/api](https://github.com/openshift/api) - OpenShift API definitions
- [openshift/library-go](https://github.com/openshift/library-go) - Shared operator libraries

## OWNERS

Component: Management Console
- @spadgett
- @jhadvig
- @TheRealJon

