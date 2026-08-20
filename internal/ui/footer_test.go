package ui

import (
	"image"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// footerFixture is a ScreenEntry whose FooterPriority carries three
// hints of increasing individual width:
//
//	"j/k navigate"       12 columns
//	"enter open"         10 columns
//	"a archive message"  17 columns
//
// footerHelpHint ("? help") is 6 columns and theme.GapHint is 3, so
// footerHints(entry, width) reserves 9 columns before it ever spends
// any on a priority hint. The boundary widths below are computed by
// hand from these fixed numbers, not from calling ansi.StringWidth
// on the fixture strings at test time, so a bug in footerHints's own
// arithmetic cannot cancel out against the same bug in the test.
func footerFixture() ScreenEntry {
	navigate := bind("j", "j/k", "navigate")
	openMsg := bind("enter", "enter", "open")
	archive := bind("a", "a", "archive message")

	return ScreenEntry{
		Type:           reflect.TypeOf(struct{}{}),
		FooterPriority: []key.Binding{navigate, openMsg, archive},
		Keys:           flatKeyMap(navigate, openMsg, archive, bind("q", "q", "quit")),
	}
}

func TestFooterHints_LiteralWidthBoundaries(t *testing.T) {
	entry := footerFixture()

	tests := []struct {
		name  string
		width int
		want  int // number of priority hints expected
	}{
		{"width 0: nothing fits", 0, 0},
		{"width 12: the reserve is load-bearing (hint 1 alone is 12 wide)", 12, 0},
		{"width 20: one column short of hint 1's boundary", 20, 0},
		{"width 21: hint 1 exactly fits (12 + 6 help + 3 gap)", 21, 1},
		{"width 33: one column short of hint 2's boundary", 33, 1},
		{"width 34: hint 2 exactly fits (12 + 3 + 10 + 6 + 3)", 34, 2},
		{"width 53: one column short of hint 3's boundary", 53, 2},
		{"width 54: hint 3 exactly fits (12 + 3 + 10 + 3 + 17 + 6 + 3)", 54, 3},
		{"width 200: every hint fits with room to spare", 200, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := footerHints(entry, tt.width)
			if len(got) != tt.want {
				t.Errorf("footerHints(%d) returned %d hints, want %d", tt.width, len(got), tt.want)
			}
		})
	}
}

func TestFooterHints_IsAPrefixOfFooterPriority(t *testing.T) {
	entry := footerFixture()

	got := footerHints(entry, 200)
	for i, b := range got {
		if !bindingsEqual(b, entry.FooterPriority[i]) {
			t.Fatalf("footerHints()[%d] = %v, want FooterPriority[%d] = %v", i, b, i, entry.FooterPriority[i])
		}
	}
}

func TestFooterHints_SkipsDisabledHints(t *testing.T) {
	navigate := bind("j", "j/k", "navigate")
	disabled := key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive"), key.WithDisabled())

	entry := ScreenEntry{
		Type:           reflect.TypeOf(struct{}{}),
		FooterPriority: []key.Binding{navigate, disabled},
		Keys:           flatKeyMap(navigate),
	}

	got := footerHints(entry, 200)
	if len(got) != 1 || !bindingsEqual(got[0], navigate) {
		t.Errorf("footerHints() with a disabled priority hint = %v, want only navigate", got)
	}
}

func TestFooterHints_EveryRenderedHintIsLegal(t *testing.T) {
	entry := footerFixture()
	legal := flattenKeys(entry.Keys)

	for _, width := range []int{10, 20, 40, 80, 160} {
		for _, b := range footerHints(entry, width) {
			if !slices.ContainsFunc(legal, func(l key.Binding) bool { return bindingsEqual(l, b) }) {
				t.Errorf("footerHints(%d) rendered %v, not present in the screen's own keymap", width, b)
			}
		}
	}
}

func TestFooterHints_MonotoneGrowthWithWidth(t *testing.T) {
	entry := footerFixture()

	prevLen := -1
	var prev []key.Binding
	for width := 0; width <= 200; width += 5 {
		got := footerHints(entry, width)
		if len(got) < prevLen {
			t.Fatalf("footerHints(%d) has %d hints, fewer than width %d's %d", width, len(got), width-5, prevLen)
		}
		if !slices.EqualFunc(prev, got[:min(len(prev), len(got))], bindingsEqual) {
			t.Fatalf("footerHints(%d) = %v is not an extension of the narrower width's %v", width, got, prev)
		}
		prevLen, prev = len(got), got
	}
}

