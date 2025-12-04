# OpenShift Console Operator - AI Context

This document outlines the core development conventions for the OpenShift Console Operator codebase. For comprehensive project structure, deployment workflows, and additional context, see [../README.md](../README.md).

## Project Overview

The console-operator is an OpenShift operator that installs and maintains the OpenShift web console on a cluster. It manages:

- Console deployment and configuration
- OAuth client configuration for console authentication
- CLI downloads (oc, kubectl) deployment
- Routes, services, and secrets for the console
- Console plugins and quickstarts
- Pod disruption budgets
- Health checks and monitoring

## Repository Structure

```
console-operator/
├── cmd/console/           # Main entry point (main.go)
├── pkg/
│   ├── api/               # API constants (namespaces, resource names, ports)
│   ├── cmd/
│   │   ├── operator/      # Operator command setup
│   │   └── version/       # Version command
│   ├── console/
│   │   ├── clientwrapper/ # Client wrapper utilities
│   │   ├── controllers/   # All controller implementations
│   │   │   ├── clidownloads/          # CLI downloads controller
│   │   │   ├── clioidcclientstatus/   # CLI OIDC client status controller
│   │   │   ├── downloadsdeployment/   # Downloads deployment controller
│   │   │   ├── healthcheck/           # Health check controller
│   │   │   ├── oauthclients/          # OAuth client controller
│   │   │   ├── oauthclientsecret/     # OAuth client secret controller
│   │   │   ├── oidcsetup/             # OIDC setup controller
│   │   │   ├── poddisruptionbudget/   # PDB controller
│   │   │   ├── route/                 # Route controller
│   │   │   ├── service/               # Service controller
│   │   │   ├── storageversionmigration/
│   │   │   ├── upgradenotification/   # Upgrade notification controller
│   │   │   └── util/                  # Shared controller utilities
│   │   ├── errors/        # Custom error types (SyncError, CustomLogoErrors)
│   │   ├── metrics/       # Prometheus metrics
│   │   ├── operator/      # Main operator logic (sync_v400.go)
│   │   ├── starter/       # Operator startup, informer setup, controller wiring
│   │   ├── status/        # Status condition handling
│   │   ├── subresource/   # Resource builders for each managed resource
│   │   │   ├── authentication/  # Authentication config handling
│   │   │   ├── configmap/       # ConfigMap builders (branding, service CA, trusted CA)
│   │   │   ├── consoleserver/   # Console server config builder
│   │   │   ├── crd/             # CRD utilities
│   │   │   ├── deployment/      # Deployment builder
│   │   │   ├── infrastructure/  # Infrastructure config
│   │   │   ├── oauthclient/     # OAuth client builder
│   │   │   ├── route/           # Route builder
│   │   │   ├── secret/          # Secret builders (session secret)
│   │   │   └── util/            # Shared subresource utilities
│   │   ├── telemetry/     # Telemetry integration
│   │   └── version/       # Version information
│   └── crypto/            # Cryptographic utilities (random string generation)
├── bindata/assets/        # Static YAML assets
│   ├── configmaps/        # ConfigMap templates
│   ├── deployments/       # Deployment templates
│   ├── pdb/               # PodDisruptionBudget templates
│   ├── routes/            # Route templates (console, downloads, custom)
│   └── services/          # Service templates
├── manifests/             # CVO manifest files (deployed to cluster)
├── profile-patches/       # Profile-specific patches (e.g., IBM Cloud managed)
├── quickstarts/           # Console quickstart YAML definitions
├── examples/              # Example configurations
│   ├── cvo-unmanage-operator.yaml  # Disable CVO management for dev
│   ├── cvo-manage-operator.yaml    # Re-enable CVO management
│   └── 07-operator-alt-image.yaml  # Custom operator image deployment
├── test/e2e/              # End-to-end tests
└── vendor/                # Vendored dependencies
```

## Key Components

### Controllers

The operator runs multiple controllers (started in `pkg/console/starter/starter.go`):

| Controller | Purpose |
|-----------|---------|
| `ConsoleOperator` | Main operator coordinating deployment, configmaps, secrets |
| `RouteController` | Manages console and downloads routes |
| `ServiceController` | Manages console and downloads services |
| `OAuthClientsController` | Creates/updates OAuth client for console authentication |
| `OAuthClientSecretController` | Syncs OAuth client secret |
| `OIDCSetupController` | Handles external OIDC authentication (feature-gated) |
| `CLIOIDCClientStatusController` | Reports CLI OIDC client status |
| `CLIDownloadsController` | Manages ConsoleCLIDownload resources |
| `DownloadsDeploymentController` | Manages the downloads deployment |
| `HealthCheckController` | Monitors console health |
| `PodDisruptionBudgetController` | Manages PDBs for console and downloads |
| `UpgradeNotificationController` | Displays upgrade notifications |
| `StorageVersionMigrationController` | Handles storage version migrations |

### Namespaces

Defined in `pkg/api/api.go`:

| Namespace | Purpose |
|-----------|---------|
| `openshift-console` | Console deployment and resources |
| `openshift-console-operator` | Operator deployment |
| `openshift-config` | Cluster configuration |
| `openshift-config-managed` | Managed configuration |
| `openshift-ingress-operator` | Ingress controller |

### Key Resources

