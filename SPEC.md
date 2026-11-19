# exmd Specification

**Version:** 1 (draft 1)
**Status:** Initial specification, pre-implementation
**Reference implementation:** `exmd` (Go)

---

## 1. Overview

exmd defines a format for **executable Markdown documents** ("runbooks"): ordinary
Markdown files whose fenced code blocks can be executed in order by a runner, sharing
state through environment variables.

A **runner** is a program that reads such a document and executes its executable blocks
according to this specification. The reference runner is `exmd`, a Go implementation
whose execution surface is a TUI (see README.md).

### 1.1 Goals

1. **Ordinary Markdown.** Runbooks are valid, readable Markdown everywhere — GitHub,
   editors, static sites. Executability must not compromise renderability.
2. **No templating layer.** Block source is byte-for-byte what executes. Values reach
   code only through the process environment. There is no substitution, interpolation,
   or escaping. A source view therefore shows exactly what runs.
3. **One state rule.** State flows forward through a single uniform mechanism: *a
   block's exported environment becomes the following blocks' inherited environment.*
4. **Fail fast.** Everything statically checkable is validated before the first side
   effect.

### 1.2 Non-goals (v1)

- Templating or interpolation of block source
- Parallel or conditional execution
- Sandboxing, secret management, or privilege management
- Full execution-transcript capture
- Non-string value types

### 1.3 Terminology

- **runner** — a program implementing this specification.
- **document** — a UTF-8 Markdown file (LF or CRLF line endings).
- **runbook** — a document whose frontmatter declares `type: runbook`.
- **block** — a fenced code block (CommonMark) whose info string matches §3.1.
- **script reference** — a block whose body is a `@path` one-liner (§3.2).
- **adapter** — the per-language machinery that executes a block and reports its
  environment delta (§5).
- **global** — a variable in the document's global namespace, declared in frontmatter
  `globals`.
- **harvest** — applying a block's environment delta to the globals (§4.3).
- **docdir** — the directory containing the document.

The key words MUST, MUST NOT, SHOULD, and MAY are to be interpreted as described in
RFC 2119.

---

## 2. Document format

### 2.1 Frontmatter

```yaml
---
type: runbook
title: Deploy the API
spec: 1
globals: {ENV: staging, IMAGE_TAG: latest, DEPLOY_ID, JOB_NAME}
---
```

- The document MUST begin with YAML frontmatter: a `---` line, a YAML mapping, and a
  `---` line. Malformed frontmatter is a load error (§6.1).
- `type` MUST be exactly `runbook` (case-sensitive) for the document to be a runbook.
  Documents that do not declare `type: runbook` are ordinary Markdown: they have no
  runner semantics, and the runner MUST refuse to execute them (§6.1).
- `title` (string, optional) — display only.
- `spec` (integer, optional, default `1`) — specification version the document targets.
  A runner MUST reject documents targeting a version it does not implement (§6.1).
- Unknown frontmatter keys MUST be ignored; runners SHOULD emit one warning about them.
  Frontmatter routinely carries site metadata (`layout`, `tags`, …) that must not break
  execution.

### 2.2 Globals

`globals` is a mapping. Block style and flow style are the same YAML type; both are
valid:

```yaml
globals:
  ENV: staging
  JOB_NAME:          # bare key — declared, starts absent
```

```yaml
globals: {ENV: staging, IMAGE_TAG: latest, DEPLOY_ID, JOB_NAME}
```

- Every key is a **declared global**: the complete inventory of names the document may
  use as globals — settable via `--set` and via block exports, and nothing else.
  Names MUST match `[A-Za-z_][A-Za-z0-9_]*`.
- Values MUST be YAML scalars:
  - non-null scalar → the global **starts present**, with the scalar's canonical string
    form (booleans → `true`/`false`; numbers → decimal form; dates → parsed string
    form).
  - `null` (explicit or bare key) → the global **starts absent**. Declaration is *not*
    a promise of launch-time presence; pre-flight MUST NOT fail on absent globals.
  - `KEY: ""` starts present with an empty-string value — distinct from absent.
- A `globals` value that is not a mapping is a load error. In particular, the string
  form `globals: A, B, C="x"` is rejected with a hint pointing at the flow-mapping
  form: YAML parses it as a single opaque string, and supporting it would require a
  second quoting grammar.
- Duplicate keys are a load error.
- Runtime value resolution precedence: **frontmatter default < launch-time override**.
  Launch-time overrides (conventionally spelled `--set KEY=VALUE` on command-line
  runners) may target any declared key (defaulted or bare); overriding an undeclared
  key is a usage error (§6.1). Overrides cannot create new globals.

