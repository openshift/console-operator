#!/usr/bin/env bash
set -e

# after running an install, run this script to
# configure the cluster for development purposes. Essentially,
# disable CVO, eliminate some noise,set replicas to 1, etc.

YELLOW="\e[93m"
RESET="\e[97m"

echo -e "${YELLOW}switch to project openshift-console-operator${RESET}"
oc project openshift-console-operator

echo -e "${YELLOW}scaling down CVO...${RESET}"
# oc scale deployment cluster-version-operator --replicas 1 --namespace openshift-cluster-version
oc scale deployment cluster-version-operator --replicas 0 --namespace openshift-cluster-version
oc get deployment cluster-version-operator --namespace openshift-cluster-version

echo -e "${YELLOW}scaling down default console-operator...${RESET}"
oc scale deployment console-operator --replicas 0 --namespace openshift-console-operator
oc get deployment console-operator --namespace openshift-console-operator

echo -e "${YELLOW}applying current rbac roles based on what exists on disk...${RESET}"
# NOTE: this stomps on RBAC that comes from whatever was created by the
# installer.  usually this is what you want if you are doing new things...
for FILE in `find ./manifests -name '*rbac*'`
do
  echo "creating ${FILE}"
  oc apply -f $FILE
done

echo -e "${YELLOW}deploying alternative operator...${RESET}"
oc apply -f examples/07-operator-alt-image.yaml
echo -e "${YELLOW}deleting lock file...${RESET}"
oc delete configmap console-operator-lock

echo -e "${YELLOW}cycling deployments in console namespace...${RESET}"
oc delete deployment console --namespace openshift-console

echo -e "${YELLOW}prometheus running at...${RESET}"
oc get route prometheus-k8s -n openshift-monitoring -o jsonpath="{.spec.host}"
echo -e ""

echo -e "${YELLOW}console running at...${RESET}"
oc get console.config.openshift.io cluster -o jsonpath="{.status.consoleURL}"
echo -e ""

echo -e "${YELLOW}given the above success:${RESET}"
echo -e "${YELLOW}CVO is no longer managing the cluster${RESET}"
echo -e "${YELLOW}console deployment deleted${RESET}"
echo -e "${YELLOW}Now, rebuild your operator image, push to your image repository, and redeploy by deleting pods${RESET}"
echo -e ""

oc get deployment -n openshift-console && oc get route -n openshift-console
