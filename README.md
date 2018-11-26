# Console Operator

An operator for OpenShift Console.

The console-operator installs and maintains the web console on a cluster.

## Run on a 4.0.0 cluster

The console operator is installed by default and will automatically maintain a console.  

## Development Setup

* Install Go -- https://golang.org/doc/install


<!-- 
TODO: create the gopaths/path/to/here setup
-->

## Development on a < 4.0.0 cluster 

If using `oc cluster up` on a `< 4.0.0` cluster you will need the `--public-hostname` flag 
when you cluster up. The `--server-loglevel` flag is helpful for debugging. OAuth issues 
will not be visible unless set to at least `3`.

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

If you don't know where your `kubeconfig` file is:

```bash 
# just a high number
oc whoami --loglevel=100
# likely output will be $HOME/.kube/config 
```

Build the console operator binary:

```bash 
make update-codegen
make
make verify 
# add the binary to your path 
# arc will be "linux" or "darwin", etc
export PATH="_output/local/bin/<arc>/amd64:${PATH}"
```

Now, run the console operator locally:

```bash
IMAGE=docker.io/openshift/origin-console:latest \ 
    console operator \
    --kubeconfig $HOME/.kube/config \
    --config examples/config.yaml \
    --create-default-console \ 
    --v 4
```

Check for the existence of expected resources:

```bash 
oc get console 
# etc
```

Explanation:

- The `IMAGE` env var is needed to declare what console image to deploy.  The `manifests/05-operator.yaml` shows this var as well
- The `--operator-flags` flag is used to pass flags to the operator binary
    - `--create-default-console true` tells the operator binary to create a console CR if one does not exist on startup.

The `IMAGE` env var exists so that when the console-operator is packaged up for a release, we can replace the value
with a final image.  See CVO documentation for details. 

