#!/usr/bin/env bash
set -e

YELLOW="\e[93m"
RESET="\e[97m"

echo -e "${YELLOW}creating a default branding configmap in openshift-config-managed...${RESET}"
oc apply -f examples/configmap.branding.online.yaml

echo -e "${YELLOW}deploying custom console top level config...${RESET}"
oc apply -f examples/cr.console.config.yaml

echo -e "${YELLOW}deploying custom console operator config...${RESET}"
oc apply -f examples/cr.console.operator.yaml

echo -e "${YELLOW}creating custom logo file...${RESET}"
oc create configmap fake-logo-red --from-file examples/fake-logo-red.png -n openshift-config

echo -e "${YELLOW}creating console extensions examples...${RESET}"
oc create -f examples/cr.console.extensions.yaml

echo -e "${YELLOW}applying htpasswd config...${RESET}"
oc create secret generic htpass-secret \
    --from-file examples/htpasswd.example.txt \
    -n openshift-config
# create the custom resource
oc apply -f examples_dev/cr.oauth.config.with.identity.providers.yaml