package tui

import (
	"fmt"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/session"
)

// reloadDocument re-reads the document from disk and replays the current
// session against it. Executable blocks are matched by content key: unchanged
// runs keep their output, changed runs start fresh, and outputs that depended
// on changed state are marked stale.
func (m *Model) reloadDocument() {
	if m.inlineActive() {
		m.flash = "stop the running block first (s), then reload"
		return
	}
	res, err := doc.Load(m.doc.Path)
	if err != nil {
		m.flash = "reload failed: " + err.Error()
		return
	}
	if res.HasErrors() {
		errs, _ := res.Counts()
		m.flash = fmt.Sprintf("reload failed: %d error(s) in document", errs)
		return
	}
	res.Sort()

	// Remember the heading the reader was on; outline indices shift when a
	// document changes, so restore it by label after the rebuild.
	heading := ""
	if m.outline.sel >= 0 && m.outline.sel < len(m.outline.entries) {
		heading = m.outline.entries[m.outline.sel].label
	}

	m.captureSnapshot()
	plan, err := m.snap.Plan(res.Doc, m.overrides)
	if err != nil {
		m.flash = "reload failed: " + err.Error()
		return
	}

	if m.inlineSession != nil {
		m.inlineSession.Close()
		m.inlineSession = nil
	}
	m.doc = res.Doc
	if keys, err := session.BlockKeys(res.Doc); err == nil {
		m.blockKeys = keys
	} else {
		m.blockKeys = nil
	}
	// Rebuild inline state from the plan: blocks whose identity changed have
	// no restored entry and must start completely fresh.
	m.inline = map[int]*inlineState{}
	interrupted := applyRestore(m, plan)
	m.resuming = true
	m.restoredState = plan.State
	m.blockSel = plan.BlockSel

	m.snap.Keys = append([]string(nil), m.blockKeys...)
	m.snap.BlockSel = m.blockSel
	if m.blockSel >= 0 && m.blockSel < len(m.blockKeys) {
		m.snap.BlockSelKey = m.blockKeys[m.blockSel]
	} else {
		m.snap.BlockSelKey = ""
	}
	m.snap.Globals = cloneStringMap(plan.State.Globals)
	m.snap.Absent = append([]string(nil), plan.State.Absent...)
	m.snap.Next = plan.State.Next

	m.relayout()
	if heading != "" {
		for i, entry := range m.outline.entries {
			if entry.label == heading {
				m.outline.sel = i
				m.jumpToSelected()
				break
			}
		}
	}
	m.saveSession()

	summary := fmt.Sprintf("reloaded — %d changed block(s), %d stale output(s)",
		len(plan.Changed), len(plan.Stale))
	if interrupted > 0 {
		summary += fmt.Sprintf(", %d interrupted run(s) stopped", interrupted)
	}
	m.flash = summary
}