**Masking.** Declared names are document-controlled for the entire run. The composed
block environment (§4.2) removes host environment variables of declared names: a
defaulted global shadows a same-named host variable; a bare global masks it entirely
(absent until set). All undeclared host environment variables pass through untouched.
Consequently, the only ways a global ever changes are: its frontmatter default,
`--set`, and block exports (§4.3).

**Reading absent globals** is ordinary language behavior: bash yields `""` (or an
error under `set -u`); Python raises `KeyError`. Documents that require a launch-time
value SHOULD guard at the point of use, so the run fails at the block that needs the
value, with the author's message:

```bash
: "${JOB_NAME:?run with --set JOB_NAME=<name>}"
```

### 2.3 Load-time validation

The following are validated during pre-flight (§5.3). All diagnostics are collected
and reported together; any failure is a spec/usage error (§6.1) that prevents
execution entirely, with zero side effects performed.

- frontmatter parses and satisfies §2.1
- global names valid, values scalar (§2.2)
- every executable fence's language has an adapter (§5.2)
- every adapter's interpreter binary is found on `PATH`
- every script reference names an existing, regular, readable file
- no unknown block attributes (§3.1)

---

## 3. Executable blocks

### 3.1 Fence grammar

```
fence-info := lang ":" "exec" [ SP attribute ]*
attribute  := attr-key "=" attr-value       ; reserved — none defined in v1
lang       := ALPHA [ ALPHA / DIGIT / "-" / "_" / "+" ]*
```

- ` ```bash:exec ` and ` ```python3:exec ` are executable blocks.
- Language matching is case-sensitive against the adapter table (§5.2). `python` is a
  defined alias for `python3`.
- No whitespace is allowed around `:`.
- `lang:exec` followed by one or more attributes is a load error in v1. Attributes are
  reserved so future extensions (`name=deploy`, `on_error=continue`) fail loudly on
  older runners instead of being silently ignored.
- Any other fence — ` ```bash `, ` ```exec `, ` ```sh ` — is display-only and has no
  execution semantics (§3.3).
- Standard CommonMark fence parsing applies; blocks inside list items or indented
  contexts still execute, in document order.

### 3.2 Script references

A block whose body is exactly one line beginning (after trimming) with `@` references
a file; the remainder of the line, verbatim, is the path:

````
```bash:exec
@scripts/deploy.sh
```

```python3:exec
@scripts/register_deploy.py
```
````

- Relative paths resolve against **docdir** — never the runner's cwd — so a runbook
  behaves identically no matter where the runner is invoked from. Absolute
  paths are used as-is. There is no `~` expansion, no variable expansion, and no
  quoting grammar: every character after `@` (trimmed) is part of the path, spaces
  included.
- The fence language selects the adapter and is authoritative; file extensions are
  not interpreted.
- A body consisting of one `@`-line plus any further line is embedded code (this keeps
  e.g. a Python block starting with a decorator distinguishable). An empty body is a
  load error.
- Script references execute with semantics identical to embedded code: same
  environment composition, same harvest, same failure handling. For the bash adapter
  this means the script is sourced into the harness shell: a shebang line is inert,
  and `$0`/argv carry no meaning — input arrives via globals.
- Pre-flight verifies existence, regular-file, and readability (§2.3).
- The block's *effective source* is the referenced file's contents; interfaces that
  display block source (source views, validation output) SHOULD show the referenced
  path and line number alongside.

Binary tools do not get fence languages; invoke them from a bash block. Fences execute
code whose state the runner harvests; binaries are just commands.

### 3.3 Display fences

Any fence not matching §3.1 is inert. In particular, ` ```bash ` blocks never execute
and have no state effects. This is what makes runbooks safely embeddable in ordinary
documents.

---

## 4. Execution semantics

### 4.1 Ordering

Blocks execute strictly top-to-bottom, single pass, sequentially. No concurrency in v1.

### 4.2 Environment composition

Before each block, the runner composes the block's process environment:

```
block_env := (host_env − declared_names) ∪ current_globals
```

`host_env` is the runner's own environment, snapshotted at startup. The subtraction
implements masking (§2.2); the union means globals override, and declared-but-absent
names are simply absent.

Blocks run with their working directory set to **docdir** (§3.2), so relative paths
in block code resolve against the document's directory, matching how script
references resolve. This does not weaken isolation (§4.5): `cd` within a block still
does not survive it.

### 4.3 State propagation (harvest)

