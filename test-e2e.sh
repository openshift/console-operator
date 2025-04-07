#!/usr/bin/env bash

set -e

ARTIFACT_DIR=${ARTIFACT_DIR:=/tmp/artifacts}

# https://ci-operator-configresolver-ui-ci.apps.ci.l2s4.p1.openshiftapps.com/help#env
OPENSHIFT_CI=${OPENSHIFT_CI:=false}

echo "Running tests..."
if [ "$OPENSHIFT_CI" = true ]; then
    KUBERNETES_CONFIG=${KUBECONFIG} go test -timeout 30m -v ./test/e2e/ 2>&1 | tee "$ARTIFACT_DIR/test.out"
    RESULT="${PIPESTATUS[0]}"
    go-junit-report < "$ARTIFACT_DIR/test.out" > "$ARTIFACT_DIR/junit.xml"
    if [ "$RESULT" -ne 0 ]; then
        exit 255
    fi
else
	echo 'KUBERNETES_CONFIG=${KUBECONFIG} go test -timeout 30m -v ./test/e2e/'
    KUBERNETES_CONFIG=${KUBECONFIG} go test -timeout 30m -v ./test/e2e/
fi

echo "Success"