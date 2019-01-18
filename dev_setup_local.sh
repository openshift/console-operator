#!/usr/bin/env bash

# see the following:
#  Run()
#  https://github.com/openshift/library-go/blob/465b6cce2b41e7acd5313f33a9f4ea36cf494d86/pkg/controller/controllercmd/builder.go#L149
#  calls .getNamespace()
#  https://github.com/openshift/library-go/blob/465b6cce2b41e7acd5313f33a9f4ea36cf494d86/pkg/controller/controllercmd/builder.go#L225
#  which attempts to read from /var/run/secrets/kubernetes.io/serviceaccount/namespace
FILE_PATH_NAMESPACE="/var/run/secrets/kubernetes.io/serviceaccount/"
FILE_NAME_NAMESPACE="namespace"
echo "library-go expects a pod"
echo "creating namespace file ${FILE_PATH_NAMESPACE}${FILE_NAME_NAMESPACE}"
sudo mkdir -p ${FILE_PATH_NAMESPACE}
sudo touch "${FILE_PATH_NAMESPACE}${FILE_NAME_NAMESPACE}"
echo "openshift-console" | sudo tee "${FILE_PATH_NAMESPACE}${FILE_NAME_NAMESPACE}"