> Let `E_in` be the composed environment passed to a block. Let `E_out` be the
> environment the block's process would pass to a child process when its execution
> completes. The **delta** is every name added, changed, or removed between `E_in`
> and `E_out`.
>
> On block completion the runner applies the delta to the globals: added or changed
> names become globals; removed names are deleted. Names not declared in `globals`
> are ignored with a warning (§4.4).

These are therefore the same operation:

```bash
export DEPLOY_ID="dpl_42"
```

```python
os.environ["DEPLOY_ID"] = "dpl_42"
```

Notes:

- Only *exported* state is harvested. In bash, `FOO=1` without `export` is invisible.
- The delta includes removals: a block that unsets a declared global removes it for
  subsequent blocks.
- The delta is applied **before** failure evaluation (§4.7), keeping semantics total;
  a future `on_error: continue` needs no additional rule.
- Values may contain any character except NUL. Runners SHOULD transport them
  losslessly (e.g. NUL-delimited dumps).

### 4.4 Closed world

Exporting an undeclared name neither fails nor propagates. The runner emits one
warning per name:

```
warning: [4/7] bash (line 58) exported undeclared variable FOO_BAR (ignored)
```

Rationale: incidental mutations (a tool exporting its own variables, `PATH` churn)
must not pollute downstream blocks, and the frontmatter must remain a complete,
self-documenting contract of what the document mutates.

### 4.5 Isolation guarantees

Each block runs in a **fresh process**. Nothing survives a block except the harvested
delta. Explicitly non-persistent across blocks:

- working directory (`cd`)
- shell functions, aliases, and options (`set -e`, `shopt`)
- unexported shell variables
- Python imports, module-level state, and locals
- environment changes to undeclared names

Consequence: re-running a document is deterministic given the same `--set` inputs and
host environment.

### 4.6 Stdin

Blocks run with stdin connected to an empty stream (`/dev/null`). Runbooks are
non-interactive; a command that reads stdin fails rather than hanging the document.
Runners MUST NOT allocate a TTY.

### 4.7 Failure

The first block to exit non-zero stops the run. The runner reports the block (index,
language, line, kind), its exit status, and exits `1`. Output is streamed live (§4.8),
so the failing output is already visible.

The v1 escape hatch for intended failure is the language's own: `|| true` in bash;
`try/except` with an explicit exit in Python.

Timeouts: none in v1.

Signals: on SIGINT/SIGTERM the runner terminates the current block (SIGTERM, then
SIGKILL after a short grace period) and exits `130`.

### 4.8 Observation

The runner MUST expose each block's execution such that:

- every line of block output is attributable to the block that produced it — output
  MUST NOT be mixed with other blocks' output or with runner narration in a way that
  loses that attribution;
- output is visible as it is produced (live streaming, or a TUI equivalent such as a
  scrolling pane);
- block output is presented unaltered. The interface MAY decorate it (panes, headers,
  colors) as long as the attribution above holds.

Stream-based runners typically add per-block headers (e.g. `▶ [3/7] bash — line 42`,
indices counting executable blocks in document order) and keep narration on stderr so
stdout captures only block output. Those are interface conventions, not requirements
of this specification.

---

## 5. Language adapters

### 5.1 Adapter contract

An adapter for language `L` MUST:

1. Execute the block source (embedded code, or the referenced file for script
   references) in a process whose environment is the composed environment (§4.2) and
   whose stdin is empty (§4.6).
2. Report, on completion, the process's final environment delta (§4.3) — names
   added/changed/removed, values lossless.
3. Pass the process's exit status through unchanged.
4. Guarantee that user code executes as-if directly: injected machinery (prelude,
   harness) MUST NOT change the observable semantics of the source beyond state
   harvesting.
5. Confine its own machinery to an internal namespace (e.g. `__mdexec_`-prefixed
   names, internal environment keys) that cannot collide with user code.
6. Suppress interpreter-managed names — those the interpreter itself maintains in
   the process environment, such as `_`, `PWD`, `OLDPWD`, and `SHLVL` for bash —
   from the reported delta. They are infrastructure, not document state, and are
   neither harvested nor warned about (§4.4).

### 5.2 v1 adapters

| Fence language | Interpreter | Alias   |
|----------------|-------------|---------|
| `bash`         | `bash`      | —       |
| `python3`      | `python3`   | `python`|

`sh`, `zsh`, and `node` are future additions (Appendix A).

### 5.3 Pre-flight

Before block 1 executes, the runner MUST perform **all** checks of §2.3 — reporting
every diagnostic, not failing at the first — with zero side effects performed. Any
failure is a spec/usage error (§6.1). This includes verifying each adapter's
interpreter exists on `PATH`.

---

## 6. Runner interface

