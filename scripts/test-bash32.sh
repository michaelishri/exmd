#!/usr/bin/env bash
# Run the execution engine's bash tests against bash 3.2, the version macOS
# ships as /bin/bash. Needs Docker. The tests are built into one static
# binary and run inside the official bash:3.2 image (which has no Go and no
# Python, so only bash tests are selected).
set -euo pipefail

cd "$(dirname "$0")/.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

CGO_ENABLED=0 go test -c -o "$work/exec.test" ./internal/exec
cp -R internal/exec/testdata "$work/testdata"

tests='^(TestHarnessBashVersion|TestBashHarnessUnderBlockTraps|TestReplacedProcessWarns|TestTruncatedDumpIgnored|TestBashExportFlow|TestMasking|TestIsolation|TestFailureStopsAndStillHarvests|TestStdinIsEmpty|TestCancelInterruptsBlock|TestCancelledBlockExitingZeroStopsRun|TestRunBlockIsolation|TestEventsSerialized)$'

docker run --rm -v "$work:/work" -w /work -e EXMD_TEST_BASH=/usr/local/bin/bash \
  bash:3.2 /work/exec.test -test.v -test.run "$tests" "$@"
