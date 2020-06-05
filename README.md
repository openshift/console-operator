Console Operator
============================

An operator for OpenShift Console. The console-operator installs and maintains the web console on a cluster.


Workflow
----------------------------

To quickly get started, check the [contrib](contrib/) and [examples](examples/) directories.  For a more detailed 
overview, continue reading. 


Development Environment
----------------------------

* Install Go 1.13 -- https://golang.org/dl/
* GVM is recommended but not required -- https://github.com/moovweb/gvm

### Cloning the Repo

Since the transition to `go.mod` in roughly 4.3 `$GOPATH` is no longer a problem.  However, backport PRs to 4.2 can be difficult.  For this reason, a certain folder structure is recommended to trick the environment.

```bash 
# rather than ~/go directory for everything, provide separate individual go roots within this dir:
mkdir $HOME/gopaths
``` 

It is fine to have `~/gopaths` next to `~/go` if you have some legacy projects.

Now, create a `dir` under `~gopaths` to hold the project:

```bash 
mkdir $HOME/gopaths/consoleoperator 
```

Certain child directories are expected in order to install dependencies and build the project appropriately.

Add a `src` and `bin` dir:

```bash 
mkdir $HOME/gopaths/consoleoperator/src
mkdir $HOME/gopaths/consoleoperator/bin 
```

Then the familiar path for go source code `src/<git>/<org>/<project>`:

```bash
# specifically for this repo
mkdir -p $HOME/gopaths/consoleoperator/src/github.com/openshift
cd $HOME/gopaths/consoleoperator/src/github.com/openshift
```

Now fork then clone into this directory:

```bash 
git clone git@github.com:openshift/console-operator.git 
# or your fork 
git clone git@github.com:<your-fork>/console-operator.git
```

### Gopath

Note that we created `$HOME/gopaths`.  This implies that each project will have
its own gopath, so you will need to set that while working:

```bash 
export GOPATH=$HOME/gopaths/consoleoperator
```

If you have multiple go projects and don't want to fuss with maintaining this when
you `cd` to different projects, give [this script](https://www.jtolio.com/2017/01/magic-gopath/)
a try. It will add a command called `calc_gopath` to your `prompt_command` and set your gopath appropriately depending on the current working directory.


Building the Binary
----------------------------

Running the `make` command will build the binary:

```bash 
make 
```

The binary output will be:

```bash 
./_output/local/bin/<os>/<arch>/console
```

You may want to add this to your path or symlink it:

```bash 
# if your ~/bin is in your path:
ln -s ./_output/local/bin/<os>/<arch>/console ~/bin/console 
```

However, it is no longer recommended to run the operator locally.  Instead, you 
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

It is suggested to run `integration` and `e2e` tests with CI.  This is automatic when opening a PR.

Development Against a 4.x Cluster 
----------------------------

To develop features for the `console-operator`, you will need to run your code against a dev cluster.
The `console-operator` expects to be running in a container.  It is difficult to fake a local environment, and the debugging experience is not like debugging a real container.  Instead, do the following to set yourself up to build your binary & deploy a new container quickly and frequently.

Visit [https://try.openshift.com/](https://try.openshift.com/), download the installer and create 
a cluster.  [Instructions](https://cloud.openshift.com/clusters/install) (including pull secret) 
are maintained here.

```bash
# create a directory for your install config
mkdir ~/cluster
# generate configs using the wizard
openshift-install create install-config
# then run the installer to get a cluster 
# the --log-level flag is recommended for detailed feedback
openshift-install create cluster --dir ~/cluster --log-level debug
```

If successful, you should have gotten instructions to set `KUBECONFIG`, login to the console, etc.

### Shutdown CVO

Since we need to customize the default components of the cluster, we need to shutdown CVO. We don't want the 
default `console-operator` to run if we are going to test our own. Therefore, do the following:

```bash
# Instruct CVO to stop managing the console operator
# CVO's job is to ensure all of the operators are functioning correctly
# if we want to make changes to the operator, we need to tell CVO to stop caring.
oc scale deployment cluster-version-operator --replicas 0 --namespace openshift-cluster-version
# Then, scale down the default console-operator 
oc scale --replicas 0 deployment console-operator --namespace openshift-console-operator
```

Note that you can also simply delete the CVO namespace if you want to turn it off completely (for all operators).  
Scaling down to 0 replicas is safer as you can scale it back up if something goes wrong.

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

But the `make` step is included in the `Dockerfile`, so this does not need to be done manually.
You can instead simply build the container image and push it to your own registry:

```bash 
# the pattern is:
docker build -f Dockerfile.rhel7 -t <registry>/<your-username>/console-operator:<version> .
# following: docker.io/openshift/origin-console-operator:latest
# for development, you are going to push to an alternate registry.
# specifically it can look something like this:
docker build -f Dockerfile.rhel7 -t quay.io/benjaminapetersen/console-operator:latest .
```

You can optionally build a specific version.

Then, push your image:

```bash 
docker push <registry>/<your-username>/console-operator:<version>
# Be sure your repository is public else the image will not be able to be pulled later
docker push quay.io/benjaminapetersen/console-operator:latest
```

Then, you will want to deploy your new container.  This means duplicating the `manifests/07-operator.yaml`
and updating the line `image: docker.io/openshift/origin-console-operator:latest` to instead use the
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
image: quay.io/benjaminapetersen/console-operator:latest
```
And ensure that the `imagePullPolicy` is still `Always`.  This will ensure a fast development feedback loop. 

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
docker build -t quay.io/benjaminapetersen/console-operator:latest .
# push container
docker push quay.io/benjaminapetersen/console-operator:latest
# delete pod, trigger a new pull & deploy
oc delete pod console-operator --namespace openshift-console-operator
```
Docker containers are layered, so there should not be a significant time delay between your pushes.

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

## Tips & General Debugging

If you don't know where your `kubeconfig` is due to running against multiple clusters this can be handy:

```bash 
# just a high number
oc whoami --loglevel=100
# likely output will be $HOME/.kube/config 
``` 
If you need to know information about your cluster:

```bash
# this will list all images, associated GitHub repo, and the commit # currently running.
# very useful to see if the image is running current code...or not.
oc adm release info --commits
# get just the list of images & sha256 digest
oc adm release info
# coming soon...
oc adm release extract 
```

Trouble pushing or pulling images?

- Be sure you are logged in via the CLI to [hub.docker.com](https://hub.docker.com/) if this is your registry.
- Be sure you are logged in via the CLI to [quay.io](https://quay.io/) if this is your registry.
- Verify your local `~/.docker/config.json` is well-formed.
- Verify your local `~/.docker/config.json` does not contain `credSstore:osxkeychain` if on OSX.
- Verify your local `~/.docker/config.json` has no duplicate entries.  For example, 
and entry for `quay.io` and `https://quay.io` will cause problems. 

Trouble building images?  Ensure your local registry isn't full

```bash
docker image prune -af --filter "until=48h"
```

Trouble connecting to the cluster?

- `export KUBECONFIG=/path/to/cluster/auth/kubeconfig` must be set
- `oc login https:api.<username>.devcluster.openshift.com:6443 -u kubeadmin -p $(tail path/to/cluster/auth/kubeadmin-password)
