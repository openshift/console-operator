#
# This is the integrated OpenShift Service Serving Cert Signer.  It signs serving certificates for use inside the platform.
#
# The standard name for this image is openshift/origin-service-ca
#
FROM openshift/origin-release:golang-1.10
COPY . /go/src/github.com/openshift/service-ca-operator
RUN cd /go/src/github.com/openshift/service-ca-operator && go build ./cmd/service-ca

FROM centos:7
COPY --from=0 /go/src/github.com/openshift/service-ca-operator/service-ca /usr/bin/service-ca
