#!/bin/bash

set -e

echo "🚀 Console Operator Custom Deployment Script"
echo "============================================="

# Check if we're connected to a cluster
if ! oc whoami >/dev/null 2>&1; then
    echo "❌ Error: Not connected to OpenShift cluster. Please run 'oc login' first."
    exit 1
fi

echo "✅ Connected to OpenShift cluster as: $(oc whoami)"

# Check if the user has built and pushed their custom operator image
echo "🔍 Custom Image Verification"
echo "============================"

# Extract image names from the YAML file
OPERATOR_IMAGE=$(grep -E '^\s*image:\s*quay\.io' examples/07-operator-alt-image.yaml | head -1 | sed 's/.*image:\s*//')
CONSOLE_IMAGE=$(grep -E '^\s*value:\s*quay\.io' examples/07-operator-alt-image.yaml | head -1 | sed 's/.*value:\s*//')

echo "📋 Detected images from examples/07-operator-alt-image.yaml:"
echo "   Operator Image: $OPERATOR_IMAGE"
echo "   Console Image: $CONSOLE_IMAGE"
echo ""

echo "Before proceeding, please confirm:"
echo "1. You have built your custom operator image"
echo "2. You have pushed it to your registry (any registry, not just Docker Hub)"
echo "3. The image path you provided is accessible"
echo "4. You have updated the image path in examples/07-operator-alt-image.yaml"
echo "5. You have updated the console image path in examples/07-operator-alt-image.yaml if you needed?"
echo ""
read -p "Have you done all the above? (y/N): " CONFIRM_BUILD


if [[ ! "$CONFIRM_BUILD" =~ ^[Yy]$ ]]; then
    echo "❌ Please build and push your custom operator image first:"
    echo "   docker build -t $CUSTOM_IMAGE_PATH ."
    echo "   docker push $CUSTOM_IMAGE_PATH"
    echo " and update the image path in examples/07-operator-alt-image.yaml"
    echo "   Then run this script again."
    exit 1
fi



# Get cluster ID dynamically
echo "📋 Getting cluster ID..."
CLUSTER_ID=$(oc get clusterversion version -o jsonpath='{.spec.clusterID}')
if [ -z "$CLUSTER_ID" ]; then
    echo "❌ Error: Could not retrieve cluster ID"
    exit 1
fi
echo "✅ Cluster ID: $CLUSTER_ID"

# Create temporary CVO unmanage configuration
echo "📝 Creating CVO unmanage configuration..."
TEMP_CVO_CONFIG=$(mktemp)
cat > "$TEMP_CVO_CONFIG" << EOF
apiVersion: config.openshift.io/v1
kind: ClusterVersion
metadata:
  name: version
spec:
  clusterID: $CLUSTER_ID
  overrides:
    - kind: Deployment
      name: console-operator
      namespace: openshift-console-operator
      unmanaged: true
      group: apps
    - kind: ClusterRole
      name: console-operator
      namespace: ""
      unmanaged: true
      group: rbac.authorization.k8s.io
EOF

# Step 1: Disable CVO management of console operator
echo ""
echo "📋 Step 1: Disabling CVO management of console operator..."
oc apply -f "$TEMP_CVO_CONFIG"
echo "✅ CVO management disabled"

# Clean up temporary file
rm "$TEMP_CVO_CONFIG"

# Step 2: Scale down the default console operator
echo ""
echo "📋 Step 2: Scaling down default console operator..."
oc scale --replicas 0 deployment console-operator --namespace openshift-console-operator
echo "✅ Default console operator scaled down"

# Step 3: Deploy the custom operator
echo ""
echo "📋 Step 3: Deploying custom console operator..."
oc apply -f examples/07-operator-alt-image.yaml
echo "✅ Custom console operator deployed"

# Step 4: Wait for the operator to be ready
echo ""
echo "📋 Step 4: Waiting for operator to be ready..."
echo "⏳ This may take a few minutes..."

# Wait for deployment to be available
oc rollout status deployment/console-operator -n openshift-console-operator --timeout=300s

# Wait for pod to be ready
echo "⏳ Waiting for pod to be ready..."
oc wait --for=condition=ready pod -l name=console-operator -n openshift-console-operator --timeout=300s

echo "✅ Operator is ready!"

# Step 5: Show status
echo ""
echo "📋 Step 5: Checking operator status..."
echo ""
echo "🔍 Pod status:"
oc get pods -n openshift-console-operator -l name=console-operator

echo ""
echo "🔍 Operator logs (last 10 lines):"
oc logs -n openshift-console-operator -l name=console-operator --tail=10

echo ""
echo "🔍 ClusterOperator status:"
oc describe clusteroperator console

echo ""
echo "🎉 Deployment complete!"
echo ""
echo "📝 Next steps:"
echo "   - Monitor logs: oc logs -f -n openshift-console-operator -l name=console-operator"
echo "   - Check operator status: oc describe clusteroperator console"
echo "   - To update your operator:"
echo "     1. Make code changes"
echo "     2. Rebuild: make"
echo "     3. Rebuild amd64 binary: GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags '...' -o console-amd64 github.com/openshift/console-operator/cmd/console"
echo "     4. Rebuild image: docker build --platform linux/amd64 -f Dockerfile.local -t quay.io/rh-ee-leoli/console-operator:latest ."
echo "     5. Push image: docker push quay.io/rh-ee-leoli/console-operator:latest"
echo "     6. Restart pod: oc delete pod -n openshift-console-operator -l name=console-operator"
echo ""
echo "🔄 To revert to default operator:"
echo "   - oc apply -f examples/cvo-manage-operator.yaml"
echo "   - oc scale --replicas 1 deployment console-operator --namespace openshift-console-operator" 