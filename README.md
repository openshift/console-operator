# Console Operator

An operator for [OpenShift Console](https://github.com/openshift/console). The
console-operator installs and maintains the web console on a cluster.

## Workflow

To quickly get started, check the [contrib](contrib/) and [examples](examples/)
directories. For a more detailed overview, continue reading.

## Dependencies

- Go 1.13 -- https://golang.org/dl/

## Building the Binary

Running the `make` command will build the binary:

```bash
make
```

The binary output will be:

```bash
./_output/local/bin/<os>/<arch>/console
```

However, it is no longer recommended to run the operator locally. Instead, you
should be building a docker image and deploying it into a development cluster.
Continue below for instructions to do this with a reasonable feedback loop.

### Verify Source Code

Test `gofmt` and other verification tools:

```bash
make verify
```

Let `gofmt` automatically update your source:

```bash
gofmt -w ./pkg
gofmt -w ./cmd
```

### Run Unit Tests

```bash
make test-unit
```

It is suggested to run `integration` and `e2e` tests with CI. This is automatic when opening a PR.

## Development Against a 4.x Cluster

To develop features for the `console-operator`, you will need to run your code
against a dev cluster. The `console-operator` expects to be running in a
container. It is difficult to fake a local environment, and the debugging
experience is not like debugging a real container. Instead, do the following
to set yourself up to build your binary & deploy a new container quickly and
frequently.

Visit [https://try.openshift.com/](https://try.openshift.com/), download the installer and create
a cluster. [Instructions](https://cloud.openshift.com/clusters/install) (including pull secret)
are maintained here.

### Shutdown CVO

Since we need to customize the default components of the cluster, we need to
shutdown CVO. We don't want the default `console-operator` to run if we are
going to test our own. Therefore, do the following:

```bash
# instruct CVO to stop managing the console operator
# CVO's job is to ensure all of the operators are functioning correctly
# if we want to make changes to the operator, we need to tell CVO to stop caring
oc scale deployment cluster-version-operator --replicas 0 --namespace openshift-cluster-version
# then, scale down the default console-operator
oc scale --replicas 0 deployment console-operator --namespace openshift-console-operator
```

Now we should be ready to build & deploy the operator with our changes.

### Preparation to Deploy Operator Changes Quickly

Typically to build your binary you will use the `make` command:

```bash
# this will build for your platform:
make
# if you are running OSX, you will need to build for Linux doing something like:
OS_DEBUG=true OS_BUILD_PLATFORMS=linux/amd64 make
# note that you can build for multiple platforms with:
make build-cross
```

But the `make` step is included in the `Dockerfile`, so this does not need to
be done manually. You can instead simply build the container image and push it
to your own registry:

```bash
docker build -f Dockerfile.rhel7 -t <registry>/<your-username>/console-operator:<version> .
```

You can optionally build a specific version.

Then, push your image:

```bash
docker push <registry>/<your-username>/console-operator:<version>
```

Be sure your repository is public else the image will not be able to be pulled
later.

Then, you will want to deploy your new container. This means duplicating the
`manifests/07-operator.yaml` and updating the line
`image: docker.io/openshift/origin-console-operator:latest` to instead use the
image you just pushed.

```bash
# duplicate the operator manifest to /examples or your ~/ home dir
cp manifests/07-operator.yaml ~/07-operator-alt-image.yaml
```

Then, update the image & replicas in your `07-operator-alt-image.yaml` file:

```yaml
# before
replicas: 2
image: docker.io/openshift/origin-console-operator:latest
# after
# image: <registry>/<your-username>/console-operator:<version>
replicas: 1
image: <registry>/<your-username>/console-operator:latest
```

And ensure that the `imagePullPolicy` is still `Always`. This will ensure a
fast development feedback loop.

```yaml
imagePullPolicy: Always
```

### Deploying

At this point, your pattern will be

- Change code
- Build a new docker image
  - This will automatically & implicitly `make build` a new binary
- Push the image to your repository
- Delete the running `console-operator` pod
  - This will cause the Deployment to pull the image again before deploying a new pod

Which looks like the following:

```bash
# build binary + container
docker build -t <registry>/<your-username>/console-operator:latest .
# push container
docker push <registry>/<your-username>/console-operator:latest
# delete pod, trigger a new pull & deploy
oc delete pod console-operator --namespace openshift-console-operator
```

### Manifest changes

If you are making changes to the manifests, you will need to `oc apply` the manifest.

#### Debugging The Operator

```bash
# inspect the clusteroperator object
oc describe clusteroperator console
# get all events in openshift-console-operator namespace
oc get events -n openshift-console-operator
# retrieve deployment info (including related events)
oc describe deployment console-operator -n openshift-console-operator
# retrieve pod info (including related events)
oc describe pod console-operator-<sha> -n openshift-console-operator
# watch the logs of the operator pod (scale down to 1, no need for mulitple during dev)
oc logs -f console-operator-<sha> -n openshift-console-operator
# exec into the pod
 oc exec -it console-operator-<sha> -- /bin/bash
```
