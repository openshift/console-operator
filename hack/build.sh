#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Building console-operator binary to _output/"
# delegate to the operator-sdk generated build scripts
# to build the binary
DIR=$( cd "$( dirname ${BASH_SOURCE[0]} )" && pwd )
source "$DIR/../tmp/build/build.sh"