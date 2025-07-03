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
   
      ```

2. **Update image paths in the deployment configuration**:
   ```bash
   # Edit the deployment file to use your custom images
   # Update both the operator image and console image paths
   vim examples/07-operator-alt-image.yaml
   ```
   
   **Important**: Before running the deployment script, you must update the following in `examples/07-operator-alt-image.yaml`:
   
   - **Operator Image**: Change `quay.io/<your username>/console-operator:latest` to your custom operator image path
   - **Console Image**: Change `quay.io/<your username>/console:latest` to your custom console image path (if you have one)
   
   Example configuration:
   ```yaml
   # In examples/07-operator-alt-image.yaml
   containers:
   - name: console-operator
     image: quay.io/your-username/console-operator:latest  # ← Update this
     # ... other config ...
     env:
     - name: CONSOLE_IMAGE
       value: quay.io/your-username/console:latest  # ← Update this if needed
   ```

3. **Deploy using the custom operator script**:
   ```bash
   # Make the script executable
   chmod +x deploy-custom-operator.sh
   
   # Run the deployment script
   ./deploy-custom-operator.sh
   ```
   
   The script will:
   - Verify your custom images are accessible
   - Disable CVO management of the console operator
   - Scale down the default operator
   - Deploy your custom operator
   - Wait for the deployment to be ready
   - Show deployment status and logs

**⚠️ Important Notes**:
- Ensure your custom images are built and pushed to a registry before running the script
- The script will prompt for confirmation that you've updated the image paths
- Make sure you have cluster admin permissions to modify operator deployments
- The deployment may take several minutes to complete

**Troubleshooting**:
- If the deployment fails, check the operator logs: `oc logs -n openshift-console-operator -l name=console-operator`
- To revert to the default operator: `oc apply -f examples/cvo-manage-operator.yaml`

   

