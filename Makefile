all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
    golang.mk \
    targets/openshift/deps-gomod.mk \
    targets/openshift/images.mk \
    targets/openshift/bindata.mk \
)

# Run core verification and all self contained tests.
#
# Example:
#   make check
check: | verify test-unit
.PHONY: check

IMAGE_REGISTRY?=registry.svc.ci.openshift.org

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
# It will generate target "image-$(1)" for building the image and binding it as a prerequisite to target "images".
$(call build-image,ocp-console-operator,$(IMAGE_REGISTRY)/ocp/4.5:console-operator,./Dockerfile.rhel7,.)

GO_TEST_PACKAGES :=./pkg/... ./cmd/...

test: test-unit test-e2e
.PHONY: test

test-e2e:
	KUBERNETES_CONFIG=${KUBECONFIG} go test -timeout 30m -v ./test/e2e/
.PHONY: test-e2e


# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR)
.PHONY: clean