This specification mandates no user interface. A runner MAY expose execution through
a TUI, a command line, a library API, or any other surface; conformance (§9) concerns
the semantics of §2–§5, not the interface. The reference runner (`exmd`) executes
through a TUI and offers headless utility verbs that never execute blocks (see
README.md).

### 6.1 Diagnostics and refusal

- Documents that are not runbooks (§2.1) MUST NOT be executed. A runner MUST report
  why, rather than executing or silently doing nothing.
- Load-time problems (§2.3) are diagnostics, distinct from runtime failures (§4.7):
  they MUST be detected in pre-flight, reported with block and line context, and MUST
  prevent execution entirely.

### 6.2 Exit-code convention

Runners that expose process exit codes SHOULD follow:

| Code | Meaning |
|------|---------|
| 0    | Success: execution completed; or a non-executing operation (validation, inspection) finished cleanly |
| 1    | Execution failed: a block exited non-zero (§4.7) |
| 2    | Spec/usage error: malformed frontmatter, non-runbook document, unsupported `spec` version, unknown language adapter, unknown block attribute, missing script file, missing interpreter, invalid override key, I/O errors |

---

## 7. Grammar summary (EBNF)

```
document     = [ frontmatter ] content
frontmatter  = "---" LF yaml-mapping "---" LF

fence-info   = lang ":" "exec" [ SP attribute ]
attribute    = attr-key "=" attr-value        ; none defined in v1 (load error if present)
lang         = ALPHA *( ALPHA / DIGIT / "-" / "_" / "+" )

script-body  = "@" path                       ; the entire body: exactly one line
path         = 1*CHAR                         ; any characters except NUL and newline

global-name  = ( ALPHA / "_" ) *( ALPHA / DIGIT / "_" )
```

---

## 8. Worked example

`deploy.md`:

````markdown
---
type: runbook
title: Deploy the API
globals: {ENV: staging, IMAGE_TAG: latest, DEPLOY_ID, JOB_NAME}
---

## Preflight

```bash:exec
: "${JOB_NAME:?run with --set JOB_NAME=<name>}"
docker info > /dev/null
```

## Build and push

```bash:exec
docker build -t "ghcr.io/acme/api:$IMAGE_TAG" .
docker push "ghcr.io/acme/api:$IMAGE_TAG"
```

## Register the deployment

```python3:exec
@scripts/register_deploy.py
```

## Verify

```bash:exec
curl -fsS "https://$ENV.api.acme.dev/healthz"
echo "deploy $DEPLOY_ID is live"
```
````

`scripts/register_deploy.py`:

```python
import json, os, urllib.request

body = json.dumps({"image": os.environ["IMAGE_TAG"], "job": os.environ["JOB_NAME"]}).encode()
req = urllib.request.Request("https://api.acme.dev/deployments", data=body, method="POST")
resp = json.load(urllib.request.urlopen(req))
os.environ["DEPLOY_ID"] = resp["id"]   # exported: harvested exactly like bash's `export`
```

Invocation (launch-time overrides `IMAGE_TAG=v1.2.3`, `JOB_NAME=nightly` — spelled
e.g. `--set IMAGE_TAG=v1.2.3 --set JOB_NAME=nightly` on a CLI runner):

Global state through the run:

| Stage                    | ENV       | IMAGE_TAG  | DEPLOY_ID | JOB_NAME  |
|--------------------------|-----------|------------|-----------|-----------|
| pre-flight (composed)    | `staging` | `v1.2.3`   | absent    | `nightly` |
| after Preflight          | —         | —          | —         | —         |
| after Build and push     | —         | —          | —         | —         |
| after Register (harvest) | —         | —          | `dpl_42`  | —         |
| Verify reads             | ✓         | ✓          | ✓         | ✓         |

Reference-runner view (illustrative; presentation conventions are interface, not spec):

```
▶ [1/4] bash — line 9
▶ [2/4] bash — line 15
#2: sha256:9f2c… pushed
▶ [3/4] python3 — @scripts/register_deploy.py (line 21)
▶ [4/4] bash — line 26
{"health":"ok"}
deploy dpl_42 is live
```

---

## 9. Conformance

A conforming runner MUST:

1. Treat documents without `type: runbook` as ordinary Markdown and refuse to
   execute them.
2. Implement §2 validation and §6.1 diagnostics; follow the §6.2 exit-code
   convention if it exposes process exit codes.
3. Compose block environments per §4.2, including masking of declared names.
4. Implement harvest per §4.3 with closed-world warnings per §4.4.
5. Provide at least the adapters it documents, satisfying the §5.1 contract.
6. Complete §5.3 pre-flight — reporting all diagnostics — before executing any block.
7. Never modify block source: no substitution, reordering, or injection visible to
   the language.
