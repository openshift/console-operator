FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/console-operator
COPY . .
RUN ADDITIONAL_GOTAGS="ocp" make build WHAT="cmd/console"; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/console-operator/_output/local/bin/linux/$(go env GOARCH)/console /tmp/build/console

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
RUN useradd console-operator
USER console-operator
COPY --from=builder /tmp/build/console /usr/bin/console

# these manifests are necessary for the installer
COPY manifests /manifests/

# extensions manifests generated from openshift/api types
COPY vendor/github.com/openshift/api/console/v1/*.yaml /manifests/

LABEL io.k8s.display-name="OpenShift console-operator" \
      io.k8s.description="This is a component of OpenShift Container Platform and manages the lifecycle of the web console." \
      io.openshift.tags="openshift" \
      maintainer="Benjamin A. Petersen <bpetersen@redhat.com>"

LABEL io.openshift.release.operator true

