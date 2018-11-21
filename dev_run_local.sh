#!/usr/bin/env bash

# we need to be system admin to install these
oc login -u system:admin

# this just deploys everything under /manifests,
# but tries to space them out a bit to avoid errors.
# in the end, it creates a custom resource to kick
# the operator into action

# necessary if doing dev locally on a < 4.0.0 cluster
CLUSTER_OPERATOR_CRD_FILE="./examples/crd-clusteroperator.yaml"
echo "creating ${CLUSTER_OPERATOR_CRD_FILE}"
oc create -f "${CLUSTER_OPERATOR_CRD_FILE}"

# examples/cr.yaml is not necessary as the operator will create
# an instance of a "console" by default.
# use it if customization is desired.

for FILE in `find ./manifests -name '00-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '01-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

# use the openshift-console project, for
#  - when we create the CR (namespace: should be defined in the resource anyway)
#  - when we run the operator locally.
oc project 'openshift-console'

for FILE in `find ./manifests -name '02-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '03-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '04-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

# at this point, we should no longer be system:admin
# oc login -u developer -p 12345

# ensure the latest binary has been built
make build

# Don't deploy the operator in `manifests`
# instead, we will instantiate the operator locally
#
#for FILE in `find ./manifests -name '05-*'`
#do
#  echo "creating ${FILE}"
#  oc create -f $FILE
#done

# temporaily add the binary to path so we can call it below
export PATH="$PATH:$HOME/gopaths/consoleoperator/src/github.com/openshift/console-operator/_output/local/bin/darwin/amd64"

IMAGE=docker.io/openshift/origin-console:latest \
    console operator \
    --kubeconfig $HOME/.kube/config \
    --config examples/config.yaml \
    --v 4

echo "TODO: support --create-default-console again!"
# TODO: GET BACK TO THIS:
#IMAGE=docker.io/openshift/origin-console:latest \
#    console operator \
#    --kubeconfig $HOME/.kube/config \
#    --config examples/config.yaml \
#    --create-default-console \
#    --v 4

# NOT creating the CR as the operator should create one automatically.
# echo "Creating the CR to activate the operator"
# oc create -f "./examples/cr.yaml"

