---
type: runbook
globals: {GREETING, DECLARED_CHECK: "declared default"}
---

## one

```bash:exec
export GREETING="hello world"
HIDDEN="invisible"
export UNDECLARED="noisy"
echo "hello from block 1"
```

## two

```bash:exec
echo "[$GREETING]"
echo "[${HIDDEN:-absent}]"
```

## three

```bash:exec
unset GREETING
```

## four

```bash:exec
echo "[${GREETING:-absent}]"
echo "[${UNDECLARED:-absent}]"
```
