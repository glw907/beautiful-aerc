package catkin

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// annotateDebounce is the idle delay after the last edit before
// annotators run. 350 ms keeps typing cheap and matches common
// IDE inline-lint timing.
const annotateDebounce = 350 * time.Millisecond

// annotateRequestMsg fires after the debounce. If gen still
// matches Model.srcGen, annotators run. Stale requests are dropped.
type annotateRequestMsg struct{ gen uint64 }

// annotationsReadyMsg carries a freshly-computed set. Model
// installs it iff gen still matches Model.srcGen.
type annotationsReadyMsg struct {
	gen uint64
	set *AnnotationSet
}

// scheduleAnnotateCmd issues a tick that fires
// annotateRequestMsg{gen} after annotateDebounce.
func scheduleAnnotateCmd(gen uint64) tea.Cmd {
	return tea.Tick(annotateDebounce, func(time.Time) tea.Msg {
		return annotateRequestMsg{gen: gen}
	})
}

// runAnnotatorsCmd runs every annotator over src and returns an
// annotationsReadyMsg{gen, set}. Pure compute, safe inside a Cmd.
func runAnnotatorsCmd(gen uint64, src string, annotators []Annotator) tea.Cmd {
	return func() tea.Msg {
		anns := runAnnotators(annotators, src)
		return annotationsReadyMsg{gen: gen, set: newAnnotationSet(src, anns)}
	}
}

// runAnnotators invokes each registered annotator and merges
// their output. Result is sorted by Range.Start.
func runAnnotators(annotators []Annotator, src string) []Annotation {
	var out []Annotation
	for _, a := range annotators {
		out = append(out, a.Annotate(src)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Range.Start < out[j].Range.Start
	})
	return out
}

// Range is a half-open byte-offset range over the raw source.
// Stored as byte offsets, not row/col, so annotators don't re-derive
// from a moving cursor. The renderer maps offsets to row/col once.
type Range struct{ Start, End int }

// Contains reports whether off lies in [Start, End).
func (r Range) Contains(off int) bool { return off >= r.Start && off < r.End }

// AnnotationKind identifies the producer category. Reserved
// future kinds (grammar, lint) sit beside KindMisspelling so
// composition rules can key off the kind.
type AnnotationKind int

const (
	KindMisspelling AnnotationKind = iota
)

// Annotation is one decoration over a source range.
type Annotation struct {
	Range   Range
	Kind    AnnotationKind
	Style   lipgloss.Style
	Payload any
}

// MisspellingPayload is the typed payload for KindMisspelling.
type MisspellingPayload struct {
	Word        string
	Suggestions []string // up to 5, frequency-ordered
}

// Annotator produces annotations over the full source. Implementations
// must be pure: no I/O, no goroutine kickoff. Heavy work runs on
// the idle tick path.
type Annotator interface {
	Name() string
	Annotate(src string) []Annotation
}

// AnnotationSet is the per-frame artifact rendering consults. The
// All slice is sorted by Range.Start.
type AnnotationSet struct {
	All       []Annotation
	byRow     []int // first index in All whose End reaches row r (-1 if none)
	rowStarts []int // byte offset of each row's first character
}

func newAnnotationSet(src string, anns []Annotation) *AnnotationSet {
	rowStarts := []int{0}
	for i := range src {
		if src[i] == '\n' {
			rowStarts = append(rowStarts, i+1)
		}
	}
	byRow := make([]int, len(rowStarts))
	for i := range byRow {
		byRow[i] = -1
	}
	// byRow[r] = first annotation index whose End reaches past
	// rowStarts[r]. Captures annotations starting on row r and
	// multi-row annotations that span into it from above.
	// Two-pointer: advance ai until we find an annotation that reaches
	// the current row, then stamp all subsequent rows it covers.
	ai := 0
	for r, rs := range rowStarts {
		// Skip annotations that end at or before this row's start.
		for ai < len(anns) && anns[ai].Range.End <= rs {
			ai++
		}
		if ai < len(anns) {
			byRow[r] = ai
		}
	}
	return &AnnotationSet{All: anns, byRow: byRow, rowStarts: rowStarts}
}

func (s *AnnotationSet) firstOnRow(row int) int {
	if s == nil || row < 0 || row >= len(s.byRow) {
		return -1
	}
	return s.byRow[row]
}

// rangesOnRow returns annotations that intersect row r. The src
// parameter is still accepted so callers (the renderer) can pass it
// unchanged. Its length determines the end of the final row. The
// newline walk is not repeated: rowStarts was computed once in
// newAnnotationSet.
func (s *AnnotationSet) rangesOnRow(src string, row int) []Annotation {
	if s == nil {
		return nil
	}
	if row < 0 || row >= len(s.rowStarts) {
		return nil
	}
	rowStart := s.rowStarts[row]
	rowEnd := len(src)
	if row+1 < len(s.rowStarts) {
		rowEnd = s.rowStarts[row+1] - 1 // exclude '\n'
	}
	start := s.firstOnRow(row)
	if start == -1 {
		return nil
	}
	var out []Annotation
	for _, a := range s.All[start:] {
		if a.Range.Start >= rowEnd {
			break
		}
		if a.Range.End <= rowStart {
			continue
		}
		out = append(out, a)
	}
	return out
}
