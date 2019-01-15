#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Running e2e tests"
KUBERNETES_CONFIG=${KUBECONFIG} GOCACHE=off go test -timeout 30m -v ./test/e2e/