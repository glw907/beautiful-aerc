package ui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func TestStatusBarView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("renders counts and connection", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
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
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
		result := stripANSI(sb.View(80, 30))
		if strings.Contains(result, "Inbox") {
			t.Error("status bar should not show folder name")
		}
	})

	t.Run("ends with ─╯", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
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
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
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
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
		result := stripANSI(sb.View(80, 30))
		if !strings.Contains(result, "○ offline") {
			t.Error("missing offline indicator")
		}
	})

	t.Run("no divider when dividerCol is 0", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
		result := stripANSI(sb.View(80, 0))
		if strings.Contains(result, "┴") {
			t.Error("should not have ┴ when dividerCol is 0")
		}
	})

	t.Run("width matches terminal", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb = sb.SetCounts(10, 3)
		sb = sb.SetConnectionState(Connected)
		sb = sb.SetBackendState(uicore.BackendConnected, nil)
		result := sb.View(80, 30)
		if lipgloss.Width(result) != 80 {
			t.Errorf("width = %d, want 80", lipgloss.Width(result))
		}
	})
}

func TestStatusBarViewerMode(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetMode(StatusViewer).SetScrollPct(42).SetConnectionState(Connected).SetBackendState(uicore.BackendConnected, nil)
	result := stripANSI(sb.View(80, 30))
	if !strings.Contains(result, "42%") {
		t.Errorf("expected scroll pct in viewer mode: %q", result)
	}
	if strings.Contains(result, "messages") {
		t.Error("viewer mode should not show message counts")
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
	out := stripANSI(sb.View(80, 30))
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
	out := stripANSI(sb.View(80, 30))
	if !strings.Contains(out, "⚠4") {
		t.Errorf("missing ⚠4 in %q", out)
	}
	if strings.Contains(out, "⇅") {
		t.Errorf("⇅ still present alongside ⚠ in %q", out)
	}
}

func TestStatusBar_BackendConnecting(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendConnecting, nil)
	out := stripANSI(sb.View(80, 0))
	if !strings.Contains(out, "connecting") {
		t.Fatalf("Connecting render missing 'connecting': %q", out)
	}
}

func TestStatusBar_BackendFailed_ShowsErrorAndRetryHint(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendFailed, errors.New("dial: timeout"))
	out := stripANSI(sb.View(120, 0))
	if !strings.Contains(out, "dial: timeout") {
		t.Fatalf("Failed render missing error text: %q", out)
	}
	if !strings.Contains(out, "r retry") {
		t.Fatalf("Failed render missing 'r retry' hint: %q", out)
	}
}

func TestStatusBar_BackendFailed_NilErr(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendFailed, nil)
	out := stripANSI(sb.View(80, 0))
	if !strings.Contains(out, "disconnected") {
		t.Fatalf("Failed nil-err render missing 'disconnected': %q", out)
	}
	if !strings.Contains(out, "r retry") {
		t.Fatalf("Failed nil-err render missing 'r retry': %q", out)
	}
}

func TestStatusBar_BackendFailed_MultilineErr(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendFailed, errors.New("dial: timeout\nextra detail"))
	out := stripANSI(sb.View(120, 0))
	if !strings.Contains(out, "dial: timeout") {
		t.Fatalf("Failed multiline render missing first line: %q", out)
	}
	if strings.Contains(out, "extra detail") {
		t.Fatalf("Failed multiline render should show only first line: %q", out)
	}
}

func TestStatusBar_BackendConnected_FallsThroughToConnState(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).
		SetConnectionState(Connected).
		SetBackendState(uicore.BackendConnected, nil)
	out := stripANSI(sb.View(80, 0))
	if !strings.Contains(out, "● connected") {
		t.Fatalf("BackendConnected render should show '● connected': %q", out)
	}
	if strings.Contains(out, "connecting") || strings.Contains(out, "retry") {
		t.Fatalf("BackendConnected render must not show pre-wire text: %q", out)
	}
}

func TestStatusBar_BackendConnected_OfflineConnState(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).
		SetConnectionState(Offline).
		SetBackendState(uicore.BackendConnected, nil)
	out := stripANSI(sb.View(80, 0))
	if !strings.Contains(out, "○ offline") {
		t.Fatalf("BackendConnected+Offline render should show '○ offline': %q", out)
	}
}

func TestStatusBar_BackendConnecting_Width(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendConnecting, nil)
	result := sb.View(80, 0)
	if lipgloss.Width(result) != 80 {
		t.Errorf("width = %d, want 80", lipgloss.Width(result))
	}
}

func TestStatusBar_BackendFailed_Width(t *testing.T) {
	styles := NewStyles(theme.Nord)
	sb := NewStatusBar(styles).SetBackendState(uicore.BackendFailed, errors.New("connect: connection refused"))
	result := sb.View(80, 0)
	if lipgloss.Width(result) != 80 {
		t.Errorf("width = %d, want 80", lipgloss.Width(result))
	}
}

func TestStatusBar_BackfillSegment(t *testing.T) {
	styles := NewStyles(theme.Nord)
	cases := []struct {
		name     string
		width    int
		done     int
		total    int
		paused   bool
		warn     bool
		contains string // empty = segment must be absent
	}{
		{"hidden when caught up", 120, 100, 100, false, false, ""},
		{"hidden when no messages", 120, 0, 0, false, false, ""},
		{"full active", 120, 18, 33, false, false, "↓ 18/33"},
		{"glyph-only Spartan", 80, 18, 33, false, false, "↓"},
		{"full paused", 120, 18, 33, true, false, "↓ paused"},
		{"glyph paused Spartan", 80, 18, 33, true, false, "↓⏸"},
		{"full warn", 120, 18, 33, false, true, "↓ ⚠"},
		{"glyph warn Spartan", 80, 18, 33, false, true, "↓⚠"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb := NewStatusBar(styles).SetBackfill(c.done, c.total, c.paused, c.warn)
			out := stripANSI(sb.View(c.width, c.width-1))
			if c.contains == "" {
				if strings.Contains(out, "↓") {
					t.Errorf("expected no backfill segment; got %q", out)
				}
				return
			}
			if !strings.Contains(out, c.contains) {
				t.Errorf("View width=%d: missing %q in %q", c.width, c.contains, out)
			}
		})
	}
}
