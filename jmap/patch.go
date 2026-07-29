package jmap

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// A Patch is an RFC 8620 section 5.3 PatchObject: a map from a JSON
// pointer, relative to the record being updated, to the value that
// pointer takes. A nil value removes the property or restores its
// default.
//
// A key naming a property replaces that whole property. A key built
// by [Pointer] patches one leaf and leaves the rest alone. Marking one
// message read is Pointer("keywords", "$seen") mapped to true; the
// bare "keywords" mapped to a map replaces the keyword set, and the
// bare "mailboxIds" mapped to an empty map files a message out of
// every mailbox, which hides it.
type Patch map[string]any

// MarshalJSON implements json.Marshaler. It reports the two RFC 8620
// section 5.3 pointer restrictions rather than sending a patch that
// breaks them: a strict server answers invalidPatch, and a lenient
// one may apply an overlapping pair in an order the caller did not
// intend.
func (p Patch) MarshalJSON() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any(p))
}

func (p Patch) validate() error {
	pointers := slices.Sorted(maps.Keys(p))
	for _, pointer := range pointers {
		if slices.ContainsFunc(strings.Split(pointer, "/"), isArrayToken) {
			return fmt.Errorf("patch pointer %q addresses an array element; replace the whole property instead", pointer)
		}
	}
	for _, outer := range pointers {
		for _, inner := range pointers {
			if strings.HasPrefix(inner, outer+"/") {
				return fmt.Errorf("patch pointers %q and %q overlap; one is a prefix of the other", outer, inner)
			}
		}
	}
	return nil
}

// isArrayToken reports whether segment is one of RFC 6901's array
// tokens: a non-negative decimal index, or "-" for the position past
// the end.
//
// RFC 8620 section 5.3 forbids a pointer that reaches into an array,
// and whether a property holds an array is a fact about the record
// schema that a patch alone does not carry. Rejecting the token shape
// over-rejects a map key that looks like an index, which the RFC's own
// id guidance (section 1.2) tells servers not to allocate. The trade
// is deliberate: the over-rejection is an error at the boundary, while
// the alternative is a silently destructive update.
func isArrayToken(segment string) bool {
	if segment == "-" {
		return true
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Pointer builds the RFC 6901 pointer that addresses one leaf under a
// property, escaping "~" and "/" within each segment. Moving a message
// between mailboxes is Pointer("mailboxIds", from) mapped to nil and
// Pointer("mailboxIds", to) mapped to true, which leaves every other
// membership untouched.
func Pointer(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, segment := range segments {
		escaped[i] = pointerEscaper.Replace(segment)
	}
	return strings.Join(escaped, "/")
}

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")
