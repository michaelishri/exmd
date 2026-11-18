package exec

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec" //nolint:depguard // the standard library's exec; package name collision is intentional
	"path/filepath"
)

// Adapter executes blocks of one language (SPEC.md §5.1). Both v1 adapters
// share the same shape: the engine stages the block source in a file, hands
// the paths to the interpreter via internal environment variables
// (`__MDEXEC_*`, §5.1.5), and the adapter reports the block's final process
// environment through `__MDEXEC_ENVOUT`.
type Adapter struct {
	Bin  string // interpreter binary
	Ext  string // extension for staged embedded sources
	Args func(tmpDir string) []string
}

// Lookup returns the adapter for a fence language (SPEC.md §5.2).
func Lookup(lang string) (Adapter, bool) {
	a, ok := adapters[lang]
	return a, ok
}

var adapters = func() map[string]Adapter {
	python := Adapter{
		Bin:  "python3",
		Ext:  "py",
		Args: func(dir string) []string { return []string{filepath.Join(dir, "harness.py")} },
	}
	return map[string]Adapter{
		"bash": {
			Bin:  "bash",
			Ext:  "sh",
			Args: func(dir string) []string { return []string{filepath.Join(dir, "harness.sh")} },
		},
		"python3": python,
		"python":  python, // alias (SPEC.md §5.2)
	}
}()

// command builds the interpreter command for a run.
func (a Adapter) command(tmpDir string) *exec.Cmd {
	return exec.Command(a.Bin, a.Args(tmpDir)...)
}

// writeHarnesses stages the per-run harness files. The harnesses are static;
// block-specific inputs arrive via the internal environment variables.
func writeHarnesses(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, "harness.sh"), []byte(bashHarness), 0o644); err != nil {
		return fmt.Errorf("write bash harness: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "harness.py"), []byte(pythonHarness), 0o644); err != nil {
		return fmt.Errorf("write python harness: %w", err)
	}
	return nil
}

// bashHarness sources the block inside a controlled shell and dumps the
// shell's exported environment — the environment the block would hand to
// its children (SPEC.md §4.3) — from an EXIT trap. The trap fires even when
// user code exits early (`exit`, `set -e`), and the shell's exit status
// passes through unchanged. `compgen -e` lists exported names without
// spawning a child (a child would inject `_=/usr/bin/env` into the dump);
// values are transported NUL-delimited, losslessly (§4.3).
//
//go:embed harness.sh
var bashHarness string

// pythonHarness runs the block as __main__ via runpy and dumps the final
// os.environ from an atexit hook, which fires on sys.exit and uncaught
// exceptions alike. os.fsencode round-trips values losslessly.
//
//go:embed harness.py
var pythonHarness string
