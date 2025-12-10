# Testing

This document covers testing patterns and debugging for the OpenShift Console Operator.

## Running Tests

```bash
# Unit tests
make test-unit

# Run tests for specific package
PKG=./pkg/console/controllers/route ./test-unit.sh

# End-to-end tests (requires cluster)
make test-e2e

# All verification checks
make verify

# Combined verify + test-unit
make check
```

## Code Quality

```bash
# Format code
gofmt -w ./pkg ./cmd

# Run vet checks
go vet ./pkg/... ./cmd/...

# Run all verification
make verify
```

## Test Patterns

- Use table-driven tests for comprehensive coverage
- Use `httptest` for HTTP handler testing
- Include proper cleanup functions
- Test both success and failure paths

## Debugging

### Inspect Cluster Operator Status

```bash
oc describe clusteroperator console
```

### Get Operator Events

```bash
oc get events -n openshift-console-operator
```

### View Operator Logs

```bash
oc logs -f deployment/console-operator -n openshift-console-operator
```

### Describe Deployments

```bash
oc describe deployment console -n openshift-console
```

### Check Console Pods

```bash
oc get pods -n openshift-console
```

### Check Operator Config

```bash
oc get console.operator.openshift.io cluster -o yaml
```

### Check Cluster Console Config

```bash
oc get console.config.openshift.io cluster -o yaml
```

## Development Workflow

### Against a Development Cluster

1. **Disable CVO management:**
   ```bash
   oc apply -f examples/cvo-unmanage-operator.yaml
   ```

2. **Scale down default operator:**
   ```bash
   oc scale --replicas 0 deployment console-operator -n openshift-console-operator
   ```

3. **Build and push your image:**
   ```bash
   docker build -f Dockerfile.ocp -t quay.io/<username>/console-operator:latest .
   docker push quay.io/<username>/console-operator:latest
   ```

4. **Deploy with custom image** using modified `examples/07-operator-alt-image.yaml`

5. **Delete pod to pull new image:**
   ```bash
   oc delete pod -l name=console-operator -n openshift-console-operator
   ```

## Container Build

```bash
# Build container image
docker build -f Dockerfile.ocp -t quay.io/<username>/console-operator:latest .

# Push to registry
docker push quay.io/<username>/console-operator:latest
```

## Building for macOS Developers

When targeting Linux from macOS:

```bash
OS_DEBUG=true OS_BUILD_PLATFORMS=linux/amd64 make
```

The binary output is: `./_output/local/bin/<os>/<arch>/console`

