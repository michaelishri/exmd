# md-exec — Hand-off Document

**Date:** 2026-11-18
**From:** development machine (macOS, darwin/arm64, Go 1.27.2)
**State:** implementation complete per plan; awaiting design feedback
**Branch:** `main`, clean tree, **no remote configured** (see §2 for transfer options)

---

## 1. What this project is

**md-exec** defines and implements *executable Markdown* ("runbooks"): ordinary
`.md` files whose fenced code blocks execute in order, sharing state through
environment variables.

The repo deliberately separates two things — keep it that way:

| | **The specification** | **The tool** |
|---|---|---|
| Lives in | `SPEC.md` | the Go implementation in this repo |
| Defines | document format + execution semantics: frontmatter, `globals`, `lang:exec` fences, `@script` references, the env-harvest state rule | how humans use the runner: TUI, rendering, colors, keys |
| Normative? | yes, versioned (`spec: 1`) | no — one conforming implementation |

Rules of thumb: a change to what a *document* means or how *blocks execute*
belongs in `SPEC.md`; a change to how the human drives the runner is tool-only.
The tool may add UX but never weakens spec semantics (§9 conformance).

Read in this order: `SPEC.md` → `README.md` → code tour (§6 below).

## 2. Moving the repository

No git remote exists yet. Three options, best first:

```bash
# Option A (recommended): push to a real remote before moving
git remote add origin git@github.com:michaelishri/md-exec.git
git push -u origin main

# Option B: bundle file, self-contained
git bundle create ../md-exec.bundle --all
# on the new machine:
git clone md-exec.bundle md-exec && cd md-exec && git checkout main

# Option C: copy the whole directory (includes .git)
```

**First hour on the new machine:**

```bash
go build -o md-exec ./cmd/md-exec   # the checked-in binary is gitignored — rebuild
go test ./...                        # everything must pass (see §7)
./md-exec check examples/demo/demo.md
./md-exec examples/demo/demo.md --set JOB_NAME=nightly   # needs a real TTY
```

Verify: `git log --oneline` matches §3, all tests pass, the demo opens and `r`
runs it end-to-end.

## 3. Commit history (context for review)

```
0265168 Tidy go.mod: explicit require block
51cf793 Step-through and cancellation: session-based engine
554165a Run mode: live execution in the TUI
df4db76 Execution core: adapters, env harvest, §4 state machine
5c5dde7 TUI viewer: borderless panes, outline sidebar, print verb
9a6bacd Load path: frontmatter, blocks, outline, headless verbs
bd273aa Initial commit: spec v1, README, and reference docs
b97feb8 Add .gitignore
```

Scale: 73 tracked files, ~6,300 lines, 35 test functions. Developed and tested
on macOS (darwin/arm64); **the engine is Unix-only** (see §8).

## 4. Repository layout

```
SPEC.md                        the v1 specification — the contract
README.md                      intro, spec/tool separation, status roadmap
docs/10-design-principles-…    Atlassian CLI design reference (shapes tool UX)

cmd/md-exec/                   CLI entry point
  main.go                      dispatch, usage, shared styles, diagnostics printer
  check.go                     validation verb (§2.3 collect-all + preflight)
  list.go                      enumerate executable blocks
  globals.go                   parameter surface; hand-rolled --set parsing (any position)
  print.go                     render document to terminal (shared renderer)
  tui.go                       bare `DOC [--set K=V]` → bubbletea program

internal/doc/                  SPEC §2–3 load path (no I/O side effects)
  load.go                      splitFrontmatter masking trick, goldmark AST walk
  frontmatter.go               YAML validation, flow/block globals, bare keys
  blocks.go                    fence grammar (lang:exec), script refs, docdir resolve
  preflight.go                 §2.3 environment checks; KnownLanguages table
  globals.go                   ResolveGlobals (defaults < overrides, §2.2)
  diagnostics.go               Severity/Diagnostic, collect-all, Sort
  doc.go                       Document/Block/Heading types, Source, BodyLine
  load_test.go blocks_test.go  fixture-driven + grammar unit tests
  testdata/                    17 fixtures: 6 valid (incl. CRLF, list items),
                               11 invalid (one diagnostic class each, plus
                               invalid-many.md kitchen sink)

internal/render/               doc + width → styled terminal lines (pure function)
  render.go                    chunk boundaries (headings + fences), marks, dedent
  style.go                     glamour per-chunk append, badge lines, darkStyle()
  render_test.go               structure, width, determinism

internal/theme/theme.go        design tokens (Dark, Notty) — OpenCode-style

internal/tui/                  the TUI (viewer + run mode)
  tui.go                       Model/Update/View, layout, focus, scroll-spy
  run.go                       run mode: events → state, session plumbing, status bar
  outline.go                   sidebar entries (Outline/Blocks tabs)
  keys.go                      keymap, doubles as help source
  styles.go                    token-derived lipgloss styles
  tui_test.go run_test.go      headless model tests (pump helper drives tea.Cmd chains)

internal/exec/                 SPEC §4–5 execution engine (Unix-only)
  engine.go                    Session: NewSession / RunNext / Close; Run wrapper
  env.go                       compose (masking), diffEnv, parseEnv, shellManaged
  adapters.go                  Adapter table (bash, python3|python), go:embed
  harness.sh                   bash harness — EXIT trap + compgen -e NUL dump
  harness.py                   python harness — runpy + atexit NUL dump
  events.go                    Event kinds (§4.8 observation)
  env_test.go engine_test.go   every §4 semantic, as unit + behavioral tests
  demo_test.go                 examples/demo runbook end-to-end anchor
  testdata/                    10 docs + helper.sh/helper.py (helpers for script refs)

examples/demo/                 the feature-tour runbook (see §9) + scripts/
```

