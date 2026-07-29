package jmap

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPatchLeaf covers JT-02 and JT-04. A keyword change and a
// mailbox move each touch one leaf, and neither ever emits the parent
// property, which would clear the whole set. The explicit-null case
// is carried from go-jmap's mail/mailbox/set_test.go, the one proof
// anywhere that a nil in a patch survives Go marshaling rather than
// being dropped by omitempty.
func TestPatchLeaf(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  string
	}{
		{
			name:  "set a keyword",
			patch: Patch{Pointer("keywords", "$seen"): true},
			want:  `{"keywords/$seen":true}`,
		},
		{
			name:  "clear a keyword",
			patch: Patch{Pointer("keywords", "$seen"): nil},
			want:  `{"keywords/$seen":null}`,
		},
		{
			name: "move between mailboxes",
			patch: Patch{
				Pointer("mailboxIds", "MA"): nil,
				Pointer("mailboxIds", "MB"): true,
			},
			want: `{"mailboxIds/MA":null,"mailboxIds/MB":true}`,
		},
		{
			name:  "clear a top-level property",
			patch: Patch{"parentId": nil},
			want:  `{"parentId":null}`,
		},
		{
			name:  "escape a slash in a key",
			patch: Patch{Pointer("keywords", "a/b"): true},
			want:  `{"keywords/a~1b":true}`,
		},
		{
			name:  "escape a tilde in a key",
			patch: Patch{Pointer("keywords", "a~b"): true},
			want:  `{"keywords/a~0b":true}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.patch)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestPatchWholePropertyIsSeparate covers JT-04's second half.
// Replacing a property is available, and it reads differently from a
// leaf patch at the call site.
func TestPatchWholePropertyIsSeparate(t *testing.T) {
	leaf := Patch{Pointer("mailboxIds", "MB"): true}
	whole := Patch{"mailboxIds": map[ID]bool{"MB": true}}

	leafData, err := json.Marshal(leaf)
	if err != nil {
		t.Fatalf("Marshal leaf: %v", err)
	}
	wholeData, err := json.Marshal(whole)
	if err != nil {
		t.Fatalf("Marshal whole: %v", err)
	}

	if want := `{"mailboxIds/MB":true}`; string(leafData) != want {
		t.Errorf("leaf = %s, want %s", leafData, want)
	}
	if want := `{"mailboxIds":{"MB":true}}`; string(wholeData) != want {
		t.Errorf("whole = %s, want %s", wholeData, want)
	}
}

// TestPatchRefusesIllegalPointer covers JT-03. Neither form reaches
// the wire: marshaling fails, so a caller cannot send a patch a
// server would answer invalidPatch to, or that a lenient server would
// apply in an order the caller did not intend.
func TestPatchRefusesIllegalPointer(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  string
	}{
		{
			name:  "array index",
			patch: Patch{"mailboxIds/0": true},
			want:  "array element",
		},
		{
			name:  "array append token",
			patch: Patch{"attachments/-": true},
			want:  "array element",
		},
		{
			name:  "nested array index",
			patch: Patch{"bodyStructure/subParts/1/name": "a.txt"},
			want:  "array element",
		},
		{
			name: "property and its own leaf",
			patch: Patch{
				"keywords":                   map[string]bool{"$flagged": true},
				Pointer("keywords", "$seen"): nil,
			},
			want: "overlap",
		},
		{
			name: "prefix pair separated by a third key",
			patch: Patch{
				"keywords":                   map[string]bool{"$flagged": true},
				"keywords!":                  true,
				Pointer("keywords", "$seen"): nil,
			},
			want: "overlap",
		},
		{
			// Both empty forms are reachable: a bare Patch{"": v}, and
			// Pointer with a trailing empty segment. Neither addresses
			// a property of any record, so neither reaches the wire.
			name:  "an empty pointer",
			patch: Patch{"": true},
			want:  "empty segment",
		},
		{
			name:  "a trailing empty segment",
			patch: Patch{"keywords/": true},
			want:  "empty segment",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.patch)
			if err == nil {
				t.Fatalf("Marshal returned %s, want an error", data)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("Marshal error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

// TestPatchRefusalReachesTheRequest proves the refusal is not
// something a caller sidesteps by marshaling a whole request: an
// illegal patch nested inside an EmailSet fails the request too.
func TestPatchRefusalReachesTheRequest(t *testing.T) {
	var req Request
	req.Invoke(&EmailSet{
		Account: "A13824",
		Update: map[ID]Patch{
			"M1": {"mailboxIds/0": true},
		},
	})

	if data, err := json.Marshal(req); err == nil {
		t.Fatalf("Marshal returned %s, want an error", data)
	}
}

// TestPatchAllowsAKeyThatOnlyLooksNested proves the prefix rule does
// not fire on two sibling leaves, which is the common case.
func TestPatchAllowsAKeyThatOnlyLooksNested(t *testing.T) {
	patch := Patch{
		Pointer("keywords", "$seen"):    true,
		Pointer("keywords", "$flagged"): nil,
	}
	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"keywords/$flagged":null,"keywords/$seen":true}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
}
