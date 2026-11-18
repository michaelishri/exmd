---
type: runbook
title: Block globals
globals:
  ENV: staging
  JOB_ID:
---

## Step

```bash:exec
echo "$ENV"
```
