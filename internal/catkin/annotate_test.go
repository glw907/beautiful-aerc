package catkin

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRangeContains(t *testing.T) {
	r := Range{Start: 3, End: 7}
	cases := []struct {
		off  int
		want bool
	}{
		{2, false},
		{3, true},
		{6, true},
		{7, false}, // half-open
	}
	for _, c := range cases {
		if got := r.Contains(c.off); got != c.want {
			t.Errorf("Range{3,7}.Contains(%d) = %v, want %v", c.off, got, c.want)
		}
	}
}

func TestAnnotationSetByRow(t *testing.T) {
	// Source: "abc\ndef\nghi" — newlines at offsets 3 and 7.
	// Annotations on row 0 (Start=0), row 1 (Start=4), row 2 (Start=8).
	src := "abc\ndef\nghi"
	anns := []Annotation{
		{Range: Range{0, 3}, Kind: KindMisspelling},
		{Range: Range{4, 7}, Kind: KindMisspelling},
		{Range: Range{8, 11}, Kind: KindMisspelling},
	}
	set := newAnnotationSet(src, anns)
	if got := set.firstOnRow(0); got != 0 {
		t.Errorf("firstOnRow(0) = %d, want 0", got)
	}
	if got := set.firstOnRow(1); got != 1 {
		t.Errorf("firstOnRow(1) = %d, want 1", got)
	}
	if got := set.firstOnRow(2); got != 2 {
		t.Errorf("firstOnRow(2) = %d, want 2", got)
	}
	if got := set.firstOnRow(99); got != -1 {
		t.Errorf("firstOnRow(99) = %d, want -1", got)
	}
}

func TestAnnotationCarriesStyleAndPayload(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true)
	a := Annotation{
		Range:   Range{0, 3},
		Kind:    KindMisspelling,
		Style:   style,
		Payload: MisspellingPayload{Word: "abc", Suggestions: []string{"abd"}},
	}
	if !a.Style.GetUnderline() {
		t.Errorf("Style not preserved: underline lost")
	}
	mp, ok := a.Payload.(MisspellingPayload)
	if !ok || mp.Word != "abc" {
		t.Errorf("Payload = %#v, want MisspellingPayload{Word:\"abc\"}", a.Payload)
	}
}
