#!/usr/bin/env bash

echo "Running unit tests"

# temp just pass it along to the unit tests
THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
$THIS_DIR/test-unit.sh