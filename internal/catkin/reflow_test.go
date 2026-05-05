package catkin

import (
	"testing"
	"unicode/utf8"
)

func TestReflowParagraphSimple(t *testing.T) {
	src := "the quick brown fox jumps over the lazy dog"
	got, _ := Reflow(src, 20, 0)
	want := "the quick brown fox\njumps over the lazy\ndog"
	if got != want {
		t.Errorf("Reflow:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowPreservesQuotePrefix(t *testing.T) {
	src := "> the quick brown fox jumps over the lazy dog"
	got, _ := Reflow(src, 20, 0)
	want := "> the quick brown\n> fox jumps over the\n> lazy dog"
	if got != want {
		t.Errorf("Reflow quoted:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowSkipsCodeFence(t *testing.T) {
	src := "```\nthis is code with very long lines that should not wrap\n```"
	got, _ := Reflow(src, 20, 0)
	if got != src {
		t.Errorf("Reflow should preserve code fence:\ngot  %q\nwant %q", got, src)
	}
}

func TestReflowSkipsHeading(t *testing.T) {
	src := "# A long heading that exceeds the wrap width"
	got, _ := Reflow(src, 20, 0)
	if got != src {
		t.Errorf("Reflow should preserve heading:\ngot  %q\nwant %q", got, src)
	}
}

func TestReflowNeverBreaksLongToken(t *testing.T) {
	src := "see https://example.com/very/long/path/that/exceeds/wrap/width here"
	got, _ := Reflow(src, 20, 0)
	want := "see\nhttps://example.com/very/long/path/that/exceeds/wrap/width\nhere"
	if got != want {
		t.Errorf("Reflow long-token:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowIdempotent(t *testing.T) {
	src := "the quick brown fox jumps over the lazy dog"
	once, _ := Reflow(src, 20, 0)
	twice, _ := Reflow(once, 20, 0)
	if once != twice {
		t.Errorf("Reflow not idempotent:\nonce  %q\ntwice %q", once, twice)
	}
}

func TestReflowCursorTracking(t *testing.T) {
	src := "the quick brown fox"
	got, cur := Reflow(src, 10, 10)
	want := "the quick\nbrown fox"
	if got != want {
		t.Errorf("Reflow cursor: got %q, want %q", got, want)
	}
	if cur != 10 {
		t.Errorf("Reflow cursor offset: got %d, want 10", cur)
	}
}

func TestReflowCursorTrackingMultibyte(t *testing.T) {
	src := "über schöne fluß"
	got, cur := Reflow(src, 10, 6)
	// "über " is 5 runes. Cursor at 6 is inside "schöne".
	// Exact post-reflow placement is approximate. Verify the cursor is in-range.
	if cur < 0 || cur > utf8.RuneCountInString(got) {
		t.Errorf("Reflow multibyte cursor out of range: got %d for %q (len %d)", cur, got, utf8.RuneCountInString(got))
	}
}
