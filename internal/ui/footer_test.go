package ui

import (
	"reflect"
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
)

// footerFixture is a ScreenEntry whose FooterPriority carries three
// hints of increasing individual width, so FooterHints has real
// prefix decisions to make at each width step.
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

func TestFooterHints_WidthMaximalPrefix(t *testing.T) {
	entry := footerFixture()

	// "j/k navigate" is 12 columns; add the help hint (6) plus two
	// gaps (3 each) and the pinned help hint's own reserve: a width
	// that fits exactly the first hint and no more proves the
	// boundary is the hint's own width, not an off-by-one.
	got := FooterHints(entry, len("j/k navigate")+len(footerHelpHint)+footerGap)
	want := entry.FooterPriority[:1]
	if !slices.EqualFunc(got, want, bindingsEqual) {
		t.Errorf("FooterHints at the one-hint boundary = %v, want %v", got, want)
	}
}

func TestFooterHints_NarrowestWidthYieldsNoPriorityHints(t *testing.T) {
	entry := footerFixture()

	got := FooterHints(entry, len(footerHelpHint))
	if len(got) != 0 {
		t.Errorf("FooterHints at the narrowest width = %v, want none", got)
	}
}

func TestFooterHints_IsAPrefixOfFooterPriority(t *testing.T) {
	entry := footerFixture()

	got := FooterHints(entry, 200)
	for i, b := range got {
		if !bindingsEqual(b, entry.FooterPriority[i]) {
			t.Fatalf("FooterHints()[%d] = %v, want FooterPriority[%d] = %v", i, b, i, entry.FooterPriority[i])
		}
	}
}

func TestFooterHints_EveryRenderedHintIsLegal(t *testing.T) {
	entry := footerFixture()
	legal := flattenKeys(entry.Keys)

	for _, width := range []int{10, 20, 40, 80, 160} {
		for _, b := range FooterHints(entry, width) {
			if !slices.ContainsFunc(legal, func(l key.Binding) bool { return bindingsEqual(l, b) }) {
				t.Errorf("FooterHints(%d) rendered %v, not present in the screen's own keymap", width, b)
			}
		}
	}
}

func TestFooterHints_MonotoneGrowthWithWidth(t *testing.T) {
	entry := footerFixture()

	prevLen := -1
	var prev []key.Binding
	for width := 0; width <= 200; width += 5 {
		got := FooterHints(entry, width)
		if len(got) < prevLen {
			t.Fatalf("FooterHints(%d) has %d hints, fewer than width %d's %d", width, len(got), width-5, prevLen)
		}
		if !slices.EqualFunc(prev, got[:min(len(prev), len(got))], bindingsEqual) {
			t.Fatalf("FooterHints(%d) = %v is not an extension of the narrower width's %v", width, got, prev)
		}
		prevLen, prev = len(got), got
	}
}

func TestHelpContent_EqualsFullKeymap(t *testing.T) {
	entry := footerFixture()

	got := HelpContent(entry)
	want := flattenKeys(entry.Keys)
	if !slices.EqualFunc(got, want, bindingsEqual) {
		t.Errorf("HelpContent() = %v, want the full keymap %v", got, want)
	}

	// The footer's width-limited prefix is always a subset of help
	// content, never a superset: help is the completeness surface.
	for _, width := range []int{0, 10, 30, 200} {
		for _, b := range FooterHints(entry, width) {
			if !slices.ContainsFunc(got, func(h key.Binding) bool { return bindingsEqual(h, b) }) {
				t.Errorf("footer hint %v at width %d is absent from HelpContent()", b, width)
			}
		}
	}
}
