---
type: runbook
globals: {BEFORE}
---

## one

```bash:exec
export BEFORE="survives"
echo "about to fail"
```

## two

```bash:exec
exit 3
```

## three

```bash:exec
echo "never runs"
```
