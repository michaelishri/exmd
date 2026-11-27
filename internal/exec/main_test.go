package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain lets the suite run against a specific bash. EXMD_TEST_BASH names
// the binary (for example /bin/bash on macOS, which is bash 3.2); it is put
// first on PATH as `bash`, exactly how the adapter finds its interpreter.
func TestMain(m *testing.M) {
	if bin := os.Getenv("EXMD_TEST_BASH"); bin != "" {
		dir, err := os.MkdirTemp("", "exmd-test-bash-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.Symlink(bin, filepath.Join(dir, "bash")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		code := m.Run()
		os.RemoveAll(dir)
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// TestHarnessBashVersion reports which bash the suite exercises, so a run
// against bash 3.2 is visible in the test log.
func TestHarnessBashVersion(t *testing.T) {
	r, out, _ := runSource(t, bashDoc(`echo "$BASH_VERSION"`), nil)
	if r.Failed != nil {
		t.Fatalf("bash did not run")
	}
	t.Logf("bash %s", out)
}