func TestHelpContent_EqualsFullKeymap(t *testing.T) {
	entry := footerFixture()

	got := helpContent(entry)
	want := flattenKeys(entry.Keys)
	if !slices.EqualFunc(got, want, bindingsEqual) {
		t.Errorf("helpContent() = %v, want the full keymap %v", got, want)
	}

	// The footer's width-limited prefix is always a subset of help
	// content, never a superset: help is the completeness surface.
	for _, width := range []int{0, 10, 30, 200} {
		for _, b := range footerHints(entry, width) {
			if !slices.ContainsFunc(got, func(h key.Binding) bool { return bindingsEqual(h, b) }) {
				t.Errorf("footer hint %v at width %d is absent from helpContent()", b, width)
			}
		}
	}
}

func TestFooterPriorityWithinKeymap(t *testing.T) {
	inKeymap := bind("j", "j/k", "navigate")
	notInKeymap := bind("a", "a", "archive")

	tests := []struct {
		name    string
		entry   ScreenEntry
		wantLen int
	}{
		{
			name: "every priority hint present in the keymap passes",
			entry: ScreenEntry{
				Type:           reflect.TypeOf(struct{}{}),
				FooterPriority: []key.Binding{inKeymap},
				Keys:           flatKeyMap(inKeymap),
			},
			wantLen: 0,
		},
		{
			name: "a priority hint absent from the keymap fails",
			entry: ScreenEntry{
				Type:           reflect.TypeOf(struct{}{}),
				FooterPriority: []key.Binding{notInKeymap},
				Keys:           flatKeyMap(inKeymap),
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := footerPriorityWithinKeymap([]ScreenEntry{tt.entry}); len(got) != tt.wantLen {
				t.Errorf("footerPriorityWithinKeymap() = %v, want %d violation(s)", got, tt.wantLen)
			}
		})
	}
}

// TestFooterPriorityWithinKeymap_LiveRegistry is CR1's live check.
func TestFooterPriorityWithinKeymap_LiveRegistry(t *testing.T) {
	if got := footerPriorityWithinKeymap(Registered()); len(got) != 0 {
		t.Errorf("footerPriorityWithinKeymap(Registered()) = %v, want none", got)
	}
}

// TestFooterHints_LiveRegistry_EveryRenderedHintIsLegal and its
// sibling below are UX-2 revision 6's own "only mechanical claims"
// (design language section 4), run against Registered() rather than
// footerFixture: the footer is always a legal prefix of each
// registered screen's own committed order.
func TestFooterHints_LiveRegistry_EveryRenderedHintIsLegal(t *testing.T) {
	for _, e := range Registered() {
		legal := flattenKeys(e.Keys)
		for _, width := range []int{0, 20, 60, 80, 120, 200} {
			for _, b := range footerHints(e, width) {
				if !slices.ContainsFunc(legal, func(l key.Binding) bool { return bindingsEqual(l, b) }) {
					t.Errorf("%s: footerHints(%d) rendered %v, not present in its own keymap", e.Type, width, b)
				}
			}
		}
	}
}

func TestFooterHints_LiveRegistry_IsAPrefixOfFooterPriority(t *testing.T) {
	for _, e := range Registered() {
		got := footerHints(e, 200)
		for i, b := range got {
			if !bindingsEqual(b, e.FooterPriority[i]) {
				t.Errorf("%s: footerHints()[%d] = %v, want FooterPriority[%d] = %v", e.Type, i, b, i, e.FooterPriority[i])
			}
		}
	}
}

// TestHelpContent_EqualsFullKeymap_LiveRegistry is UX-2 revision 6's
// other mechanical claim: the help overlay's content is always the
// registered screen's complete keymap, regardless of what its
// footer's width-limited prefix shows.
func TestHelpContent_EqualsFullKeymap_LiveRegistry(t *testing.T) {
	for _, e := range Registered() {
		got, want := helpContent(e), flattenKeys(e.Keys)
		if !slices.EqualFunc(got, want, bindingsEqual) {
			t.Errorf("%s: helpContent() = %v, want the full keymap %v", e.Type, got, want)
		}
	}
}

// TestFooterHelpHint_DerivesFromGrammarKeysHelp pins BACKLOG #62's
// defect class against the pinned help hint itself: its width
// reserve comes from GrammarKeys.Help.Help(), never an independent
// literal that binding could drift from.
func TestFooterHelpHint_DerivesFromGrammarKeysHelp(t *testing.T) {
	want := GrammarKeys.Help.Help().Key + " " + GrammarKeys.Help.Help().Desc
	if footerHelpHint != want {
		t.Errorf("footerHelpHint = %q, want %q (derived from GrammarKeys.Help)", footerHelpHint, want)
	}
}

