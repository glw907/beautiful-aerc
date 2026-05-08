package compose

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func newTestPicker(t *testing.T) AttachPicker {
	t.Helper()
	styles := NewStyles(theme.OneDark)
	return NewAttachPicker(styles, uicore.SimpleIcons)
}

func TestAttachPicker_StartsClosed(t *testing.T) {
	p := newTestPicker(t)
	if p.IsOpen() {
		t.Fatal("new picker should not be open")
	}
}

func writeTree(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAttachPicker_OpenReadsDir(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "alpha.txt", "beta.txt", "sub/", ".hidden")

	p := newTestPicker(t)
	p, cmd := p.Open(dir)
	if cmd == nil {
		t.Fatal("Open should return a readDir cmd")
	}
	if !p.IsOpen() {
		t.Fatal("Open should mark picker open")
	}
	msg := cmd()
	rd, ok := msg.(readDirMsg)
	if !ok {
		t.Fatalf("expected readDirMsg, got %T", msg)
	}
	if rd.id != p.id {
		t.Fatalf("id mismatch: msg=%d picker=%d", rd.id, p.id)
	}
	p, _ = p.Update(rd)
	names := make([]string, len(p.entries))
	for i, e := range p.entries {
		names[i] = e.name
	}
	sort.Strings(names)
	want := []string{"alpha.txt", "beta.txt", "sub"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v (hidden excluded)", names, want)
	}
}

func feedKeys(p AttachPicker, keys ...string) AttachPicker {
	for _, k := range keys {
		p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(k)}))
	}
	return p
}

func loadDir(t *testing.T, p AttachPicker, dir string) AttachPicker {
	t.Helper()
	p, cmd := p.Open(dir)
	if cmd != nil {
		p, _ = p.Update(cmd())
	}
	p = p.SetSize(60, 10)
	return p
}

func TestAttachPicker_StaleReadDirDropped(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := newTestPicker(t)
	p, _ = p.Open(dir)
	staleID := p.id
	// reopen — bumps id
	p, _ = p.Open(dir)
	stale := readDirMsg{id: staleID, entries: []attachEntry{{name: "ghost", path: "/ghost"}}}
	p, _ = p.Update(stale)
	for _, e := range p.entries {
		if e.name == "ghost" {
			t.Fatal("stale readDirMsg should have been dropped")
		}
	}
}

func TestAttachPicker_SelectAndAccept(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt", "b.txt", "c.txt")
	p := loadDir(t, newTestPicker(t), dir)

	// toggle a and c
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(" ")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(" ")}))

	if c := selectedCount(p); c != 2 {
		t.Fatalf("selected = %d, want 2", c)
	}

	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("a")}))
	if cmd == nil {
		t.Fatal("accept should emit cmd")
	}
	msg := cmd()
	acc, ok := msg.(AttachAcceptedMsg)
	if !ok {
		t.Fatalf("expected AttachAcceptedMsg, got %T", msg)
	}
	if len(acc.Paths) != 2 {
		t.Errorf("Paths len = %d, want 2 (%v)", len(acc.Paths), acc.Paths)
	}
}

func TestAttachPicker_EnterOnFileShortcut(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "only.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter on file with empty selection should accept")
	}
	acc, ok := cmd().(AttachAcceptedMsg)
	if !ok || len(acc.Paths) != 1 || !strings.HasSuffix(acc.Paths[0], "only.txt") {
		t.Fatalf("got %#v", cmd())
	}
}

func TestAttachPicker_AcceptZeroSelectedNoOp(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt", "b.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("a")}))
	if cmd != nil {
		t.Fatal("accept with 0 selected should be no-op")
	}
}

func TestAttachPicker_EscEmitsCancelled(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.txt")
	p := loadDir(t, newTestPicker(t), dir)
	_, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("Esc should emit cmd")
	}
	if _, ok := cmd().(AttachCancelledMsg); !ok {
		t.Fatalf("got %T, want AttachCancelledMsg", cmd())
	}
}

func has(es []attachEntry, name string) bool {
	for _, e := range es {
		if e.name == name {
			return true
		}
	}
	return false
}

func entryNames(es []attachEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.name
	}
	return out
}

func TestAttachPicker_HiddenToggle(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "visible.txt", ".secret")
	p := loadDir(t, newTestPicker(t), dir)
	if has(p.entries, ".secret") {
		t.Fatal("hidden should be excluded by default")
	}
	p, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(".")}))
	if cmd == nil {
		t.Fatal("toggle should re-issue readDirCmd")
	}
	p, _ = p.Update(cmd())
	if !has(p.entries, ".secret") {
		t.Errorf("after toggle: %v should include .secret", entryNames(p.entries))
	}
}

func TestAttachPicker_DescendAndAscendRestoresCursor(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a", "b", "child/", "child/inner.txt")
	p := loadDir(t, newTestPicker(t), dir)

	// find the "child" entry and cursor onto it
	idx := -1
	for i, e := range p.entries {
		if e.name == "child" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("child not in entries")
	}
	for p.cursor < idx {
		p, _ = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("j")}))
	}

	// descend — Enter returns a readDirCmd
	p, cmd := p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("descend should issue readDirCmd")
	}
	p, _ = p.Update(cmd())
	if !strings.HasSuffix(p.dir, "/child") {
		t.Fatalf("after descend dir = %q, want suffix /child", p.dir)
	}
	if p.cursor != 0 {
		t.Errorf("after descend cursor = %d, want 0", p.cursor)
	}

	// ascend — Backspace returns a readDirCmd
	p, cmd = p.Update(tea.KeyMsg(tea.Key{Type: tea.KeyBackspace}))
	if cmd != nil {
		p, _ = p.Update(cmd())
	}
	if p.dir != dir {
		t.Errorf("after ascend dir = %q, want %q", p.dir, dir)
	}
	if p.cursor != idx {
		t.Errorf("after ascend cursor = %d, want %d (restored to child entry)", p.cursor, idx)
	}
}

func TestAttachPicker_Nav(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a", "b", "c", "d", "e")
	p := loadDir(t, newTestPicker(t), dir)

	if p.cursor != 0 {
		t.Fatalf("initial cursor = %d", p.cursor)
	}
	p = feedKeys(p, "j", "j")
	if p.cursor != 2 {
		t.Errorf("after jj: cursor = %d, want 2", p.cursor)
	}
	p = feedKeys(p, "k")
	if p.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", p.cursor)
	}
	p = feedKeys(p, "G")
	if p.cursor != len(p.entries)-1 {
		t.Errorf("after G: cursor = %d, want %d", p.cursor, len(p.entries)-1)
	}
	p = feedKeys(p, "g")
	if p.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", p.cursor)
	}

	// bounds
	p = feedKeys(p, "k", "k")
	if p.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", p.cursor)
	}
	p = feedKeys(p, "G", "j")
	if p.cursor != len(p.entries)-1 {
		t.Errorf("j at bottom: cursor = %d, want %d", p.cursor, len(p.entries)-1)
	}
}

