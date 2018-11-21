#!/usr/bin/env bash

# simply create one of each resource, with a sleep in between
# watch the logs and verify that the sync loop sees the events
# then delete each of the resources.
# these are generic resources, not specific.

declare -a FILES_ARRAY=("secret.yaml"
 "configmap.yaml"
 "service.yaml"
 "route.yaml"
 "deployment.yaml"
 "oauth.yaml"
)

# create them all
for FILE in "${FILES_ARRAY[@]}"
do
    echo "creating ${FILE}"
    oc create -f "./examples/sync-loop/${FILE}"
    sleep 3
done

# then, delete them all.


for FILE in "${FILES_ARRAY[@]}"
do
    echo "deleting ${FILE}"
    oc delete -f "./examples/sync-loop/${FILE}"
    sleep 3
done