// TestHintSegs_RolesFollowTheAtom proves the hint atom's own contrast
// split (decision 8, the ratified shell exemplar): the key run plain
// RoleFg, the description run RoleFgMuted with its single leading
// space, and both derived from b.Help() rather than a literal.
func TestHintSegs_RolesFollowTheAtom(t *testing.T) {
	b := bind("j", "j/k", "navigate")
	got := hintSegs(b)

	want := []rowSeg{
		{text: "j/k", role: theme.RoleFg},
		{text: " navigate", role: theme.RoleFgMuted},
	}
	if !slices.Equal(got, want) {
		t.Errorf("hintSegs(%v) = %+v, want %+v", b, got, want)
	}
}

// TestFooterLeftSegs_GapsBetweenHints proves the GapHint separator
// between successive hint atoms, and that it never appears before the
// first.
func TestFooterLeftSegs_GapsBetweenHints(t *testing.T) {
	entry := footerFixture()
	got := footerLeftSegs(entry, 200)

	gap := strings.Repeat(" ", theme.GapHint)
	gaps := 0
	for _, s := range got {
		if s.text == gap && s.role == theme.RoleFg {
			gaps++
		}
	}
	if want := len(entry.FooterPriority) - 1; gaps != want {
		t.Errorf("footerLeftSegs() has %d GapHint separators, want %d", gaps, want)
	}
}

// footerRowFields strips got's own styling and splits it on runs of
// two or more spaces (PadBand and GapHint both qualify; the hint
// atom's own single space between key and description never does),
// so the result is exactly the row's own ordered content: one entry
// per rendered hint, plus the pinned help hint last.
var footerFieldSplit = regexp.MustCompile(`\s{2,}`)

