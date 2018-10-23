# Console Operator

An operator for OpenShift Console built using the operator-sdk.

The console-operator installs and maintains the web console on a cluster.

## Setup

* Install Go -- https://golang.org/doc/install
* Install the Operator SDK -- https://github.com/operator-framework/operator-sdk

## Build and Run

When you make changes be sure to run the command to generate code:

```bash
$ operator-sdk generate k8s
```

Then use the sdk to build your image. I've tended to build with a
simple name, then re-tag to push to hub.docker, quay.io or gitlab registry:

```bash
$ operator-sdk build openshift/console-operator:v0.0.1
```

If it's going to the official registry on quay:

```bash
$ docker tag openshift/console-operator:v0.0.1 quay.io/openshift/console-operator:latest
```

If you are going to your own for testing:
(just be sure you update your yaml files to reference your image)

```bash
$ docker tag openshift/console-operator:v0.0.1 \
   quay.io/benjaminapetersen/console-operator:latest
```

Then push the image to the registry of your choice:

```bash
$ docker push quay.io/benjaminapetersen/console-operator:latest
```

Create the manifests:

```bash
$ oc create -f manfiests/
```

Finally, you can create an instance of your custom resource (CR).
Once an instance exists, your operator will take over managing it.

```bash
$ oc create -f examples/cr.yaml
```

Now check to ensure that your operator creates all of the resource that you
expect for your custom resource to function.  For the console, this should be
at least a deployment, service, route, configmap, and a secret.

```bash
$ oc get all
```

You can now declaratively make changes to your custom resource on the fly by
updating `examples/cr.yaml`. For example, update the number of replicas generated
by changing spec.count.

```bash
$ oc apply -f examples/cr.yaml
```

Do the same verification check to ensure your change was applied and everything
is happy:

```bash
$ oc get console/<name-of-console-pod> -o yaml
```

## Run Locally

It's much easier to dev & run the binary locally (rather than build it, put it
in a container, push the container, then deploy the container...repeat.)

If you are running with `oc cluster up`, make sure you provide a
`--public-hostname` flag in order for the console to properly find the master
public URL. Something like `oc cluster up --public-hostname=<your.ip.address>`
should suffice.

Follow local dev instructions from `operator-sdk` including a few extras:

```bash
$ oc cluster up --public-hostname=<your.ip.address>
```

Create the manifests:

```bash
$ oc create -f manifests/
```
Build the binary, etc. This will appear in `tmp/_output/bin`.

```bash
$ operator-sdk build openshift/console-operator:v0.0.X
```

Run locally for dev:

```bash
$ operator-sdk up local
```

Finally, create a custom resource for the operator to watch. Be sure you are
within the `namespace` that your operator is watching.

```bash
$ oc create -f examples/cr.yaml
```

The operator will look at this manifest and use it to generate all of the
deployments, etc that are needed for the resource to function.

## OpenShift Dependencies

Run the following to add OpenShift dependencies, which are required by the operator:

```bash
dep ensure --add github.com/openshift/api
dep ensure --add github.com/openshift/client-go
```

## Deploy to an Existing Cluster

To deploy the console to a cluster, simply create the manifests in the
`manifests` directory and the custom resource in the `deploy` directory:

```bash
$ oc create -f manifests/
$ oc create -f examples/cr.yaml
```
