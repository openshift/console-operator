#!/usr/bin/env bash

# as system admin, delete everything that matters
oc login -u system:admin

# this tells CVO not to reconcile the operator deployment.
# this is necessary if you want to run the operator binary locally
# and not fight with the existing deployment.
# otherwise, CVO will continually re-deploy the operator deployment
# every time you delete it and the local operator will have to fight
# with it.  You may still be able to "win" as the configmap console-operator-lock
# ensures via leader election that only one of the operators will
# act at a time, but its best to avoid this hassle.
oc patch -f ./dev/cvo-disable-operator.yaml

# not going to delete/recreate rbac or the crd, these are out of the control of the operator

# wipe out project, all console resources, and recreate only the namespace
oc delete project openshift-console
oc create -f ./manifests/01-namespace.yaml
# 04-config.yaml is not strictly needed, we will use a local below
oc create -f ./manifests/04-sa.yaml

# wipe out the oauth client & recreate it clean. this is just easier than editing it
oc delete oauthclient openshift-console
oc create -f ./manifests/00-oauth.yaml

# login w console-operator service account token (not system:admin)
oc login --token=$(oc sa get-token console-operator -n openshift-console)

# start the operator binary
IMAGE=docker.io/openshift/origin-console:latest \
    console operator \
    --kubeconfig $HOME/openshift/installer/aws/auth/kubeconfig \
    --config examples/config.yaml \
    --create-default-console \
    --v 4

