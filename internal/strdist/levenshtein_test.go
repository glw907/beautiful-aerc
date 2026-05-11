package strdist

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		limit int
		want  int
	}{
		{"identity", "abc", "abc", 0, 0},
		{"empty a", "", "abc", 0, 3},
		{"empty b", "abc", "", 0, 3},
		{"both empty", "", "", 0, 0},
		{"classic kitten", "kitten", "sitting", 0, 3},
		{"jmap vs imap", "jmap", "imap", 0, 1},
		{"uncapped large", "abcdef", "xyzwvu", 0, 6},
		{"cap triggered by length skew", "a", "abcdefg", 2, 3},
		{"cap triggered mid-row", "abc", "xyz", 1, 2},
		{"within cap returns true distance", "abc", "abd", 2, 1},
		{"cap exactly at distance", "kitten", "sitting", 3, 3},
		{"negative limit disables cap", "abcdef", "ghijkl", -1, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Levenshtein(tt.a, tt.b, tt.limit)
			if got != tt.want {
				t.Errorf("Levenshtein(%q,%q,%d) = %d, want %d", tt.a, tt.b, tt.limit, got, tt.want)
			}
		})
	}
}
