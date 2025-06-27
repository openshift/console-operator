# Console Operator

The OpenShift Console Operator is a Kubernetes operator that manages the OpenShift web console deployment. It handles the installation, configuration, and lifecycle management of the OpenShift Console and its associated components.

## Overview

The Console Operator is responsible for:
- Deploying and managing the OpenShift web console
- Managing console downloads deployment
- Configuring console authentication and authorization
- Handling console customization (logos, branding, etc.)
- Managing console routes and services
- Ensuring console availability and health

## Quick Start

### Prerequisites

- Go 1.23.0 or later
- Docker
- OpenShift CLI (`oc`)
- Access to an OpenShift cluster

### Building the Operator

```bash
# Clone the repository
git clone https://github.com/openshift/console-operator.git
cd console-operator

# Build the operator binary
make


# Build for specific platform (e.g., Linux AMD64)
GOOS=linux GOARCH=amd64 make
```

### Running Tests

```bash
# Run unit tests
make test-unit

# Run end-to-end tests
# It is suggested to run `integration` and `e2e` tests with CI.  This is automatic when opening a PR.
make test-e2e

# Run all tests
make test

# Verify code formatting and linting
make verify
```

## Development

### Local Development Setup

For detailed development instructions, see [DEVELOPMENT.md](DEVELOPMENT.md).

#### Quick Development Workflow

1. **Build and deploy to a development cluster**:
   ```bash
   # Build Docker image
   # Note: Ensure the image architecture matches your target platform
   # For multi-arch builds, use: docker buildx build --platform linux/amd64,linux/arm64
   docker build -f Dockerfile.rhel7 -t quay.io/your-username/console-operator:latest .

   
   # Push to registry
   docker push quay.io/your-username/console-operator:latest
   
   # Deploy to cluster
   ./deploy-custom-operator.sh
   ```

2. **Make changes and update**:
   ```bash
   # Update operator only
   ./update-operator.sh
   
   # Update operator and console UI
   ./update-both.sh
   ```

### Project Structure

```
├── cmd/                    # Command-line applications
│   └── console/           # Main operator binary
├── pkg/                   # Core packages
│   └── console/           # Console operator logic
│       ├── controllers/   # Kubernetes controllers
│       ├── subresource/   # Resource management
│       └── operator/      # Operator lifecycle
├── bindata/               # Embedded assets
│   └── assets/           # Kubernetes manifests
├── manifests/             # Operator manifests
├── test/                  # Test files
│   └── e2e/              # End-to-end tests
├── examples/              # Example configurations
└── quickstarts/           # Console quick starts
```



### Key Components
- **Controllers**: Manage different aspects of the console (deployment, routes, services, etc.)
- **Subresources**: Handle resource creation and updates
- **Bindata**: Embedded Kubernetes manifests for console deployment
- **Manifests**: Operator installation and configuration files
