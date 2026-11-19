---
type: runbook
title: exmd feature tour
spec: 1
tags: demo,reference    # unknown frontmatter key → runner warns (spec §2.1)
globals: {AUTHOR: "Michael Ishri", NOTE: "hello from frontmatter", JOB_NAME, DEPLOY_ID, WORK, LOG_LINES}
---

# exmd feature tour

This runbook exercises every v1 feature of the [spec](../../SPEC.md) safely:
it only writes inside a scratch workspace it creates for itself.
Provide `JOB_NAME` when running (launch-time override, e.g. `--set JOB_NAME=nightly`).

## 1. Preflight and point-of-use guards

```bash:exec
: "${JOB_NAME:?provide JOB_NAME via a launch-time override (e.g. --set JOB_NAME=nightly)}"
echo "author:  $AUTHOR"
echo "note:    $NOTE"
```

## 2. Isolation: `cd` does not leak

Blocks run in fresh processes (spec §4.5) — working-directory changes do not
survive a block:

```bash:exec
cd /tmp && echo "inside this block: $(pwd)"
```

```bash:exec
echo "in the next block:   $(pwd)"
```

## 3. Set a global from bash

`export` is the whole API (spec §4.3):

```bash:exec
WORK="$(mktemp -d)"
export WORK
echo "workspace: $WORK"
```

## 4. Set a global from Python (alias adapter)

`python` maps to the `python3` adapter (spec §5.2). `os.environ` writes are
harvested exactly like bash `export`:

```python:exec
import os
os.environ["DEPLOY_ID"] = "dpl-" + os.environ["JOB_NAME"]
print("registered:", os.environ["DEPLOY_ID"])
```

## 5. Script references

A body that is a single `@path` line executes a file relative to *this document's
directory* (spec §3.2). Shebangs are inert; input arrives via globals.

### 5.1 From bash

```bash:exec
@scripts/prepare.sh
```

### 5.2 From Python

```python3:exec
@scripts/register.py
```

## 6. Closed world and value fidelity

### 6.1 Only exported state is harvested

```bash:exec
INVISIBLE="set but not exported"   # invisible to later blocks (spec §4.5)
export UNDECLARED_X="undeclared"   # harvested → warning: ignored (spec §4.4)
false || echo "survived: || true is the failure escape hatch (spec §4.7)"
```

### 6.2 What you should see next

```bash:exec
echo "INVISIBLE is: ${INVISIBLE:-<absent — unexported state did not survive>}"
echo "--- LOG_LINES (a multiline global, harvested losslessly) ---"
echo "$LOG_LINES"
echo "--- end ---"
```

## 7. Cleanup: unsetting a global

The harvest delta includes removals (spec §4.3):

```bash:exec
unset WORK
echo "WORK unset"
```

```bash:exec
echo "WORK is now: ${WORK:-<absent — unset propagated>}"
```

## 8. Display-only fences

Fences without `:exec` never execute (spec §3.3):

```bash
# This block is inert — it renders, it does not run.
echo "just showing code"
```

## 9. Where we ended up

```bash:exec
echo "run by $AUTHOR for job $JOB_NAME"
echo "deploy id: $DEPLOY_ID"
echo "workspace: ${WORK:-<unset>}"
```
