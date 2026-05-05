package catkin

import (
	"os"
	"reflect"
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
is
real
and
here
`

func TestSpellerCheckKnown(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	cases := []struct {
		w    string
		want bool
	}{
		{"the", true},
		{"The", true},   // case-insensitive
		{"BROWN", true}, // all caps known word
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

func TestSpellerSuggest(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	got := s.Suggest("tradeof", 5)
	if len(got) == 0 || got[0] != "tradeoff" {
		t.Errorf("Suggest(tradeof) = %v, want first suggestion \"tradeoff\"", got)
	}
}

func TestSpellerSuggestRespectsLimit(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	got := s.Suggest("brwn", 3) // close to "brown"
	if len(got) > 3 {
		t.Errorf("Suggest returned %d, want ≤ 3", len(got))
	}
}

func TestSpellerSuggestFrequencyOrder(t *testing.T) {
	// Both "the" and "tea" are within edit distance 2 of "tee".
	// "the" has lower rank (more frequent) so should sort first.
	s, err := newSpellerFromReader(strings.NewReader("the\ntea\n"), nil, nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	got := s.Suggest("tee", 5)
	if len(got) < 2 || got[0] != "the" {
		t.Errorf("Suggest(tee) = %v, want \"the\" first by frequency", got)
	}
}

func TestLoadUserWordlistMissing(t *testing.T) {
	got, err := LoadUserWordlist(t.TempDir() + "/does-not-exist.txt")
	if err != nil {
		t.Errorf("missing file should not be an error; got %v", err)
	}
	if got != nil {
		t.Errorf("missing file should return nil; got %v", got)
	}
}

func TestLoadUserWordlistParses(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/wordlist.txt"
	body := "# comment\nfrobnicate\n\nQuux\n   spaced   \n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadUserWordlist(path)
	if err != nil {
		t.Fatalf("LoadUserWordlist: %v", err)
	}
	want := []string{"frobnicate", "Quux", "spaced"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadUserWordlist = %v, want %v", got, want)
	}
}

func TestSpellcheckAnnotator(t *testing.T) {
	speller := newFixtureSpeller(t, nil)
	a := NewSpellcheckAnnotator(speller)

	cases := []struct {
		name       string
		src        string
		wantRanges []Range
	}{
		{
			name:       "single misspelling",
			src:        "the tradeof is real",
			wantRanges: []Range{{4, 11}},
		},
		{
			name:       "no misspellings",
			src:        "the quick brown fox",
			wantRanges: nil,
		},
		{
			name:       "skip inside fenced code",
			src:        "the\n```go\nquxxxxxx\n```\nover",
			wantRanges: nil,
		},
		{
			name:       "skip inside inline code",
			src:        "the `quxxxxxx` over",
			wantRanges: nil,
		},
		{
			name:       "skip inside link URL",
			src:        "the [quick](http://exampole.com/foo) over",
			wantRanges: nil,
		},
		{
			name:       "skip all-caps acronym <=4 runes",
			src:        "JMAP and HTTP and IMAP",
			wantRanges: nil,
		},
		{
			name:       "all-caps >4 still checked",
			src:        "QUXXXXX",
			wantRanges: []Range{{0, 7}},
		},
		{
			// Regression: buildSkipMask multi-span offset tracking.
			// Line has inline-code, a misspelled word, then a link.
			// The misspelling must be flagged at its true byte offset.
			// The link URL must not produce a false positive.
			name: "multi-span: code + misspelling + link",
			src:  "`fox` tradeof and [dog](http://exampole.com)",
			// "tradeof" starts at byte 6 (after "`fox` ").
			wantRanges: []Range{{6, 13}},
		},
		{
			// Regression: misspelling before first span must survive
			// when a later span is also present.
			name:       "multi-span: misspelling + code",
			src:        "tradeof and `fox`",
			wantRanges: []Range{{0, 7}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := a.Annotate(c.src)
			if len(got) != len(c.wantRanges) {
				t.Fatalf("Annotate(%q) returned %d ranges, want %d: %v", c.src, len(got), len(c.wantRanges), got)
			}
			for i, want := range c.wantRanges {
				if got[i].Range != want {
					t.Errorf("range %d = %v, want %v", i, got[i].Range, want)
				}
				if got[i].Kind != KindMisspelling {
					t.Errorf("range %d kind = %v, want KindMisspelling", i, got[i].Kind)
				}
			}
		})
	}
}

func TestSpellcheckAnnotatorPayload(t *testing.T) {
	speller := newFixtureSpeller(t, nil)
	a := NewSpellcheckAnnotator(speller)
	got := a.Annotate("tradeof here")
	if len(got) != 1 {
		t.Fatalf("Annotate returned %d, want 1", len(got))
	}
	mp, ok := got[0].Payload.(MisspellingPayload)
	if !ok {
		t.Fatalf("Payload type = %T, want MisspellingPayload", got[0].Payload)
	}
	if mp.Word != "tradeof" {
		t.Errorf("Word = %q, want \"tradeof\"", mp.Word)
	}
	if len(mp.Suggestions) == 0 || mp.Suggestions[0] != "tradeoff" {
		t.Errorf("Suggestions = %v, want first \"tradeoff\"", mp.Suggestions)
	}
}
