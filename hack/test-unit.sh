#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Running unit tests"


PACKAGES_TO_TEST=(
    "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
    "github.com/openshift/console-operator/pkg/crypto"
    "github.com/openshift/console-operator/pkg/operator"
    "github.com/openshift/console-operator/pkg/stub"
)

for PACKAGE in "${PACKAGES_TO_TEST[@]}"
do
    os::log::info "Testing ${PACKAGE}"
    go test $PACKAGE
done
