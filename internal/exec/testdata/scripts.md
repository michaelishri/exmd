---
type: runbook
globals: {SHVAL, PYVAL: "pydefault"}
---

## one

```bash:exec
@helper.sh
```

## two

```bash:exec
echo "[$SHVAL] [$PYVAL]"
```

## three

```python3:exec
@helper.py
```

## four

```bash:exec
echo "[$PYVAL]"
```
