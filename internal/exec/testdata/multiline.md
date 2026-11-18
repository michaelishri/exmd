---
type: runbook
globals: {MULTI}
---

## one

```python3:exec
import os
os.environ["MULTI"] = "p1\np2"
```

## two

```bash:exec
printf '[%s]\n' "$MULTI"
```
