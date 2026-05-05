package catkin

import (
	"strings"
	"testing"
)

const fixtureExtra = "" // overridden in user-overlay test

func newFixtureSpeller(t *testing.T, extra []string) *Speller {
	t.Helper()
	s, err := newSpellerFromReader(strings.NewReader(fixtureWordlist), nil, extra)
	if err != nil {
		t.Fatalf("newSpellerFromReader: %v", err)
	}
	return s
}

const fixtureWordlist = `the
quick
brown
fox
jumps
over
lazy
dog
tradeoff
markdown
`

func TestSpellerCheckKnown(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	cases := []struct {
		w    string
		want bool
	}{
		{"the", true},
		{"The", true},    // case-insensitive
		{"BROWN", true},  // all caps known word
		{"tradeof", false},
		{"markdwn", false},
		{"", false}, // empty is not a word
	}
	for _, c := range cases {
		if got := s.Check(c.w); got != c.want {
			t.Errorf("Check(%q) = %v, want %v", c.w, got, c.want)
		}
	}
}

func TestSpellerExtraWords(t *testing.T) {
	s := newFixtureSpeller(t, []string{"frobnicate", "quux"})
	if !s.Check("frobnicate") {
		t.Errorf("extra word frobnicate should pass Check")
	}
	if !s.Check("Quux") {
		t.Errorf("extra word Quux should pass Check (case-insensitive)")
	}
}
