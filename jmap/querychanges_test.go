package jmap

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestQueryChangesOmitsANullFilter covers DV-03 on the /queryChanges
// pair. A Filter interface holding a typed nil is not nil to Go and is
// null on the wire, a form RFC 8620 never blesses and Stalwart
// rejected before v0.16.10.
func TestQueryChangesOmitsANullFilter(t *testing.T) {
	cases := []struct {
		name   string
		method Method
		want   string
	}{
		{
			name:   "mailbox",
			method: &MailboxQueryChanges{Account: "A1", Filter: (*MailboxFilterCondition)(nil)},
			want:   `{"accountId":"A1"}`,
		},
		{
			name:   "email",
			method: &EmailQueryChanges{Account: "A1", Filter: (*EmailFilterCondition)(nil)},
			want:   `{"accountId":"A1"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.method)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestQueryChangesResponseDecodes covers JT-17's response half,
// including the AddedItem index RFC 8620 section 5.6 splices on.
func TestQueryChangesResponseDecodes(t *testing.T) {
	const body = `{
	  "methodResponses": [
	    ["Email/queryChanges", {
	      "accountId": "A1",
	      "oldQueryState": "q1",
	      "newQueryState": "q2",
	      "total": 42,
	      "removed": ["id2", "id31"],
	      "added": [{"id": "id5", "index": 0}]
	    }, "0"]
	  ],
	  "sessionState": "s1"
	}`

	var resp Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	changes := argsOf[*EmailQueryChangesResponse](t, &resp, "0")

	if changes.OldQueryState != "q1" || changes.NewQueryState != "q2" {
		t.Errorf("query states = %q to %q, want q1 to q2", changes.OldQueryState, changes.NewQueryState)
	}
	if changes.Total != 42 {
		t.Errorf("Total = %d, want 42", changes.Total)
	}
	if want := []ID{"id2", "id31"}; !slices.Equal(changes.Removed, want) {
		t.Errorf("Removed = %v, want %v", changes.Removed, want)
	}
	if want := []AddedItem{{ID: "id5", Index: 0}}; !slices.Equal(changes.Added, want) {
		t.Errorf("Added = %v, want %v", changes.Added, want)
	}
}

// TestSpliceFollowsTheRFCWalkthrough covers JT-17's splice math
// against the worked example of RFC 8620 section 5.6, whose cached
// array is sparse. A hole is the empty id, which section 1.2's
// alphabet cannot spell, so it never collides with a real one.
func TestSpliceFollowsTheRFCWalkthrough(t *testing.T) {
	cached := []ID{"id1", "id2", "", "", "id3", "id4", "", "", ""}

	got, err := Splice(cached, []ID{"id2", "id31"}, []AddedItem{{ID: "id5", Index: 0}})
	if err != nil {
		t.Fatalf("Splice: %v", err)
	}
	want := []ID{"id5", "id1", "", "", "id3", "id4", "", "", ""}
	if !slices.Equal(got, want) {
		t.Errorf("Splice = %v, want %v", got, want)
	}
	if slices.Equal(cached, want) {
		t.Error("Splice wrote through to the caller's slice")
	}
}

// TestSpliceOrdersItsWork covers the ordering half of JT-17: an
// insertion shifts every later index, so applying the added array out
// of order lands ids in the wrong rows and the listing silently
// disagrees with the server.
func TestSpliceOrdersItsWork(t *testing.T) {
	cases := []struct {
		name    string
		cached  []ID
		removed []ID
		added   []AddedItem
		want    []ID
	}{
		{
			name:   "insertions apply lowest index first",
			cached: []ID{"a", "b"},
			added:  []AddedItem{{ID: "x", Index: 0}, {ID: "y", Index: 2}},
			want:   []ID{"x", "a", "y", "b"},
		},
		{
			name:   "an index at the end appends",
			cached: []ID{"a"},
			added:  []AddedItem{{ID: "z", Index: 1}},
			want:   []ID{"a", "z"},
		},
		{
			name:    "removals apply before insertions",
			cached:  []ID{"a", "b", "c"},
			removed: []ID{"a"},
			added:   []AddedItem{{ID: "x", Index: 2}},
			want:    []ID{"b", "c", "x"},
		},
		{
			name:    "a removal the cache never held is not an error",
			cached:  []ID{"a"},
			removed: []ID{"gone"},
			want:    []ID{"a"},
		},
		{
			name:    "every copy of a removed id goes",
			cached:  []ID{"a", "b", "a"},
			removed: []ID{"a"},
			want:    []ID{"b"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Splice(c.cached, c.removed, c.added)
			if err != nil {
				t.Fatalf("Splice: %v", err)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("Splice = %v, want %v", got, c.want)
			}
		})
	}
}

// TestSpliceRefusesAnUnusableChange covers JT-17's loudness
// requirement. An index the cache cannot reach means the client and
// the server disagree about the query, and appending anyway buries
// that disagreement in a listing that looks fine.
func TestSpliceRefusesAnUnusableChange(t *testing.T) {
	cases := []struct {
		name    string
		cached  []ID
		added   []AddedItem
		wantErr string
	}{
		{
			name:    "index past the end",
			cached:  []ID{"a"},
			added:   []AddedItem{{ID: "x", Index: 5}},
			wantErr: "index 5",
		},
		{
			name:    "index past the end after an earlier insertion",
			cached:  []ID{"a"},
			added:   []AddedItem{{ID: "x", Index: 0}, {ID: "y", Index: 9}},
			wantErr: "index 9",
		},
		{
			name:    "added out of index order",
			cached:  []ID{"a", "b", "c"},
			added:   []AddedItem{{ID: "x", Index: 2}, {ID: "y", Index: 1}},
			wantErr: "ascending",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Splice(c.cached, nil, c.added)
			if err == nil {
				t.Fatalf("Splice = %v, want an error", got)
			}
			if got != nil {
				t.Errorf("Splice returned %v alongside its error, want nil", got)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("Splice error = %q, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

// TestQueryChangesCannotCalculate proves RFC 8620 section 5.6's own
// error reaches a caller as the sentinel that forces a cache rebuild,
// rather than as an empty change set.
func TestQueryChangesCannotCalculate(t *testing.T) {
	const body = `{
	  "methodResponses": [
	    ["error", {"type": "cannotCalculateChanges"}, "0"]
	  ],
	  "sessionState": "s1"
	}`

	var resp Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	err := argsOf[*MethodError](t, &resp, "0")
	if !errors.Is(err, ErrCannotCalculateChanges) {
		t.Errorf("error = %v, want cannotCalculateChanges", err)
	}
}
