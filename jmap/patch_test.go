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
// lenient server would apply in an order the caller did not intend,
// or one that addresses no property of any record.
func TestPatchRefusesIllegalPointer(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  string
	}{
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
			"M1": {
				"keywords":                   map[string]bool{"$flagged": true},
				Pointer("keywords", "$seen"): nil,
			},
		},
	})

	if data, err := json.Marshal(req); err == nil {
		t.Fatalf("Marshal returned %s, want an error", data)
	}
}

// TestPatchCarriesAnIDMadeOfDigits is the other half of JT-03: what
// the refusal must not swallow. RFC 6901 section 4 makes a segment an
// array index only where the value under it is an array, and
// mailboxIds is an Id[Boolean] object (RFC 8621 section 4.1), so
// "mailboxIds/7" names the mailbox whose id is "7". RFC 8620 section
// 1.2 only recommends that servers avoid those ids, and Stalwart
// allocates them, so a client that refused this pointer could not
// move a message into such a mailbox at all.
func TestPatchCarriesAnIDMadeOfDigits(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  string
	}{
		{
			name: "a move into a mailbox whose id is all digits",
			patch: Patch{
				Pointer("mailboxIds", "MA"): nil,
				Pointer("mailboxIds", "7"):  true,
			},
			want: `{"mailboxIds/7":true,"mailboxIds/MA":null}`,
		},
		{
			name:  "an id that is one digit",
			patch: Patch{Pointer("mailboxIds", "0"): true},
			want:  `{"mailboxIds/0":true}`,
		},
		{
			name:  "an id that is a single hyphen",
			patch: Patch{Pointer("mailboxIds", "-"): true},
			want:  `{"mailboxIds/-":true}`,
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
