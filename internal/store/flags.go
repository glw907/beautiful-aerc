package store

import "slices"

// Flags is message.flags: the query-relevant subset of a message's
// keywords, packed as a bitfield so an is: filter or a sort never
// reaches into message.data (ADR-0002's fat-table rule). A keyword
// with no named bit still round-trips: EncodeFlags returns it as
// overflow, which the caller carries in message.data, and
// DecodeFlags folds it back into the full keyword set.
type Flags uint32

// Named bits cover the keywords SR-2's is: grammar and LT-2's
// triage verbs touch. Every other JMAP keyword ($junk, $phishing,
// a custom label) has no bit and travels only as overflow.
const (
	FlagSeen Flags = 1 << iota
	FlagAnswered
	FlagFlagged
	FlagDraft
	FlagForwarded
)

var namedFlag = map[string]Flags{
	"$seen":      FlagSeen,
	"$answered":  FlagAnswered,
	"$flagged":   FlagFlagged,
	"$draft":     FlagDraft,
	"$forwarded": FlagForwarded,
}

var flagKeyword = map[Flags]string{
	FlagSeen:      "$seen",
	FlagAnswered:  "$answered",
	FlagFlagged:   "$flagged",
	FlagDraft:     "$draft",
	FlagForwarded: "$forwarded",
}

// EncodeFlags splits keywords into message.flags's bitfield and the
// keywords with no named bit, which the caller stores as message.data
// overflow.
func EncodeFlags(keywords []string) (bits Flags, overflow []string) {
	for _, kw := range keywords {
		if bit, ok := namedFlag[kw]; ok {
			bits |= bit
			continue
		}
		overflow = append(overflow, kw)
	}
	return bits, overflow
}

// DecodeFlags reconstructs a message's full keyword set from
// message.flags and its message.data overflow.
func DecodeFlags(bits Flags, overflow []string) []string {
	keywords := slices.Clone(overflow)
	for bit, kw := range flagKeyword {
		if bits&bit != 0 {
			keywords = append(keywords, kw)
		}
	}
	return keywords
}