func footerRowFields(row string) []string {
	var fields []string
	for _, f := range footerFieldSplit.Split(ansi.Strip(row), -1) {
		if f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

// TestRenderFooter_ContainsComputedPrefixInOrder closes the loop
// between footerHints's own computed prefix and what renderFooter
// actually paints (the plan's task 7 criterion): at every width, the
// row's own fields are exactly footerHints's rendered set, in order,
// plus the pinned help hint last.
func TestRenderFooter_ContainsComputedPrefixInOrder(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()

	for _, width := range []int{25, 34, 54, 60, 80, 120, 200} {
		row := renderFooter(entry, th, width)
		fields := footerRowFields(row)

		hints := footerHints(entry, max(0, width-2*theme.PadBand))
		want := make([]string, 0, len(hints)+1)
		for _, b := range hints {
			want = append(want, hintText(b))
		}
		want = append(want, footerHelpHint)

		if !slices.Equal(fields, want) {
			t.Errorf("renderFooter(%d) fields = %v, want %v", width, fields, want)
		}
	}
}

// TestRenderFooter_HelpAlwaysPinnedRight proves decision 8's "at
// every width": the pinned help hint's fields end the row at every
// tested width, including the narrowest one that fits no priority
// hint at all.
func TestRenderFooter_HelpAlwaysPinnedRight(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()

	for _, width := range []int{9, 20, 60, 200} {
		fields := footerRowFields(renderFooter(entry, th, width))
		if len(fields) == 0 || fields[len(fields)-1] != footerHelpHint {
			t.Errorf("renderFooter(%d) fields = %v, want the last field to be %q", width, fields, footerHelpHint)
		}
	}
}

// TestRenderFooter_MonotoneGrowth mirrors
// TestFooterHints_MonotoneGrowthWithWidth at the render layer:
// widening the footer band never drops an already-rendered hint.
func TestRenderFooter_MonotoneGrowth(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()

	var prev []string
	for width := 20; width <= 200; width += 5 {
		fields := footerRowFields(renderFooter(entry, th, width))
		hints := fields[:len(fields)-1] // drop the pinned help hint
		if len(hints) < len(prev) {
			t.Fatalf("renderFooter(%d) has %d hints, fewer than a narrower width's %d", width, len(hints), len(prev))
		}
		if !slices.Equal(prev, hints[:len(prev)]) {
			t.Fatalf("renderFooter(%d) hints %v are not an extension of the narrower width's %v", width, hints, prev)
		}
		prev = hints
	}
}

// TestRenderFooter_ExactWidth is the footer's own version of the
// status band's width-invariant test: at every width a real terminal
// reaches (the spartan floor of 60 upward), the rendered band's
// display width is exactly width.
func TestRenderFooter_ExactWidth(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()

	for _, width := range []int{60, 80, 100, 120, 150, 200} {
		got := ansi.StringWidth(ansi.Strip(renderFooter(entry, th, width)))
		if got != width {
			t.Errorf("renderFooter(%d) display width = %d, want %d", width, got, width)
		}
	}
}

// TestFooterHitSpans_MatchesTheRenderedColumns proves the hit spans
// ADR-0017's character grain requires: each span's Rect lands exactly
// on the columns the rendered row's own hint text occupies, one span
// per rendered hint plus the pinned help hint last.
func TestFooterHitSpans_MatchesTheRenderedColumns(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()
	const width = 80

	row := ansi.Strip(renderFooter(entry, th, width))
	footerRow := image.Rect(0, 7, width, 8) // a non-zero origin, proving translation
	spans := FooterHitSpans(entry, StateDigitsSwitch, width, footerRow)

	hints := footerHints(entry, width-2*theme.PadBand)
	if len(spans) != len(hints)+1 {
		t.Fatalf("FooterHitSpans() returned %d spans, want %d (%d hints plus the pinned help hint)", len(spans), len(hints)+1, len(hints))
	}

	for i, span := range spans {
		if span.Target != PointerFooterHint {
			t.Errorf("span %d: Target = %v, want PointerFooterHint", i, span.Target)
		}
		if span.Rect.Min.Y != footerRow.Min.Y || span.Rect.Max.Y != footerRow.Min.Y+1 {
			t.Errorf("span %d: Rect = %v, want a 1-row span at Y=%d", i, span.Rect, footerRow.Min.Y)
		}
		x0, x1 := span.Rect.Min.X-footerRow.Min.X, span.Rect.Max.X-footerRow.Min.X
		if x0 < 0 || x1 > len(row) || x0 > x1 {
			t.Fatalf("span %d: Rect columns [%d,%d) out of bounds for row %q", i, x0, x1, row)
		}
		if got, want := row[x0:x1], hintText(span.Verb); got != want {
			t.Errorf("span %d: row columns [%d,%d) = %q, want the bound hint text %q", i, x0, x1, got, want)
		}
	}
	if spans[len(spans)-1].Verb.Help().Desc != GrammarKeys.Help.Help().Desc {
		t.Errorf("last span's Verb = %v, want GrammarKeys.Help", spans[len(spans)-1].Verb)
	}
}

// footerGrowthFixture is a ScreenEntry whose FooterPriority carries
// 11 hints, each rendering at a uniform 9 columns (one key, one
// space, a seven-letter description): TestRenderFooter_GrowsAcrossGoldenWidths's
// own fixture, sized so the design language's own three golden column
// rungs (spartan 80, standard 120, wide 150) each land on a different
// hint count.
func footerGrowthFixture() ScreenEntry {
	words := []string{
		"archive", "compose", "forward", "message", "reverse",
		"request", "respond", "confirm", "dismiss", "clarify", "acquire",
	}
	keys := "abcdefghijk"
	bindings := make([]key.Binding, len(words))
	for i, w := range words {
		k := string(keys[i])
		bindings[i] = bind(k, k, w)
	}
	return ScreenEntry{
		Type:           reflect.TypeOf(struct{}{}),
		FooterPriority: bindings,
		Keys:           flatKeyMap(bindings...),
	}
}

// TestRenderFooter_GrowsAcrossGoldenWidths pins the footer's own
// growth goldens at the design language's three column rungs
// (spartan 80, standard 120, wide 150): the rendered hint count
// strictly grows from 80 to 120 and from 120 to 150, and the pinned
// help hint still ends every row (decision 8: "? help" at every
// width).
func TestRenderFooter_GrowsAcrossGoldenWidths(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerGrowthFixture()

	tests := []struct {
		width     int
		wantHints int
	}{
		{80, 5},
		{120, 9},
		{150, 11},
	}

	prev := -1
	for _, tt := range tests {
		fields := footerRowFields(renderFooter(entry, th, tt.width))
		hints := fields[:len(fields)-1]
		if len(hints) != tt.wantHints {
			t.Errorf("renderFooter(%d) rendered %d hints, want %d: %v", tt.width, len(hints), tt.wantHints, hints)
		}
		if len(hints) <= prev {
			t.Errorf("width %d: hint count %d did not grow past the narrower width's %d", tt.width, len(hints), prev)
		}
		prev = len(hints)
		if got := fields[len(fields)-1]; got != footerHelpHint {
			t.Errorf("renderFooter(%d) last field = %q, want the pinned help hint %q", tt.width, got, footerHelpHint)
		}
	}
}

// TestFooterHitSpans_OfferedInEveryLegalState proves PointerFooterHint
// is legal in every StateClass (registry.go's pointerLegalStates): a
// footer hint accelerates its own key, which is itself legal in
// whatever state offers it, so spans are never withheld the way a
// surface-digit click is outside StateDigitsSwitch.
func TestFooterHitSpans_OfferedInEveryLegalState(t *testing.T) {
	entry := footerFixture()
	footerRow := image.Rect(0, 0, 80, 1)

	for _, state := range []StateClass{StateDigitsSwitch, StatePrintableEntry, StateModal} {
		if got := FooterHitSpans(entry, state, 80, footerRow); len(got) == 0 {
			t.Errorf("FooterHitSpans(state=%v) returned none, want spans (PointerFooterHint is legal in every state)", state)
		}
	}
}
