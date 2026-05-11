package sidebar

import (
	"reflect"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func customEntry(name string, unseen int) folderEntry {
	return folderEntry{
		cf: mail.ClassifiedFolder{
			Folder:      mail.Folder{Name: name, Unseen: unseen},
			DisplayName: name,
			Group:       mail.GroupCustom,
		},
	}
}

func TestWalkCustom_FlatLeaves(t *testing.T) {
	in := []folderEntry{customEntry("Lists", 0), customEntry("Receipts", 2)}
	got := walkCustom(in, nil, 0)
	want := []rowMeta{
		{entry: in[0], depth: 0, isLast: false, aggUnread: 0, hasChildren: false},
		{entry: in[1], depth: 0, isLast: true, aggUnread: 0, hasChildren: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestWalkCustom_NestedExpanded(t *testing.T) {
	in := []folderEntry{
		customEntry("Lists/golang", 3),
		customEntry("Lists/rust", 1),
		customEntry("Receipts", 0),
	}
	expanded := map[string]bool{"Lists": true}
	got := walkCustom(in, expanded, 0)
	if len(got) != 4 {
		t.Fatalf("want 4 rows (Lists parent + 2 children + Receipts), got %d", len(got))
	}
	if got[0].entry.cf.DisplayName != "Lists" || got[0].depth != 0 || !got[0].hasChildren {
		t.Errorf("row 0: want synthesized parent Lists at depth 0 with children, got %+v", got[0])
	}
	if got[1].depth != 1 || got[1].entry.cf.Folder.Name != "Lists/golang" || got[1].isLast {
		t.Errorf("row 1: want golang at depth 1 non-last, got %+v", got[1])
	}
	if got[2].depth != 1 || got[2].entry.cf.Folder.Name != "Lists/rust" || !got[2].isLast {
		t.Errorf("row 2: want rust at depth 1 last, got %+v", got[2])
	}
	if got[3].depth != 0 || got[3].entry.cf.Folder.Name != "Receipts" || !got[3].isLast {
		t.Errorf("row 3: want Receipts at depth 0 last, got %+v", got[3])
	}
}

func TestWalkCustom_CollapsedAggregatesUnread(t *testing.T) {
	in := []folderEntry{
		customEntry("Lists/golang", 3),
		customEntry("Lists/rust", 1),
	}
	got := walkCustom(in, nil, 0) // Lists collapsed (no entry in expanded map)
	if len(got) != 1 {
		t.Fatalf("want 1 row (collapsed Lists), got %d", len(got))
	}
	if got[0].aggUnread != 4 {
		t.Errorf("collapsed Lists: want aggUnread=4, got %d", got[0].aggUnread)
	}
	if !got[0].hasChildren {
		t.Errorf("collapsed Lists: want hasChildren=true")
	}
}

func TestWalkCustom_MaxDepthCap(t *testing.T) {
	in := []folderEntry{
		customEntry("a/b/c/leaf", 5),
		customEntry("a/b/sibling", 1),
	}
	expanded := map[string]bool{"a": true, "a/b": true, "a/b/c": true}
	got := walkCustom(in, expanded, 1) // cap at depth 1
	for _, r := range got {
		if r.depth > 1 {
			t.Errorf("row %+v exceeds maxDepth=1", r)
		}
	}
	var total int
	for _, r := range got {
		total += r.entry.cf.Folder.Unseen + r.aggUnread
	}
	if total < 6 {
		t.Errorf("want total >=6 reachable across visible rows, got %d", total)
	}
}
