# WIP
#
# - how does this relate to the file in /tmp/build/ ?
# - how should it relate to Make commands?
#   - example(s):
#     - https://github.com/openshift/cluster-image-registry-operator/blob/master/Makefile
#   - possible commands:
#     - make build  # build go app into binary
#     - make build-image # build container image with included binary
#     - make build-all # build binary + docker image?
#     - make build-devel # different? or dangerous, to use alpine base?
#
# FROM openshift/origin-release:golang-1.10 as builder
# WORKDIR /go/src/github.com/openshift/console-operator
# COPY . .
# RUN make build
#
#
FROM openshift/origin-release:golang-1.10 as builder
COPY . /go/src/github.com/openshift/console-operator/
RUN cd /go/src/github.com/openshift/console-operator && \
    go build ./cmd/console-operator

FROM centos:7
# TODO: label for enabling install to pick this up:
# LABEL io.openshift.release.operator true

COPY --from=0 /go/src/github.com/openshift/console-operator /usr/bin/
COPY deploy/00-crd.yaml \
    deploy/01-namespace.yaml \
    deploy/02-rbac.yaml \
    deploy/03-operator.yaml \
    /manifests/
# atm no assets
# COPY tmp/build/assets /opt/openshift/

RUN useradd console-operator
USER console-operator

ENTRYPOINT []
CMD ["/usr/bin/console-operator"]