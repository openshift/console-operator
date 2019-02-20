#
# This is the integrated OpenShift Service CA Operator.  It signs serving certificates for use inside the platform.
#
# The standard name for this image is openshift/origin-service-ca-operator
#
FROM openshift/origin-release:golang-1.10
COPY . /go/src/github.com/openshift/service-ca-operator
RUN cd /go/src/github.com/openshift/service-ca-operator && go build ./cmd/service-ca-operator

FROM centos:7
COPY --from=0 /go/src/github.com/openshift/service-ca-operator/service-ca-operator /usr/bin/service-ca-operator

COPY manifests /manifests
LABEL io.openshift.release.operator=true
