# Console Operator

An operator for OpenShift Console built using the operator-sdk.

The console-operator installs and maintains the web console on a cluster.

## Setup

* Install Go -- https://golang.org/doc/install
* Install the Operator SDK -- https://github.com/operator-framework/operator-sdk

## Build and Run

When you make changes to any of the source be sure to also run this command to generate code:

```bash
# this will not always change source
$ operator-sdk generate k8s
```

The output should be included in any git commit.

## Run on a 4.0.0 cluster

The console operator is installed by default and will automatically maintain a console.  

## Run locally

If using `oc cluster up` on a `< 4.0.0` cluster you will need the `--public-hostname` flag when you cluster up. The
`--server-loglevel` flag is helpful for debugging. OAuth issues will not be visible unless set to at least `3`.

```bash 
# there are a variety of ways to get your machine IP address
# this example works on OSX
oc cluster up --public-hostname=$(ipconfig getifaddr en0) --server-loglevel 3 
```

Then, create the manifests:

```bash
# pre 4.0.0 needs this, but it is not part of the post 4.0.0 manifests payload
oc create -f ./examples/crd-clusteroperator.yaml
# standard 4.0.0 deploy of the operator
oc create -f ./manifests
# to run the operator locally, delete the deployment and follow instructions below
oc delete -f ./manifests/05-operator.yaml 
```

And finally run the operator:

```bash 
IMAGE=docker.io/openshift/origin-console:latest \ 
    operator-sdk up local \ 
    --namespace=openshift-console \
    --operator-flags="--create-default-console=true"
```

Explanation:

- The `IMAGE` env var is needed to declare what console image to deploy.  The `manifests/05-operator.yaml` shows this var as well
- The `--operator-flags` flag is used to pass flags to the operator binary
    - `--create-default-console true` tells the operator binary to create a console CR if one does not exist on startup.

The `IMAGE` env var exists so that when the console-operator is packaged up for a release, we can replace the value
with a final image.  See CVO documentation for details. 
