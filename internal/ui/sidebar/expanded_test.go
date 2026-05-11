package sidebar

import (
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func TestExpanded_ToggleAndPrune(t *testing.T) {
	m := Model{expanded: map[string]bool{}, cache: &renderCache{dirty: true}}
	if m.IsExpanded("a") {
		t.Fatal("new model: nothing expanded")
	}
	m.ToggleExpanded("a")
	if !m.IsExpanded("a") {
		t.Fatal("after toggle: a should be expanded")
	}
	m.ToggleExpanded("a")
	if m.IsExpanded("a") {
		t.Fatal("after second toggle: a should be collapsed")
	}

	m.expanded = map[string]bool{"a": true, "vanished": true, "b/c": true}
	known := map[string]struct{}{"a": {}, "b/c": {}}
	m.pruneExpanded(known)
	if m.expanded["vanished"] {
		t.Errorf("pruneExpanded should drop vanished keys: %+v", m.expanded)
	}
	if !m.expanded["a"] || !m.expanded["b/c"] {
		t.Errorf("pruneExpanded must keep live keys: %+v", m.expanded)
	}
}

func TestView_SpartanCapsDepthAtOne(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "a/b/c/leaf"}, DisplayName: "a/b/c/leaf", Group: mail.GroupCustom},
	}
	m := New(Styles{}, classified, config.UIConfig{}, 14, 10, uicore.SimpleIcons)
	m.SetLayout(uicore.LayoutMode{Spartan: true})
	m.ToggleExpanded("a")
	rows := m.visibleRows()
	for _, r := range rows {
		if r.depth > 1 {
			t.Errorf("Spartan must cap depth at 1, got row %+v", r)
		}
	}
}
