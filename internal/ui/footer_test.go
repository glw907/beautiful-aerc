package ui

import (
	"image"
	"os"
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
// footerHelpHint ("? help") is 6 columns and theme.GapPin is 6 (F1's
// ruling: the pinned exemplar's reserve before the pinned help
// hint), so footerHints(entry, width) reserves 12 columns before it
// ever spends any on a priority hint. The boundary widths below are
// computed by hand from these fixed numbers, not from calling
// ansi.StringWidth on the fixture strings at test time, so a bug in
// footerHints's arithmetic cannot cancel out against the same bug
// in the test.
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
		{"width 23: one column short of hint 1's boundary", 23, 0},
		{"width 24: hint 1 exactly fits (12 + 6 help + 6 pin)", 24, 1},
		{"width 36: one column short of hint 2's boundary", 36, 1},
		{"width 37: hint 2 exactly fits (12 + 3 + 10 + 6 + 6)", 37, 2},
		{"width 56: one column short of hint 3's boundary", 56, 2},
		{"width 57: hint 3 exactly fits (12 + 3 + 10 + 3 + 17 + 6 + 6)", 57, 3},
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

// footerCompletenessWidths is the width sweep UX-2 revision 6's
// help-completeness tests exercise footerHints across
// (task-7-findings-r1.md F2): narrow enough that nothing fits,
// through the design language's column rungs, to generous enough
// that a modest priority list always shows in full.
var footerCompletenessWidths = []int{0, 20, 60, 80, 120, 200}

// TestFooterHints_LiveRegistry_EveryRenderedHintIsLegal and its
// sibling below are UX-2 revision 6's "only mechanical claims"
// (design language section 4), run against Registered() rather than
// footerFixture: the footer is always a legal prefix of each
// registered screen's committed order.
func TestFooterHints_LiveRegistry_EveryRenderedHintIsLegal(t *testing.T) {
	for _, e := range Registered() {
		legal := flattenKeys(e.Keys)
		for _, width := range footerCompletenessWidths {
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

// TestHelpContent_EqualsFullKeymap_LiveRegistry proves UX-2 revision
// 6's completeness claim as an actual oracle rather than helpContent
// compared against its definition (task-7-findings-r1.md F2:
// flattenKeys(x) == flattenKeys(x) can never fail). The subset half
// runs against every registered screen: every hint footerHints ever
// renders across footerCompletenessWidths is present in that screen's
// helpContent. Pass 2's currently registered screens are all
// one-hint placeholder keymaps (placeholder.go), so footer and help
// coincide exactly for every one of them today. The strictness half
// (help carries something the footer never shows at any width) is
// proved instead against footerFixture, this file's fixture built
// with exactly that shape: its "quit" binding sits in Keys but never
// in FooterPriority.
func TestHelpContent_EqualsFullKeymap_LiveRegistry(t *testing.T) {
	for _, e := range Registered() {
		help := helpContent(e)
		for _, width := range footerCompletenessWidths {
			for _, b := range footerHints(e, width) {
				if !slices.ContainsFunc(help, func(h key.Binding) bool { return bindingsEqual(h, b) }) {
					t.Errorf("%s: footerHints(%d) rendered %v, absent from helpContent()", e.Type, width, b)
				}
			}
		}
	}

	entry := footerFixture()
	help := helpContent(entry)
	shown := make(map[string]bool)
	for _, width := range footerCompletenessWidths {
		for _, b := range footerHints(entry, width) {
			shown[b.Help().Desc] = true
		}
	}
	if !slices.ContainsFunc(help, func(h key.Binding) bool { return !shown[h.Help().Desc] }) {
		t.Errorf("footerFixture: helpContent() = %v shows nothing beyond what footerHints renders across %v, want at least one binding the footer never shows", help, footerCompletenessWidths)
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

// TestHintSegs_RolesFollowTheAtom proves the hint atom's contrast
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

// footerRowFields strips got's styling and splits it on runs of
// two or more spaces (PadBand and GapHint both qualify; the hint
// atom's single space between key and description never does),
// so the result is exactly the row's ordered content: one entry
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

// footerBoundaryWidths is the width sweep footerFixture's boundary
// crossings sit on (24/37/57, TestFooterHints_LiteralWidthBoundaries),
// plus the design language's column rungs: shared by the render-
// and hit-span-layer tests below (task-7-findings-r1.md F4/F5), each
// of which also ranges over Registered() so a real registered screen's
// (currently one-hint) priority list gets the same proof.
var footerBoundaryWidths = []int{25, 37, 57, 60, 80, 120, 200}

// footerLoopEntries returns footerFixture, footerGrowthFixture, and
// every currently registered screen's entry: the shared input set
// F4 and F5 range the render- and hit-span-layer tests over.
func footerLoopEntries() []ScreenEntry {
	entries := []ScreenEntry{footerFixture(), footerGrowthFixture()}
	return append(entries, Registered()...)
}

// TestRenderFooter_ContainsComputedPrefixInOrder closes the loop
// between footerHints's computed prefix and what renderFooter
// actually paints (the plan's task 7 criterion): at every width, for
// footerFixture, footerGrowthFixture, and every registered screen, the
// row's fields are exactly footerHints's rendered set, in order,
// plus the pinned help hint last.
func TestRenderFooter_ContainsComputedPrefixInOrder(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	for _, entry := range footerLoopEntries() {
		for _, width := range footerBoundaryWidths {
			row := renderFooter(entry, th, width)
			fields := footerRowFields(row)

			hints := footerHints(entry, width-2*theme.PadBand)
			want := make([]string, 0, len(hints)+1)
			for _, b := range hints {
				want = append(want, hintText(b))
			}
			want = append(want, footerHelpHint)

			if !slices.Equal(fields, want) {
				t.Errorf("%v renderFooter(%d) fields = %v, want %v", entry.Type, width, fields, want)
			}
		}
	}
}

// TestRenderFooter_HelpAlwaysPinnedRight proves decision 8's "at
// every width": the pinned help hint's fields end the row at every
// tested width, including one (60, widthSpartanMin, layout.go) that
// fits no priority hint at all. 60 replaces the original narrow case
// of 9 (task-7-findings-r1.md F3): 9 exercised renderFooter's
// defensive clamp for a width no caller ever supplies, and renderFooter
// no longer carries one, since its documented precondition is that
// Render never calls it below the width floor.
func TestRenderFooter_HelpAlwaysPinnedRight(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerFixture()

	for _, width := range []int{20, 60, 200} {
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

// TestRenderFooter_ExactWidth is the footer's version of the
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
// on the columns the rendered row's hint text occupies, one span
// per rendered hint plus the pinned help hint last.
func TestFooterHitSpans_MatchesTheRenderedColumns(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	for _, entry := range footerLoopEntries() {
		for _, width := range footerBoundaryWidths {
			row := ansi.Strip(renderFooter(entry, th, width))
			footerRow := image.Rect(0, 7, width, 8) // a non-zero origin, proving translation
			spans := FooterHitSpans(entry, StateDigitsSwitch, footerRow)

			hints := footerHints(entry, width-2*theme.PadBand)
			if len(spans) != len(hints)+1 {
				t.Fatalf("%v width %d: FooterHitSpans() returned %d spans, want %d (%d hints plus the pinned help hint)",
					entry.Type, width, len(spans), len(hints)+1, len(hints))
			}

			for i, span := range spans {
				if span.Target != PointerFooterHint {
					t.Errorf("%v width %d: span %d: Target = %v, want PointerFooterHint", entry.Type, width, i, span.Target)
				}
				if span.Rect.Min.Y != footerRow.Min.Y || span.Rect.Max.Y != footerRow.Min.Y+1 {
					t.Errorf("%v width %d: span %d: Rect = %v, want a 1-row span at Y=%d", entry.Type, width, i, span.Rect, footerRow.Min.Y)
				}
				x0, x1 := span.Rect.Min.X-footerRow.Min.X, span.Rect.Max.X-footerRow.Min.X
				if x0 < 0 || x1 > len(row) || x0 > x1 {
					t.Fatalf("%v width %d: span %d: Rect columns [%d,%d) out of bounds for row %q", entry.Type, width, i, x0, x1, row)
				}
				if got, want := row[x0:x1], hintText(span.Verb); got != want {
					t.Errorf("%v width %d: span %d: row columns [%d,%d) = %q, want the bound hint text %q", entry.Type, width, i, x0, x1, got, want)
				}
			}

			// The pin's column: the last span is always the pinned
			// help hint, and its Rect lands on the row's trailing
			// "? help" text, not merely on some final span.
			pin := spans[len(spans)-1]
			if pin.Verb.Help().Desc != GrammarKeys.Help.Help().Desc {
				t.Errorf("%v width %d: last span's Verb = %v, want GrammarKeys.Help", entry.Type, width, pin.Verb)
			}
			px0, px1 := pin.Rect.Min.X-footerRow.Min.X, pin.Rect.Max.X-footerRow.Min.X
			if got, want := row[px0:px1], footerHelpHint; got != want {
				t.Errorf("%v width %d: pin columns [%d,%d) = %q, want %q", entry.Type, width, px0, px1, got, want)
			}
		}
	}
}

// footerGrowthFixture is a ScreenEntry whose FooterPriority carries
// 13 hints, each rendering at a uniform 9 columns (one key, one
// space, a seven-letter description): TestRenderFooter_GrowsAcrossGoldenWidths's
// fixture, sized so the design language's three golden column
// rungs (spartan 80, standard 120, wide 150) each land on a different
// hint count, and 150 (11 of 13) still truncates rather than showing
// every hint whole (task-7-findings-r1.md F7).
func footerGrowthFixture() ScreenEntry {
	words := []string{
		"archive", "compose", "forward", "message", "reverse",
		"request", "respond", "confirm", "dismiss", "clarify", "acquire",
		"execute", "attempt",
	}
	keys := "abcdefghijklm"
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

// TestRenderFooter_GrowsAcrossGoldenWidths pins the footer's
// growth goldens at the design language's three column rungs (spartan
// 80, standard 120, wide 150), literal expected rows rather than a
// field count (task-7-findings-r1.md F7's golden: PadBand's
// leading and trailing two cells, GapHint's three-cell inter-hint gap,
// and GapPin's six-cell-or-wider reserve before the pin all show
// in the exact text). 150 renders 11 of the fixture's 13 hints, so the
// golden also proves the wide rung still truncates rather than always
// showing everything whole.
func TestRenderFooter_GrowsAcrossGoldenWidths(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	entry := footerGrowthFixture()

	tests := []struct {
		width int
		want  string
	}{
		{80, "  a archive   b compose   c forward   d message   e reverse             ? help  "},
		{120, "  a archive   b compose   c forward   d message   e reverse   f request   g respond   h confirm                 ? help  "},
		{150, "  a archive   b compose   c forward   d message   e reverse   f request   g respond   h confirm   i dismiss   j clarify   k acquire           ? help  "},
	}

	for _, tt := range tests {
		if got := ansi.Strip(renderFooter(entry, th, tt.width)); got != tt.want {
			t.Errorf("renderFooter(%d) = %q, want %q", tt.width, got, tt.want)
		}
	}
}

// exemplarFooterPriority transcribes the pinned shell exemplar's
// FOOTER_PRIORITY
// (docs/poplar/design/2026-08-19-shell-exemplar/shell-exemplar.py),
// in its order: TestRenderFooter_MatchesPinnedExemplarAt80's
// fixture, so the byte-for-byte comparison below is against the exact
// hint set the exemplar itself renders, never a hand-picked stand-in.
func exemplarFooterPriority() []key.Binding {
	pairs := [][2]string{
		{"Enter", "open"},
		{"a", "archive"},
		{"d", "delete"},
		{"r", "reply"},
		{"m", "compose"},
		{"/", "search"},
		{"u", "undo"},
		{"g", "go to"},
		{"*", "flag"},
		{"s", "move"},
		{"Tab", "unread"},
		{"j/k", "select"},
	}
	bindings := make([]key.Binding, len(pairs))
	for i, p := range pairs {
		bindings[i] = bind(p[0], p[0], p[1])
	}
	return bindings
}

// exemplarRenderPath is the pinned shell exemplar's committed
// render, the source TestRenderFooter_MatchesPinnedExemplarAt80 reads
// directly rather than transcribing a line of it as a literal, so the
// pin cannot drift from what the exemplar itself renders.
const exemplarRenderPath = "../../docs/poplar/design/2026-08-19-shell-exemplar/render.txt"

// exemplarRenderLine returns line n (1-based) of exemplarRenderPath,
// with show()'s two-cell print indent stripped: that indent is
// never part of the row itself.
func exemplarRenderLine(t *testing.T, n int) string {
	t.Helper()
	data, err := os.ReadFile(exemplarRenderPath)
	if err != nil {
		t.Fatalf("read %s: %v", exemplarRenderPath, err)
	}
	lines := strings.Split(string(data), "\n")
	if n < 1 || n > len(lines) {
		t.Fatalf("%s has %d line(s), want line %d to exist", exemplarRenderPath, len(lines), n)
	}
	return strings.TrimPrefix(lines[n-1], "  ")
}

// TestRenderFooter_MatchesPinnedExemplarAt80 verifies the pin-reserve
// ruling: rendering the exemplar's FOOTER_PRIORITY at 80 columns
// reproduces the exemplar's "Spartan 80" row, line 78 of
// exemplarRenderPath, byte for byte.
func TestRenderFooter_MatchesPinnedExemplarAt80(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	bindings := exemplarFooterPriority()
	entry := ScreenEntry{
		Type:           reflect.TypeOf(struct{}{}),
		FooterPriority: bindings,
		Keys:           flatKeyMap(bindings...),
	}

	want := exemplarRenderLine(t, 78)
	if got := ansi.Strip(renderFooter(entry, th, 80)); got != want {
		t.Errorf("renderFooter(80) = %q, want the exemplar's own row %q", got, want)
	}
}

// TestFooterHitSpans_OfferedInEveryLegalState proves PointerFooterHint
// is legal in every StateClass except StateModal (registry.go's
// pointerLegalStates, corrected by task-10-findings-r2.md's F2
// corollary ruling): a footer hint accelerates its key, which is
// itself legal in whatever state offers it, so spans are never
// withheld the way a surface-digit click is outside StateDigitsSwitch;
// StateModal is the one exception, since the modal renders
// full-terminal and no footer band exists there to click.
func TestFooterHitSpans_OfferedInEveryLegalState(t *testing.T) {
	entry := footerFixture()
	footerRow := image.Rect(0, 0, 80, 1)

	for _, state := range []StateClass{StateDigitsSwitch, StatePrintableEntry} {
		if got := FooterHitSpans(entry, state, footerRow); len(got) == 0 {
			t.Errorf("FooterHitSpans(state=%v) returned none, want spans", state)
		}
	}
	if got := FooterHitSpans(entry, StateModal, footerRow); got != nil {
		t.Errorf("FooterHitSpans(StateModal) = %v, want none (the modal renders full-terminal)", got)
	}
}
