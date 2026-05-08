package compose

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

