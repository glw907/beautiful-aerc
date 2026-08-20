package ui

import (
	"reflect"
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
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
