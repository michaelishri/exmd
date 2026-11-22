---
type: runbook
globals: {GREETING, TOKEN: "default"}
---

## one

```bash:exec
export GREETING="hello from one"
echo "one done"
```

## two

```bash:exec
echo "two sees [$GREETING] [$TOKEN]"
```

## three

```bash:exec
echo "three done"
```
