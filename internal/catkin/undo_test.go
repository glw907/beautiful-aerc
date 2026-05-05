package catkin

import "testing"

func TestUndoRing_RecordCoalesce(t *testing.T) {
	var r undoRing
	r.seed(snap{"", 0})
	r.record(snap{"h", 1})
	r.record(snap{"he", 2})
	r.record(snap{"hel", 3})
	if got := len(r.snaps); got != 2 {
		t.Fatalf("intra-word coalesce: want 2 entries, got %d (%v)", got, r.snaps)
	}
	r.record(snap{"hel ", 4})
	r.record(snap{"hel w", 5})
	if got := len(r.snaps); got != 4 {
		t.Fatalf("post-space new entry: want 4, got %d (%v)", got, r.snaps)
	}
}

func TestUndoRing_UndoRedo(t *testing.T) {
	var r undoRing
	r.seed(snap{"", 0})
	r.record(snap{"a", 1})
	r.record(snap{"a ", 2})
	r.record(snap{"a b", 3})

	got, ok := r.undo()
	if !ok || got.val != "a " {
		t.Fatalf("undo 1: got %+v", got)
	}
	got, ok = r.undo()
	if !ok || got.val != "a" {
		t.Fatalf("undo 2: got %+v", got)
	}
	got, ok = r.redo()
	if !ok || got.val != "a " {
		t.Fatalf("redo: got %+v", got)
	}

	r.record(snap{"a B", 3})
	if _, ok := r.redo(); ok {
		t.Fatalf("record after undo should clear redo stack")
	}
}

func TestUndoRing_Cap(t *testing.T) {
	var r undoRing
	r.seed(snap{"", 0})
	for i := 0; i < undoCap*2; i++ {
		r.record(snap{string(rune('a'+i%26)) + " ", i})
	}
	if len(r.snaps) > undoCap {
		t.Fatalf("ring exceeded cap: %d", len(r.snaps))
	}
}
