all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
    targets/openshift/deps-gomod.mk \
    targets/openshift/images.mk \
    targets/openshift/bindata.mk \
    targets/openshift/operator/profile-manifests.mk \
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

# This will include additional actions on the update and verify targets to ensure that profile patches are applied
# to manifest files
# $0 - macro name
# $1 - target name
# $2 - profile patches directory
# $3 - manifests directory
$(call add-profile-manifests,manifests,./profile-patches,./manifests)

GO_UNIT_TEST_PACKAGES :=./pkg/... ./cmd/...

# Run tests
test: test-unit test-e2e

test-unit: install-go-junit-report
	./test-unit.sh PKG=$(GO_UNIT_TEST_PACKAGES)
.PHONY: test-unit

test-e2e: install-go-junit-report
	./test-e2e.sh
.PHONY: test-e2e

# Automatically install go-junit-report if not found
GO_JUNIT_REPORT := $(shell command -v go-junit-report 2> /dev/null)
install-go-junit-report:
ifndef GO_JUNIT_REPORT
	@echo "Installing go-junit-report..."
	go install -mod= github.com/jstemmer/go-junit-report@latest
else
	@echo "go-junit-report already installed."
	go-junit-report --version
endif

# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR)
.PHONY: clean
