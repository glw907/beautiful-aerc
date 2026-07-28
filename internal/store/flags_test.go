package store

import (
	"slices"
	"testing"
)

func TestFlagsRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		keywords     []string
		wantBits     Flags
		wantOverflow []string
	}{
		{"seen", []string{"$seen"}, FlagSeen, nil},
		{"answered", []string{"$answered"}, FlagAnswered, nil},
		{"flagged", []string{"$flagged"}, FlagFlagged, nil},
		{"draft", []string{"$draft"}, FlagDraft, nil},
		{"forwarded", []string{"$forwarded"}, FlagForwarded, nil},
		{
			"named bits plus overflow",
			[]string{"$seen", "$flagged", "$junk", "team-red"},
			FlagSeen | FlagFlagged,
			[]string{"$junk", "team-red"},
		},
		{"no keywords", nil, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bits, overflow := EncodeFlags(tt.keywords)
			if bits != tt.wantBits {
				t.Errorf("EncodeFlags(%v) bits = %#b, want %#b", tt.keywords, bits, tt.wantBits)
			}
			slices.Sort(overflow)
			wantOverflow := slices.Clone(tt.wantOverflow)
			slices.Sort(wantOverflow)
			if !slices.Equal(overflow, wantOverflow) {
				t.Errorf("EncodeFlags(%v) overflow = %v, want %v", tt.keywords, overflow, wantOverflow)
			}

			got := DecodeFlags(bits, overflow)
			slices.Sort(got)
			want := slices.Clone(tt.keywords)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("DecodeFlags(%#b, %v) = %v, want %v", bits, overflow, got, want)
			}
		})
	}
}
