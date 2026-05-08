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