Dependencies (pinned in `go.sum`, not vendored): bubbletea v1.3.10, lipgloss
v1.1.1-pseudo (required by glamour v1.0.0), bubbles v1.0.0, glamour v1.0.0,
x/ansi v0.11.9, termenv v0.16.0, goldmark v1.8.7, x/term v0.48.0, yaml.v3
v3.0.1. First build needs module-proxy access (`go mod download`); `go mod
vendor` is an option if the new machine is offline.

## 5. Architecture: the seven load-bearing decisions

1. **No templating.** Block source is byte-for-byte what executes; values
   arrive only via the process environment. This is why `--dry-run`-style
   fidelity is trivial and there are no escaping rules. Do not add
   interpolation without a spec change.

2. **State = env delta harvest (§4.3).** Before each block:
   `env := (host_env − declared_names) ∪ globals` (declared names mask the
   host, §2.2; globals appended last so they win). After each block, diff the
   env the block would hand to its children against what was passed in:
   added/changed → globals (if declared), unset → removed, undeclared →
   warning + ignored (§4.4 closed world). The delta applies *before* failure
   evaluation, so semantics are total.

3. **Adapter harnesses are the only interpreter coupling** (§5.1). bash:
   static harness sources the block and dumps `compgen -e` NUL-delimited from
   an `EXIT` trap (survives `exit`/`set -e`, status passes through, works on
   stock bash 3.2). Do NOT switch to `env -0` — the child process injects
   `_=/usr/bin/env` into the dump. python: wrapper runs the block as
   `__main__` via `runpy`, dumps `os.environ` from an `atexit` hook.
   Machinery is confined to `__MDEXEC_*` env names.

4. **`shellManaged` suppression (§5.1.6).** bash rewrites `PWD`/`SHLVL`/
   `OLDPWD`/`_` every block; without suppression every bash block warns.
   Adapters must keep this list updated per interpreter.

5. **Blocks run in docdir** (§4.2), fresh process per block, stdin `/dev/null`,
   in their own process group. Cancellation (context) SIGTERMs the group,
   then SIGKILLs after 2s (§4.7); a killed block produces no dump → no
   harvest, and the run stops there with `Interrupted`.

6. **Session, not one-shot.** `Session.RunNext` executes one block carrying
   state; `Run(ctx, …)` loops it. This is what powers TUI step-through (`n`),
   continue (`r`), and stop (`s`). Rule: only touch a Session while no
   `RunNext` is in flight.

7. **The renderer is a pure function** `(doc, width, color) → lines + marks`,
   shared by the TUI and `print`. It chunks the source at heading/fence
   boundaries, glamour-renders each chunk (stripping `:exec` from the fence
   info *in the render input only*, so chroma lexes correctly), records the
   rendered line of every heading and block, and adds badge lines. Inter-chunk
   spacing: one blank line unless the previous chunk already ends visually
   blank (glamour pads heading blocks).

## 6. Suggested code-reading order

1. `SPEC.md` §4.2–§4.4 (composition + harvest) — then `internal/exec/env.go`
   (compose/diff, ~100 lines, the heart)
2. `internal/exec/harness.sh` + `harness.py` — the adapter contract made real
3. `internal/exec/engine.go` — Session, cancellation
4. `internal/doc/load.go` (frontmatter masking trick: YAML content is blanked
   with spaces so byte offsets stay valid for the AST walk)
5. `internal/render/render.go` — chunk boundaries
6. `internal/tui/tui.go` then `run.go` — event → state → view

## 7. Testing strategy

```bash
go test ./...      # ~35 tests, all packages, <5s
go vet ./...       # kept clean
gofmt -l .         # kept empty
```

