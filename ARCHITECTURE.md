# Architecture

This document outlines the system architecture for the OpenShift Console Operator.

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

## Controllers

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

## Key Resources

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

## Feature Gates

Feature gates are accessed via `featuregates.FeatureGateAccess` (in `starter.go`):

| Feature Gate | Purpose |
|-------------|---------|
| `ExternalOIDC` | External OIDC authentication support |
| `ConsolePluginContentSecurityPolicy` | Content Security Policy for plugins |

## Resource Syncing

Use `resourcesynccontroller` for syncing ConfigMaps and Secrets between namespaces:
- `oauth-serving-cert` from `openshift-config-managed` to `openshift-console`
- `default-ingress-cert` from `openshift-config-managed` to `openshift-console`

## Quickstarts

Console quickstarts are contributed to the `quickstarts/` directory. See [quickstarts/README.md](./quickstarts/README.md) for guidelines. Quickstarts require cluster profile annotations:

```yaml
annotations:
  include.release.openshift.io/ibm-cloud-managed: "true"
  include.release.openshift.io/self-managed-high-availability: "true"
  include.release.openshift.io/single-node-developer: "true"
```

