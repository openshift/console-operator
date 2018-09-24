#!/usr/bin/env bash

# initial set of commands a subset of
# https://github.com/openshift/origin/blob/master/Makefile
build:
	hack/build.sh
.PHONY: build

test: test-unit test-integration test-e2e
.PHONY: test

test-unit:
	hack/test-unit.sh
.PHONY: test-unit

test-integration:
	hack/test-integration.sh
.PHONY: test-integration

test-e2e:
	hack/test-e2e.sh
.PHONY: test-e2e

verify:
	hack/verify-gofmt.sh
	# hack/verify-golint.sh this is noisy
	hack/verify-govet.sh
.PHONY: verify

clean:
#	rm -rf $(OUT_DIR)
.PHONY: clean
