package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/exec"
)

const threeBlockSource = `---
type: runbook
globals: {A, B, C}
---

` + "```bash:exec\n" + `export A=one
` + "```\n" + `
` + "```bash:exec\n" + `export B=two
` + "```\n" + `
` + "```bash:exec\n" + `export C=three
` + "```\n"

func TestBlockKeysIgnoreProseButTrackCode(t *testing.T) {
	path := writeDoc(t, threeBlockSource)
	d := load(t, path)
	keys, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}

	prosePath := writeDoc(t, threeBlockSource+"\nextra prose\n")
	proseDoc := load(t, prosePath)
	proseKeys, err := BlockKeys(proseDoc)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keys, proseKeys) {
		t.Fatalf("prose edit changed block keys: %v -> %v", keys, proseKeys)
	}

	edited := strings.Replace(threeBlockSource, "export B=two", "export B=changed", 1)
	codePath := writeDoc(t, edited)
	codeDoc := load(t, codePath)
	codeKeys, err := BlockKeys(codeDoc)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(keys, codeKeys) {
		t.Fatal("editing a block did not change its key")
	}
}

func TestPlanRollsBackAtChangedBlock(t *testing.T) {
	path := writeDoc(t, threeBlockSource)
	d := load(t, path)
	s, err := New(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}
	s.Keys = keys

	s.Blocks = []BlockSnapshot{
		{Key: keys[0], Index: 0, Lines: []string{"one"}, Done: true},
		{Key: keys[1], Index: 1, Lines: []string{"two"}, Done: true},
		{Key: keys[2], Index: 2, Lines: []string{"three"}, Done: true},
	}
	state0 := exec.SessionState{Globals: map[string]string{"A": "one"}, Absent: []string{"B", "C"}, Next: 1}
	state1 := exec.SessionState{Globals: map[string]string{"A": "one", "B": "two"}, Absent: []string{"C"}, Next: 2}
	state2 := exec.SessionState{Globals: map[string]string{"A": "one", "B": "two", "C": "three"}, Next: 3}
	s.Runs = []RunSnapshot{
		{Key: keys[0], After: state0},
		{Key: keys[1], After: state1},
		{Key: keys[2], After: state2},
	}
	s.Globals = cloneMap(state2.Globals)
	s.Absent = append([]string(nil), state2.Absent...)
	s.Next = state2.Next

	edited := strings.Replace(threeBlockSource, "export B=two", "export B=changed", 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	changed := load(t, path)
	plan, err := s.Plan(changed, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got := plan.State.Globals["A"]; got != "one" {
		t.Fatalf("globals rolled back too far: A=%q", got)
	}
	if _, ok := plan.State.Globals["B"]; ok {
		t.Fatal("globals from the changed block survived")
	}
	if _, ok := plan.State.Globals["C"]; ok {
		t.Fatal("globals from blocks after the change survived")
	}

	first := plan.Blocks[0]
	if first.Stale || strings.Join(first.Lines, "\n") != "one" {
		t.Fatalf("unchanged block was not restored: %+v", first)
	}
	if _, ok := plan.Blocks[1]; ok {
		t.Fatal("changed block kept its old output")
	}
	third := plan.Blocks[2]
	if !third.Stale || strings.Join(third.Lines, "\n") != "three" {
		t.Fatalf("later output was not marked stale: %+v", third)
	}
	if len(plan.Changed) != 1 || len(plan.Stale) != 1 {
		t.Fatalf("changed=%v stale=%v", plan.Changed, plan.Stale)
	}
}

func TestPlanWithoutChangesRestoresFinalState(t *testing.T) {
	path := writeDoc(t, threeBlockSource)
	d := load(t, path)
	s, err := New(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}
	s.Keys = keys
	final := exec.SessionState{
		Globals: map[string]string{"A": "one"},
		Absent:  []string{"B", "C"},
		Next:    3,
	}
	s.Runs = []RunSnapshot{{Key: keys[0], After: final}}
	s.Globals = cloneMap(final.Globals)
	s.Absent = append([]string(nil), final.Absent...)
	s.Next = final.Next

	plan, err := s.Plan(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.State.Globals["A"]; got != "one" {
		t.Fatalf("final state not restored: %q", got)
	}
	if len(plan.Changed) != 0 || len(plan.Stale) != 0 {
		t.Fatalf("unexpected invalidation: changed=%v stale=%v", plan.Changed, plan.Stale)
	}
}

func TestBlockKeysIncludeScriptContents(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.sh")
	if err := os.WriteFile(script, []byte("echo old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "runbook.md")
	src := "---\ntype: runbook\n---\n\n```bash:exec\n@helper.sh\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	d := load(t, path)
	before, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("echo new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}
	if before[0] == after[0] {
		t.Fatal("script edit did not change block key")
	}
}
