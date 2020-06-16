#!/usr/bin/env bash
set -e
set -o errexit
set -o nounset
set -o pipefail

# script usage:
# USERNAME=you ./contrib/dev_vuild_and_deploy.sh
# USERNAME=you TAG=your-desired-tag ./contrib/dev_vuild_and_deploy.sh
# USERNAME=you TAG=your-desired-tag TAG_ALSO_LATEST=true ./contrib/dev_vuild_and_deploy.sh
# USERNAME=you CONTAINER_NAME=my-awesome-operator TAG=your-desired-tag TAG_ALSO_LATEST=true ./contrib/dev_vuild_and_deploy.sh
# USERNAME=you REGISTRY=docker.io CONTAINER_NAME=my-awesome-operator TAG=your-desired-tag TAG_ALSO_LATEST=true ./contrib/dev_vuild_and_deploy.sh
# colors https://misc.flogisoft.com/bash/tip_colors_and_formatting
YELLOW="\e[93m"
GRAY="\e[33m"
RESET="\e[97m"

CONTAINER_NAME=${CONTAINER_NAME:-console-operator}
# tag will default to latest, unless a tag is provided
TAG=${TAG:-latest}
GIT_HASH=$(git log -1 --pretty=%h)
USERNAME=${USERNAME:-openshift}  # TODO: perhaps just remove the default username?
REGISTRY=${REGISTRY:-quay.io}
# if a tag is provided, optionally ALSO tag latest
TAG_ALSO_LATEST=${TAG_ALSO_LATEST:-""}

echo -e "${YELLOW}building images for:${RESET}"
echo -e "${GRAY}  ${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}${RESET}"
echo -e "${GRAY}  ${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:${TAG}${RESET}"

# prune old images to keep disk from getting too full
# build the image
# push the image to the repository
# delete the pod
# namespace is explicit in each step
echo -e "${YELLOW}switch to project openshift-console-operator${RESET}"
oc project openshift-console-operator

echo -e "${YELLOW}pruning images older than 48hrs${RESET}"
docker image prune -af --filter "until=48h"

echo "building ${CONTAINER_NAME}:${TAG} from ${PWD}"
docker build -f Dockerfile.rhel7 -t "${CONTAINER_NAME}:${TAG}" "${PWD}"

# tag it several ways.
# using a real name is helpful.
# using the git tag is also helpful.
# using latest as well may be helpful, for quick and easy iteration.
# (all will use the same image)
echo "tagging ${CONTAINER_NAME} as ${USERNAME}/${CONTAINER_NAME}:${TAG}"
docker tag "${CONTAINER_NAME}:${TAG}" "${USERNAME}/${CONTAINER_NAME}:${TAG}"

echo "tagging ${CONTAINER_NAME} as ${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}"
docker tag "${CONTAINER_NAME}:${TAG}" "${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}"

echo "tagging ${CONTAINER_NAME} as ${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:${TAG}"
docker tag "${CONTAINER_NAME}:${TAG}" "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:${TAG}"

echo "tagging ${CONTAINER_NAME} as ${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}"
docker tag "${CONTAINER_NAME}:${TAG}" "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}"

# its handy to specify a tag, but sometimes also push to latest.
# for example, for a historical record that is human readable, push tag :some-tag, but continue to
# also push tag :latest so that you don't have to continually update your deployment YAML with
# new tag names.
if [[ -z "${TAG_ALSO_LATEST}" ]]; then
  echo "also tagging ${CONTAINER_NAME} as ${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:latest"
  docker tag "${CONTAINER_NAME}:${TAG}" "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:latest"
  docker push "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:latest"
fi

echo ''
echo ''

docker push "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:${TAG}"
docker push "${REGISTRY}/${USERNAME}/${CONTAINER_NAME}:git${GIT_HASH}"

echo ''
echo ''

echo -e "${YELLOW}applying operator (:latest) manifest....${RESET}"
oc apply -f examples_dev/07-operator-mine.yaml

echo -e "${YELLOW}deleting operator deployment/pods...${RESET}"
oc get pods --namespace openshift-console-operator
oc delete configmap console-operator-lock
# dont delete the deployment of the operator, it won't come back without CVO.
oc delete pod "$(oc get --no-headers pods -o custom-columns=:metadata.name --namespace openshift-console-operator)" --namespace openshift-console-operator
# oc delete deployment console-operator

echo -e "${YELLOW}deleting console deployment/pods...${RESET}"
oc delete deployment console -n openshift-console

echo "Deploying new pods..."
oc get pods --namespace openshift-console-operator
