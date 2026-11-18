---
type: runbook
globals: {FROM_HOST: "from-default"}
---

## one

```bash:exec
echo "$FROM_HOST"
```

## two

```bash:exec
export FROM_HOST="changed-by-block"
```

## three

```bash:exec
echo "$FROM_HOST"
```