8. Expose execution per §4.8: block output attributable to its block and visible
   live.

A conforming runner MAY add adapters, interface surface, color, and richer diagnostics. It
MUST NOT add execution semantics that alter the §4 state model (e.g. interpolation,
persistent sessions) while claiming conformance to v1.

## 10. Versioning

This is specification version **1**. Documents declare compatibility with `spec: 1`
(default). Future versions MUST increment the integer and SHOULD keep v1 documents
executable under later parsers where practical. Runners MUST reject documents
targeting versions they do not implement.

---

## Appendix A — Deferred to v2

Named here so v1 runners can reject them loudly rather than misinterpret them:

- Block attributes: `name=`, `on_error=`, `timeout=`
- `required: [KEY, ...]` — launch-time enforcement of absent globals
- Per-global metadata: `secret`, `description`, `inherit` (host-env fallback for
  declared names)
- Built-in values (`DATE` auto-fill and friends)
- `${VAR}` host-environment references in global defaults
- Opt-in `{{name}}` interpolation
- Arguments to script references (`@script.sh --flag`)
- Output capture into globals beyond env mechanics
- `--only` / `--from` (partial runs, resume)
- Parallel or conditional blocks
- Typed globals (numbers, booleans, secrets)
- Adapters: `sh`, `zsh`, `node`
- `--json` machine-readable output

## Appendix B — Design rationale (informative)

- **Closed world.** The frontmatter is the complete, self-documenting contract of the
  document's variable surface. Undeclared exports are noise (tool-internal variables,
  `PATH` churn) and must not flow forward. Strictness at launch-time overrides (error
  on undeclared keys) gives the same protection to the launch-time path — typo'd keys
  fail loudly.

- **Fresh process per block + env-delta harvest.** One state rule, uniform across
  languages. The persistent-session alternative is shell-flavored and cannot express
  the same rule for Python. Isolation also makes re-runs deterministic: no leaked
  `cd`, functions, or imports.

- **Environment-only value delivery.** No interpolation means the block source is
  byte-for-byte what executes: no escaping hazards, source views are perfectly
  faithful, and any language that reads environment variables can consume globals
  while any language whose process environment can be observed at exit can set them.

- **Loud rejection of unknown syntax.** Reserved attributes and unknown spec versions
  fail on older runners instead of executing with different meaning.

- **Harvest applied even on failure.** Semantics stay total; a future
  `on_error: continue` requires no additional rule.

- **Attribution of output.** Block output must never lose attribution to its block
  (§4.8). Stream-based interfaces reinforce this by keeping narration on stderr so
  stdout is exactly the concatenation of block output.

- **Bare key = declared-absent, not "required".** The frontmatter is a variable
  inventory; whether a value must exist at launch is a runtime concern best enforced
  where the value is needed (`${VAR:?message}`), with the author's own message.

## Appendix C — Implementation notes (Go, informative)

- **YAML:** `gopkg.in/yaml.v3`. Decode `globals` into a nilable map
  (`map[string]*string` or a custom `UnmarshalYAML`) so bare keys decode as
  "declared, absent" rather than empty strings. Bare keys in flow mappings
  (`{A, B: "x"}`) were verified against libyaml-lineage parsers; re-confirm with
  yaml.v3 during implementation.
- **Duplicate globals keys:** rely on yaml.v3's duplicate-key detection or post-check.
- **Markdown parsing:** use a CommonMark parser (e.g. `goldmark`) and walk code-block
  nodes for fence info strings and bodies; do not regex the document.
- **bash harness:** snapshot the composed environment; `eval` embedded source (or
  `source` the referenced script) inside a controlled shell; dump `env -0` from an
  `EXIT` trap; diff the NUL-delimited snapshots. The trap must fire even when user
  code exits early (`set -e`, `exit`), and the harness must preserve the user exit
  status.
- **python adapter:** generated wrapper — a prelude snapshots `os.environ` and
  registers an `atexit` hook that writes the delta to a runner-provided side channel
  (an extra file descriptor or a temp-file path in an internal environment variable);
  embedded source then runs via `exec(compile(...))`, script references via `runpy`.
  Uncaught exceptions print a traceback and exit 1, as usual.
- **Shared load path:** frontmatter parsing, block extraction, and validation are one
  code path used by `check`, `list`, `globals`, `print`, and the TUI.
- **Process management:** each block runs in its own process group for clean Ctrl-C
  teardown; stdin is `/dev/null`; the environment is composed per §4.2.
