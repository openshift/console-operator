#!/usr/bin/env bash

# NOTE: Makefile MUST use a "tab", not "spaces as tabs" inside
# commands, else it will error with cryptic:
#     Makefile:<line-#>: *** missing separator.  Stop.
# IMAGE ?= docker.io/openshift/console-operator:latest
# PROG  := console-operator

all: generate build build-image test

generate:
	operator-sdk generate k8s
.PHONY: generate

# operator-sdk script to build operator binary
# operator-sdk script to put binary into a container
build:
    # hack/build.sh 
	./tmp/build/build.sh
.PHONY: build

build-image:
	 IMAGE=docker.io/openshift/console-operator ./tmp/build/docker_build.sh
.PHONY: build-image

build-all:
	./tmp/build/build.sh
	IMAGE=docker.io/openshift/console-operator ./tmp/build/docker_build.sh
.PHONY: build-all

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
	hack/verify-golint.sh -m
	hack/verify-govet.sh
.PHONY: verify

clean:
	hack/clean.sh
.PHONY: clean
