#!/usr/bin/env bash
# Referenced via @scripts/prepare.sh — sourced by the runner's bash harness.
# A shebang line is inert (spec §3.2); input arrives via globals.

mkdir -p "$WORK/staging"
printf 'artifact\n' > "$WORK/staging/artifact.txt"

# Multiline value: exercises lossless harvesting (spec §4.3)
export LOG_LINES=$'first line\nsecond line\nthird line'
echo "prepared $WORK/staging"
