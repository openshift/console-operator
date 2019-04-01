FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
WORKDIR /go/src/github.com/openshift/service-ca-operator
COPY . .
RUN go build -o service-ca-operator ./cmd/service-ca-operator

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/service-ca-operator/service-ca-operator /usr/bin/
COPY manifests /manifests
ENTRYPOINT ["/usr/bin/service-ca-operator"]
LABEL io.k8s.display-name="OpenShift service-ca-operator" \
      io.k8s.description="This is a component of OpenShift and manages serving certificates" \
      com.redhat.component="service-ca-operator" \
      maintainer="OpenShift Auth Team <aos-auth-team@redhat.com>" \
      name="openshift/ose-service-ca-operator" \
      version="v4.0.0" \
      io.openshift.tags="openshift" \
      io.openshift.release.operator=true