| Resource | API Group | Purpose |
|----------|-----------|---------|
| `Console` | operator.openshift.io/v1 | Operator configuration CR |
| `Console` | config.openshift.io/v1 | Cluster console configuration |
| `ClusterOperator` | config.openshift.io/v1 | Cluster operator status |
| `OAuthClient` | oauth.openshift.io/v1 | Console OAuth client |
| `ConsoleCLIDownload` | console.openshift.io/v1 | CLI download configurations |
| `ConsolePlugin` | console.openshift.io/v1 | Dynamic console plugins |
| `ConsoleNotification` | console.openshift.io/v1 | Console notifications |
| `ConsoleQuickStart` | console.openshift.io/v1 | Guided tutorials |

## Development Commands

### Building

```bash
make                 # Build the operator binary
make build           # Same as above

# For macOS developers targeting Linux:
OS_DEBUG=true OS_BUILD_PLATFORMS=linux/amd64 make
```

The binary output is: `./_output/local/bin/<os>/<arch>/console`

### Testing

```bash
make test-unit       # Run unit tests
make test-e2e        # Run end-to-end tests (requires cluster)
make check           # Run verify + test-unit
make verify          # Run all verification checks

# Run tests for specific package:
PKG=./pkg/console/controllers/route ./test-unit.sh
```

### Code Quality

```bash
gofmt -w ./pkg ./cmd   # Format code
go vet ./pkg/... ./cmd/...  # Run vet checks
make verify            # Run all verification
```

### Container Build

```bash
# Build container image
docker build -f Dockerfile.ocp -t quay.io/<username>/console-operator:latest .

# Push to registry
docker push quay.io/<username>/console-operator:latest
```

## Code Conventions

### Go Style

- Use `gofmt` for formatting
- Follow standard Go naming conventions
- Group imports: standard lib, 3rd party, kube/openshift, internal (marked with comments)
- Use meaningful error messages with context

### Import Organization

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

### Controller Pattern

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

### Sync Loop Pattern

The main sync loop (`sync_v400`) works incrementally:
- Start from zero, work through requirements
- If anything is missing, create/update it and return
- Next loop continues from where previous left off
- Ensures simple, non-coordinated logic

### Status Conditions

Set conditions using `status.Handle*` functions with prefixes:

```go
statusHandler.AddConditions(status.HandleDegraded("RouteSync", reason, err))
statusHandler.AddConditions(status.HandleProgressing("Deployment", reason, err))
statusHandler.AddConditions(status.HandleAvailable("Console", reason, nil))
```

Condition types:
- `*Degraded` - Errors occurred
- `*Progressing` - Changes are being applied
- `*Available` - Resource is functioning
- `*Upgradeable` - Safe to upgrade

### Feature Gates

Feature gates are accessed via `featuregates.FeatureGateAccess` (in `starter.go`):

| Feature Gate | Purpose |
|-------------|---------|
| `ExternalOIDC` | External OIDC authentication support |
| `ConsolePluginContentSecurityPolicy` | Content Security Policy for plugins |

### Resource Syncing

Use `resourcesynccontroller` for syncing ConfigMaps and Secrets between namespaces:
- `oauth-serving-cert` from `openshift-config-managed` to `openshift-console`
- `default-ingress-cert` from `openshift-config-managed` to `openshift-console`

## Debugging

```bash
# Inspect cluster operator status
oc describe clusteroperator console

# Get operator events
oc get events -n openshift-console-operator

# View operator logs
oc logs -f deployment/console-operator -n openshift-console-operator

# Describe deployments
oc describe deployment console -n openshift-console

# Check console pods
oc get pods -n openshift-console

# Check operator config
oc get console.operator.openshift.io cluster -o yaml

# Check cluster console config
oc get console.config.openshift.io cluster -o yaml
```

## Development Workflow

### Against a Development Cluster

1. Disable CVO management:
```bash
oc apply -f examples/cvo-unmanage-operator.yaml
```

2. Scale down default operator:
```bash
oc scale --replicas 0 deployment console-operator -n openshift-console-operator
```

3. Build and push your image:
```bash
docker build -f Dockerfile.ocp -t quay.io/<username>/console-operator:latest .
docker push quay.io/<username>/console-operator:latest
```

4. Deploy with custom image using modified `examples/07-operator-alt-image.yaml`

5. Delete pod to pull new image:
```bash
oc delete pod -l name=console-operator -n openshift-console-operator
```

## Branch Naming

- Feature work: `CONSOLE-####` (Jira ticket number)
- Bug fixes: `OCPBUGS-####` (Jira bug number)

## Related Repositories

- [openshift/console](https://github.com/openshift/console) - The web console frontend and backend
- [openshift/api](https://github.com/openshift/api) - OpenShift API definitions
- [openshift/library-go](https://github.com/openshift/library-go) - Shared operator libraries
- [openshift/oauth-templates](https://github.com/openshift/oauth-templates) - OAuth login page templates

## Quickstarts

Console quickstarts are contributed to the `quickstarts/` directory. See [quickstarts/README.md](../quickstarts/README.md) for guidelines. Quickstarts require cluster profile annotations:

```yaml
annotations:
  include.release.openshift.io/ibm-cloud-managed: "true"
  include.release.openshift.io/self-managed-high-availability: "true"
  include.release.openshift.io/single-node-developer: "true"
```

## OWNERS

Reviewers and approvers are defined in the `OWNERS` file:
- @spadgett
- @jhadvig
- @TheRealJon

Component: Management Console
