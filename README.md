# Console Operator 

An operator for OpenShift Console built
using the operator-sdk.

## Building

```bash  
# you may need to fiddle around with $GOPATH
echo $GOPATH // /Users/bpeterse/go
GOPATH=$(pwd)
# NOTE: I need to reference gvm pkgset here
```

```bash 
# be sure to generate code: 
operator-sdk generate k8s
# for this project: 
operator-sdk build quay.io/openshift/console-operator:v0.0.1
# re-tag if you need to push to another registry
# for testing purposes. Just be sure that your 
# deployment YAML files reference this.
docker tag quay.io/openshift/console-operator:v0.0.1 \ 
   quay.io/benjaminapetersen/console-operator:latest 
# then push 
docker push quay.io/benjaminapetersen/console-operator:latest
# now, make sure the operator.yaml references you 
# image so that you can deploy it and see that it 
# creates the other objects as expected 
# deploy the RBAC 
# deploy the OPERATOR 
# (this assumes kubectl points to minikube, minishift, etc)
kubectl create -f deploy/rbac.yaml
kubectl create -f deploy/operator.yaml
# check your deployments, and verify your operator exists 
kubectl get deployment 
# check any customizations that you desire to make in the 
# cr.yaml file 
vim deploy/cr.yaml 
# then create an instance of your custom resource, so that 
# your operator can manage it 
kubectl create -f deploy/cr.yaml 
# get your deployments 
kubectl get deployments 
# check pods for custom resource status 
kubectl get pods 
# check your specific pod
kubectl get console/<name-of-console-pod> -o yaml 
```

