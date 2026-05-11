package catkin

import (
	"testing"

	"charm.land/lipgloss/v2"
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
	src := "abc\ndef\nghi"
	anns := []Annotation{
		{Range: Range{0, 3}, Kind: KindMisspelling},
		{Range: Range{4, 7}, Kind: KindMisspelling},
		{Range: Range{8, 11}, Kind: KindMisspelling},
	}
	set := newAnnotationSet(src, anns)
	cases := []struct {
		row, want int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{99, -1},
	}
	for _, c := range cases {
		if got := set.firstOnRow(c.row); got != c.want {
			t.Errorf("firstOnRow(%d) = %d, want %d", c.row, got, c.want)
		}
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
	calls int
	out   []Annotation
}

func (f *fakeAnnotator) Annotate(_ string) []Annotation {
	f.calls++
	return f.out
}

func TestRegisterAnnotator(t *testing.T) {
	m := New()
	a := &fakeAnnotator{out: []Annotation{{Range: Range{0, 3}, Kind: KindMisspelling}}}
	m = m.WithAnnotator(a)
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
	a := &fakeAnnotator{}
	m = m.WithAnnotator(a)
	m.srcGen = 7
	// Request from gen 6 should not run annotators.
	m, _ = m.Update(annotateRequestMsg{gen: 6})
	if a.calls != 0 {
		t.Errorf("stale request should not invoke annotators; calls = %d", a.calls)
	}
}

func TestTidyAnnotator(t *testing.T) {
	a := newTidyAnnotator()
	style := lipgloss.NewStyle().Underline(true)
	a.SetStyle(style)

	// No state set, so no annotations.
	if got := a.Annotate("anything"); len(got) != 0 {
		t.Errorf("empty state: got %d annotations, want 0", len(got))
	}

	a.Set("hello world", []Range{{0, 5}, {6, 11}})

	// Matching src returns the stored ranges with configured style.
	got := a.Annotate("hello world")
	if len(got) != 2 {
		t.Fatalf("matching src: got %d, want 2", len(got))
	}
	if got[0].Range != (Range{0, 5}) || got[1].Range != (Range{6, 11}) {
		t.Errorf("ranges = %v, want [{0,5},{6,11}]", got)
	}
	if got[0].Kind != KindTidyChange {
		t.Errorf("kind = %v, want KindTidyChange", got[0].Kind)
	}
	if got[0].Style.GetUnderline() != true {
		t.Errorf("style not propagated: GetUnderline() = false")
	}

	// Any divergence returns empty.
	if got := a.Annotate("hello world!"); len(got) != 0 {
		t.Errorf("divergent src: got %d, want 0", len(got))
	}
	if got := a.Annotate("hello"); len(got) != 0 {
		t.Errorf("shorter src: got %d, want 0", len(got))
	}
}

func TestTidyAnnotatorClearedByMutation(t *testing.T) {
	m := New()
	m = m.WithValue("teh quick brown")
	m = m.WithTidyHighlights("the quick brown", []Range{{1, 3}})
	if got := runAnnotators(m.annotators, "the quick brown"); len(got) == 0 {
		t.Fatalf("post-set: want at least one tidy annotation")
	}
	if got := runAnnotators(m.annotators, "the quick brown!"); len(got) != 0 {
		t.Errorf("post-mutation: got %d, want 0", len(got))
	}
}
