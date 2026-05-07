package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestStatusBarView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("renders counts and connection", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := stripANSI(sb.View(80, 30))
		if !strings.Contains(result, "10 messages") {
			t.Error("missing message count")
		}
		if !strings.Contains(result, "3 unread") {
			t.Error("missing unread count")
		}
		if !strings.Contains(result, "● connected") {
			t.Error("missing connection indicator")
		}
	})

	t.Run("no folder name", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := stripANSI(sb.View(80, 30))
		if strings.Contains(result, "Inbox") {
			t.Error("status bar should not show folder name")
		}
	})

	t.Run("ends with ─╯", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := stripANSI(sb.View(80, 30))
		trimmed := strings.TrimRight(result, " ")
		if !strings.HasSuffix(trimmed, "─╯") {
			t.Errorf("status bar should end with ─╯: %q", trimmed)
		}
	})

	t.Run("has ┴ at divider position", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := stripANSI(sb.View(80, 30))
		runes := []rune(result)
		if len(runes) > 30 && runes[30] != '┴' {
			t.Errorf("expected ┴ at position 30, got %c", runes[30])
		}
	})

	t.Run("offline state", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Offline)
		result := stripANSI(sb.View(80, 30))
		if !strings.Contains(result, "○ offline") {
			t.Error("missing offline indicator")
		}
	})

	t.Run("no divider when dividerCol is 0", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := stripANSI(sb.View(80, 0))
		if strings.Contains(result, "┴") {
			t.Error("should not have ┴ when dividerCol is 0")
		}
	})

	t.Run("width matches terminal", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		result := sb.View(80, 30)
		if lipgloss.Width(result) != 80 {
			t.Errorf("width = %d, want 80", lipgloss.Width(result))
		}
	})
}

func TestStatusBarViewerMode(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetMode(StatusViewer).SetScrollPct(42).SetConnectionState(Connected)
	result := stripANSI(sb.View(80, 30))
	if !strings.Contains(result, "42%") {
		t.Errorf("expected scroll pct in viewer mode: %q", result)
	}
	if strings.Contains(result, "messages") {
		t.Error("viewer mode should not show message counts")
	}
}

func TestStatusBarSetScrollPctClamps(t *testing.T) {
	styles := NewStyles(theme.Nord)
	cases := []struct{ in, want int }{{-5, 0}, {0, 0}, {50, 50}, {100, 100}, {150, 100}}
	for _, c := range cases {
		sb := NewStatusBar(styles).SetMode(StatusViewer).SetScrollPct(c.in)
		if sb.scrollPct != c.want {
			t.Errorf("SetScrollPct(%d) = %d, want %d", c.in, sb.scrollPct, c.want)
		}
	}
}

func TestStatusBar_OutboxSegmentHiddenWhenZero(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles)
	sb = sb.SetCounts(10, 2).SetOutboxDepth(0, 0)
	out := sb.View(80, 30)
	if strings.Contains(out, "⇅") || strings.Contains(out, "⚠") {
		t.Errorf("unexpected outbox glyph in %q", out)
	}
}

func TestStatusBar_OutboxPendingRendersArrows(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles)
	sb = sb.SetCounts(10, 2).SetOutboxDepth(3, 0)
	out := sb.View(80, 30)
	if !strings.Contains(out, "⇅3") {
		t.Errorf("missing ⇅3 in %q", out)
	}
	if strings.Contains(out, "⚠") {
		t.Errorf("unexpected ⚠ in %q", out)
	}
}

func TestStatusBar_OutboxConflictWinsOverPending(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles)
	sb = sb.SetCounts(10, 2).SetOutboxDepth(3, 1)
	out := sb.View(80, 30)
	if !strings.Contains(out, "⚠4") {
		t.Errorf("missing ⚠4 in %q", out)
	}
	if strings.Contains(out, "⇅") {
		t.Errorf("⇅ still present alongside ⚠ in %q", out)
	}
}
