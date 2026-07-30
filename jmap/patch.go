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

// MarshalJSON implements json.Marshaler. It reports the RFC 8620
// section 5.3 pointer restrictions it can decide from the pointers
// alone rather than sending a patch that breaks them: a strict server
// answers invalidPatch, and a lenient one may apply an overlapping
// pair in an order the caller did not intend.
func (p Patch) MarshalJSON() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any(p))
}

// validate reports the two restrictions the pointers decide on their
// own: no empty segment, and no pointer that is the prefix of another.
// Every JMAP property is named and every id is at least one octet (RFC
// 8620 section 1.2), so an empty segment addresses nothing on any
// record and is a caller passing a value it never filled in.
//
// Section 5.3 also forbids a pointer that reaches into an array, and
// that one is not decidable here. RFC 6901 section 4 reads a segment
// as an array index only where the value it is applied to is an array,
// and as a member name everywhere else; a Patch travels without the
// record it patches, so the shape of a segment says nothing about
// which it is. Refusing the shape refuses "mailboxIds/7", which is a
// message filed into the mailbox whose id is "7" and which a
// conformant server applies. Section 1.2 recommends servers avoid ids
// made only of digits and calls the recommendation optional, so those
// ids are real: Stalwart hands one out after about twenty mailboxes,
// and a client that refuses them cannot move mail into them at all.
// The pointer that really does reach into an array is the server's to
// refuse, which section 5.3 makes it do with invalidPatch.
func (p Patch) validate() error {
	pointers := slices.Sorted(maps.Keys(p))
	for _, pointer := range pointers {
		for segment := range strings.SplitSeq(pointer, "/") {
			if segment == "" {
				return fmt.Errorf("patch pointer %q has an empty segment", pointer)
			}
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
