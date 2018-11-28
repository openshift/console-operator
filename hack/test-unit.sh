#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Running unit tests"


PACKAGES_TO_TEST=(
    "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
    "github.com/openshift/console-operator/pkg/cmd/operator"
    "github.com/openshift/console-operator/pkg/cmd/version"
    "github.com/openshift/console-operator/pkg/console/operator"
    "github.com/openshift/console-operator/pkg/console/starter"
    "github.com/openshift/console-operator/pkg/console/resource/configmap"
    "github.com/openshift/console-operator/pkg/console/resource/deployment"
    "github.com/openshift/console-operator/pkg/console/resource/oauthclient"
    "github.com/openshift/console-operator/pkg/console/resource/route"
    "github.com/openshift/console-operator/pkg/console/resource/secret"
    "github.com/openshift/console-operator/pkg/console/resource/service"
    "github.com/openshift/console-operator/pkg/console/resource/util"
    "github.com/openshift/console-operator/pkg/console/version"
    "github.com/openshift/console-operator/pkg/controller"
    "github.com/openshift/console-operator/pkg/crypto"
    # "github.com/openshift/console-operator/pkg/generated"
)

for PACKAGE in "${PACKAGES_TO_TEST[@]}"
do
    os::log::info "Testing ${PACKAGE}"
    go test $PACKAGE
done
