package tidy

import (
	"reflect"
	"testing"
)

func TestDiffRanges(t *testing.T) {
	cases := []struct {
		name    string
		oldText string
		newText string
		want    []ByteRange
	}{
		{"empty/empty", "", "", nil},
		{"identical", "hello world", "hello world", nil},
		{"empty old, all new", "", "hello", []ByteRange{{0, 5}}},
		{"single rune insertion", "hello world", "hello, world", []ByteRange{{5, 6}}},
		{"single rune deletion", "hello, world", "hello world", nil},
		{"contiguous run", "teh quick brown", "the quick brown", []ByteRange{{1, 3}}},
		{"two non-adjacent edits", "teh quick fox", "the quick foxes", []ByteRange{{1, 3}, {13, 15}}},
		{"multibyte rune change", "café", "cafe", []ByteRange{{3, 4}}},
		{"all-different short", "abc", "xyz", []ByteRange{{0, 3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DiffRanges(tc.oldText, tc.newText)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DiffRanges(%q, %q) = %v, want %v",
					tc.oldText, tc.newText, got, tc.want)
			}
		})
	}
}
