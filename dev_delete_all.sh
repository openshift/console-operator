#!/usr/bin/env bash

# this just deletes everything under /manifests,
# but tries to space them out a bit to avoid errors.
# in the end, it creates a custom resource to kick
# the operator into action

FILE=examples/cr.yaml
echo "creating ${FILE}"
oc delete -f $FILE

for FILE in `find ./manifests -name '05-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done

for FILE in `find ./manifests -name '04-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done

for FILE in `find ./manifests -name '03-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done

for FILE in `find ./manifests -name '02-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done

for FILE in `find ./manifests -name '01-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done

for FILE in `find ./manifests -name '00-*'`
do
  echo "deleting ${FILE}"
  oc delete -f $FILE
done



