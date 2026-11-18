---
type: runbook
title: Valid
globals: {OK_VAR: "fine"}
---

## Mixed problems

```ruby:exec
puts 1
```

```bash:exec name=deploy
echo hi
```

```bash:exec verbose
echo hi
```

```bash:exec
@does/not/exist.sh
```

```bash:exec
```

```bash:exec
@ 
```
