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
	// Source: "abc\ndef\nghi". Newlines at offsets 3 and 7.
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

type fakeAnnotator struct {
	name  string
	calls int
	out   []Annotation
}

func (f *fakeAnnotator) Name() string { return f.name }
func (f *fakeAnnotator) Annotate(src string) []Annotation {
	f.calls++
	return f.out
}

func TestRegisterAnnotator(t *testing.T) {
	m := New()
	a := &fakeAnnotator{name: "fake", out: []Annotation{{Range: Range{0, 3}, Kind: KindMisspelling}}}
	m.RegisterAnnotator(a)
	set := runAnnotators(m.annotators, "abc def")
	if len(set) != 1 || set[0].Range.Start != 0 {
		t.Fatalf("runAnnotators output = %#v, want one annotation at 0", set)
	}
	if a.calls != 1 {
		t.Errorf("Annotate calls = %d, want 1", a.calls)
	}
}

func TestAnnotateStaleDrop(t *testing.T) {
	m := New()
	m.srcGen = 5
	// Ready msg from generation 4 should be ignored.
	m, _ = m.Update(annotationsReadyMsg{gen: 4, set: &AnnotationSet{}})
	if m.annotations != nil {
		t.Errorf("stale annotationsReadyMsg should not install a set")
	}
	// Matching gen installs.
	want := &AnnotationSet{}
	m, _ = m.Update(annotationsReadyMsg{gen: 5, set: want})
	if m.annotations != want {
		t.Errorf("matching gen should install the set")
	}
}

func TestAnnotateRequestStaleDrop(t *testing.T) {
	m := New()
	a := &fakeAnnotator{name: "fake"}
	m.RegisterAnnotator(a)
	m.srcGen = 7
	// Request from gen 6 should not run annotators.
	m, _ = m.Update(annotateRequestMsg{gen: 6})
	if a.calls != 0 {
		t.Errorf("stale request should not invoke annotators; calls = %d", a.calls)
	}
}
