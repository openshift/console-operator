
FROM openshift/origin-release:golang-1.10 as builder
WORKDIR /go/src/github.com/openshift/console-operator
COPY . .
RUN make build

FROM centos:7
RUN useradd console-operator
USER console-operator
COPY --from=builder /go/src/github.com/openshift/console-operator/tmp/_output/bin/console-operator /usr/bin

# these manifests are necessary for the installer
COPY deploy/00-crd.yaml \
    deploy/01-namespace.yaml \
    deploy/02-rbac.yaml \
    deploy/03-operator.yaml \
    /manifests/

LABEL io.k8s.display-name="OpenShift console-operator" \
      io.k8s.description="This is a component of OpenShift Container Platform and manages the lifecycle of the web console." \
      io.openshift.tags="openshift" \
      maintainer="Benjamin A. Petersen <bpeterse@redhat.com>"

# to enable install integration, can add to the set of labels above when ready
#LABEL io.openshift.release.operator true

# entrypoint specified in 03-operator.yaml as `console-operator`
CMD ["/usr/bin/console-operator"]