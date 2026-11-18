---
type: runbook
title: Flow globals
spec: 1
globals: {ENV: staging, JOB_ID, NOTE: "hello from frontmatter"}
---

## Step one

```bash:exec
echo "$ENV $NOTE ${JOB_ID:-absent}"
```

## Step two

```bash:exec
export JOB_ID="j-42"
```
