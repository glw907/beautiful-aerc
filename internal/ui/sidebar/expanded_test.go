package sidebar

import "testing"

func TestExpanded_ToggleAndPrune(t *testing.T) {
	m := Model{expanded: map[string]bool{}}
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
