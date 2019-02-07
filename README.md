# Console Operator

An operator for OpenShift Console.

The console-operator installs and maintains the web console on a cluster.

## Run on a 4.0.0 Cluster

The console operator is installed by default and will automatically maintain a console.  

## Development Setup

* Install Go -- https://golang.org/doc/install

## Clone the Repo & Build Locally

To avoid some of the standard quirks of `gopath`, the recommended way to clone and 
work with this repository is to do the following:

### Cloning the Repo

```bash 
# rather than ~/go for everything, provide separate gopaths
mkdir $HOME/gopaths
``` 

It is fine to have `~/gopaths` next to `~/go` if you have some legacy projects.

Now, create a `dir` under `~gopaths` to hold the project:

```bash 
mkdir $HOME/gopaths/consoleoperator 
```

The name of this directory doesn't matter much, but the child directories are 
important in order to install dependencies and build the project appropriately.

An `src` and `bin` dir is expected:

```bash 
mkdir $HOME/gopaths/consoleoperator/src
mkdir $HOME/gopaths/consoleoperator/bin 
```

Then the familiar path for source code `src/github.com/openshift/console-operator`:

```bash
# specifically for this repo
mkdir -p $HOME/gopaths/consoleoperator/src/github.com/openshift
cd $HOME/gopaths/consoleoperator/src/github.com/openshift

```

Now clone (or fork, then clone) into this directory:

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

If you have multiple goprojects and don't want to fuss with maintaining this when
you `cd` to different projects, give [this script](https://www.jtolio.com/2017/01/magic-gopath/)
a try. It will add a command called `calc_gopath` to your `prompt_command` and 
set your gopath appropriately depending on the current working directory.

(At some point `golang` will fix `$GOPATH` and this won't be necessary)

### Development & Building the Binary

Running the `make` command will build the binary:

```bash 
make 
```

The binary output will be:

```bash 
./_output/local/bin/<os>/<arch>/console
```

You will likely want to add this to your path or symlink it:

```bash 
# if your ~/bin is in your path:
ln -s ./_output/local/bin/<os>/<arch>/console ~/bin/console 
```


Test `gofmt` and other verification tools:

```bash 
make verify
```

Let `gofmt` automatically update your source:

```bash 
gofmt -w ./pkg
gofmt -w ./cmd 
```

Run the tests with 

```bash
make test-unit
```

It is suggested to run `integration` and `e2e` tests with CI.  This is automatic when opening a PR.

### Run the Binary Locally 

The operator expects the `IMAGE` env var to be set in order to know which console to deploy. In 
addition, it is recommended to use the `--create-default-console` flag as this ensures the operator 
will deploy a console even if a `cr.yaml` for the openshift console is not found.  The console will 
be deployed with reasonable default values. 

The `./examples/config.yaml` file mirrors `04-config.yaml` and can be used for dev testing.  Currently 
it only controls leader election and provides nothing interesting to configure.

Run:

```bash 
IMAGE=docker.io/openshift/origin-console:latest \
    console operator \
    --kubeconfig $HOME/.kube/config \
    --config examples/config.yaml \
    --create-default-console \
    --v 4
```

NOTE: your `--kubeconfig` may be in another location.

### Deploy an alternative image 

To build a new container image and then deploy it do the following:

```bash 
# build the operator binary
make 
# now build a container image, tagging it to your own remote image registry:
docker build -t <registry>/<username>/<repo> .
# example:
docker build -t quay.io/harrypotter/griffindor:latest
# push to the registry
docker push quay.io/harrypotter/griffindor:latest
```
In order to deploy the image, you will want to edit the `./examples/05-operator-alt-image.yaml` 
file. It mirrors `05-operator.yaml` which is the official deployment manifest.  Updating the 
`container` `image` to `image: quay.io/harrypotter/griffindor:latest` should be sufficient to then:

```bash 
oc create -f ./examples/05-operator-alt-image.yaml
```

## Running Against a 4.0.0 Cluster

The console operator is installed by default and will automatically maintain a console. For development,
if you want to run the console-operator locally against a 4.0 cluster with the appropriate 
capabilities (not as `system:admin` but rather using the correct service account) do the following:

```bash 
# if you want to remove the existing console entirely to start fresh
oc login -u system:admin
oc delete project openshift-console 
oc delete project openshift-console-operator

# then recreate ensure all the necessary resources (including the namespace)
# rolebindings, service account, etc
oc create -f manifests/00*.yaml
oc create -f manifests/01*.yaml
oc create -f manifests/02*.yaml
oc create -f manifests/03*.yaml
oc create -f manifests/04*.yaml
```

Then to correctly run the operator you will want to login using the token from the 
service account:

```bash 
oc login --token=$(oc sa get-token console-operator -n openshift-console-operator)
```

After doing the above steps, you can run the operator locally with the following:

```bash 
# if you make changes, be sure to rebuild the binary with `make`
IMAGE=docker.io/openshift/origin-console:latest \
    console operator \
    --kubeconfig $HOME/.kube/config \
    --config examples/config.yaml \
    --create-default-console \
    --v 4
```

NOTE: your `--kubeconfig` may be in another location.

## Running against a < 4.0.0 Cluster (min 3.11 Recommended)


If using oc cluster up on a < 4.0.0 cluster you will need the `--public-hostname` flag 
when you cluster up. The `--server-loglevel` flag is helpful for debugging. 
OAuth issues will not be visible unless the loglevel is set to at least `3`.

```bash 
# there are a variety of ways to get your machine IP address
# this example works on OSX
OAUTH_VISIBLE_LOGLEVEL=3
# get IP address on Linux
MACHINE_IP_ADDR=hostname -I
# or get IP address on OSX (uglier eh?)
MACHINE_IP_ADDR=ipconfig getifaddr en0
# put it all together
oc cluster up --public-hostname=$MACHINE_IP_ADDR --server-loglevel $OAUTH_VISIBLE_LOGLEVEL
```

Once you have a running cluster, you can deploy everything like this:

```bash 
oc create -f manifests
```
But if you want to run the operator locally instead of the deployed container, remove the deployment:

```bash 
oc delete -f manifests/05-operator.yaml
```

In addition, you may need the `clusteroperator` CRD:

```bash 
# pre 4.0.0 needs this, but it is not part of the post 4.0.0 manifests payload
oc create -f ./examples/crd-clusteroperator.yaml
```

## Tips

If you don't know where your `kubeconfig` is due to running against multiple clusters this can be handy:

```bash 
# just a high number
oc whoami --loglevel=100
# likely output will be $HOME/.kube/config 
``` 