- `internal/doc` — fixtures under `testdata/` prove each diagnostic class and
  each valid shape. `invalid-many.md` exercises collect-all reporting (§2.3).
  `valid-crlf.md` is genuinely CRLF — don't normalize it.
- `internal/exec` — one test per §4 semantic (harvest, masking, isolation,
  failure-stop-with-harvest, exit codes, alias routing, tracebacks, script
  refs, empty stdin, undeclared-override refusal), plus:
  - `TestDemoRunbook` — the examples/demo runbook through the real engine
    (the end-to-end anchor; keep it passing).
  - `TestCancelInterruptsBlock` — cancels 200ms into a `sleep 5`; asserts the
    process group dies fast (~0.2s).
  - `TestSessionStepping` — state carries across `RunNext` calls.
- `internal/tui` — headless model tests: build the model, send `tea.KeyMsg`s,
  and use the `pump` helper to execute `tea.Cmd` chains synchronously
  (recursing into `tea.BatchMsg`). `eventChanBuffer` (4096) exists so this
  synchronous driving can't deadlock; the live program drains concurrently.
- `internal/render` — structure/ordering, width budget, determinism.

There is **no CI configured yet** — a GitHub Actions workflow running
`go vet`, `gofmt -l`, and `go test ./...` on ubuntu + macOS would be the
natural first addition.

## 8. Platform constraints and gotchas

- **Unix-only engine.** `syscall.SysProcAttr{Setpgid: true}` and
  `syscall.Kill(-pid, …)` do not build on Windows. Cross-platform needs
  build-tagged process management (job objects on Windows).
- **Interpreters must exist** for execution: `bash` (3.2+ ok), `python3`.
  `check` reports missing ones; TUI refuses error docs outright.
- **TUI needs a real TTY** — `md-exec DOC` exits 2 with a hint when stdout
  isn't a terminal (verified behavior; also what happens if you run it from
  inside another tool's shell).
- **glamour quirk:** lines containing ambiguous-width characters (`§`, `—`)
  near the wrap edge can break one word early (its width measurement counts
  them wide). Cosmetic, accepted; if it bothers you, it's a glamour/measure
  issue, not ours.
- **Examples/demo intentionally contains** an unknown frontmatter key
  (`tags:`) to exercise the warning path, and a `false || echo …` to show the
  failure escape hatch. Don't "fix" them.
- Frontmatter line numbers in diagnostics are offset by the opening `---`
  line (`adjustYamlLine` is best-effort regex on yaml.v3 messages).

## 9. The demo runbook is the living spec example

`examples/demo/demo.md` (+ `scripts/`) exercises every v1 feature a document
can self-demonstrate: flow-form globals with defaults and bare keys, a
point-of-use guard (`${JOB_NAME:?…}`), `cd` isolation, bash `export` harvest,
python `os.environ` harvest via the `python` alias, script references
(bash + python3), a lossless multi-line value, harvest-of-changed values,
unexported-invisible state, closed-world warning (`UNDECLARED_X`), `|| true`,
unset propagation, display-only fences, and h1–h3 outline depth.
`SPEC.md` §8's worked example mirrors its first half. Keep demo, spec, and
tests in agreement when semantics change.

## 10. Status and what's next

**Complete per plan:** spec v1 · load path with collect-all diagnostics ·
headless verbs (`check`, `list`, `globals`, `print`) · borderless TUI viewer
(¾ content + ¼ shaded sidebar, Outline/Blocks tabs, unified scroll-spy
cursor) · execution core (§4–5) · run mode (`r` run, `n` step, `s` stop,
live globals inspector, `--set` overrides).

**Known open items:**

1. **Design feedback pending** — the author has tested the TUI and is saving
   feedback until after full implementation. *Collect it first* before
   polishing visuals.
2. `--theme` flag (tokens exist in `internal/theme`; a setter + flag wiring
   is small).
3. No CI, no remote, no LICENSE file yet.
4. Deferred v2 list lives in SPEC Appendix A (block attributes, `required:`
   globals, per-global metadata, script args, `--only`/`--from`, parallel/
   conditional blocks, typed globals, sh/zsh/node adapters, `--json`).
5. Signal handling is TUI-side only (`shutdownRun` on quit). A future headless
   runner must wire SIGINT → context cancel → exit 130 (§4.7).

## 11. Invariants to preserve

- `SPEC.md` is the contract: document-semantics changes require a spec edit
  (and eventually a `spec:` bump); tool UX changes don't touch it.
- Never modify block source before execution (§9.7) — the `:exec` strip
  happens on render input only.
- Closed world stays closed: undeclared overrides and exports must not leak.
- Keep `go vet` and `gofmt` clean; tests pass before any commit.
- The spec/tool separation table in README stays honest.
