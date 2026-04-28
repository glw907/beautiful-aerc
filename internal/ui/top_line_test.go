// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestTopLineView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("basic frame line", func(t *testing.T) {
		tl := NewTopLine(styles)
		result := stripANSI(tl.View(80, 30))
		if !strings.HasSuffix(strings.TrimRight(result, " "), "╮") {
			t.Errorf("top line missing ╮ at right edge: %q", result)
		}
		if !strings.Contains(result, "┬") {
			t.Errorf("top line missing ┬ divider junction: %q", result)
		}
	})

	t.Run("divider at correct position", func(t *testing.T) {
		tl := NewTopLine(styles)
		result := stripANSI(tl.View(80, 30))
		runes := []rune(result)
		if runes[30] != '┬' {
			t.Errorf("expected ┬ at position 30, got %c", runes[30])
		}
	})

	t.Run("width matches terminal", func(t *testing.T) {
		tl := NewTopLine(styles)
		result := tl.View(80, 30)
		if lipgloss.Width(result) != 80 {
			t.Errorf("width = %d, want 80", lipgloss.Width(result))
		}
	})

	t.Run("no divider when dividerCol is 0", func(t *testing.T) {
		tl := NewTopLine(styles)
		result := stripANSI(tl.View(80, 0))
		if strings.Contains(result, "┬") {
			t.Errorf("should not have ┬ when dividerCol is 0: %q", result)
		}
	})

	t.Run("toast overlays right side", func(t *testing.T) {
		tl := NewTopLine(styles)
		tl = tl.SetToast("✓ 3 archived")
		result := stripANSI(tl.View(80, 30))
		if !strings.Contains(result, "✓ 3 archived") {
			t.Errorf("toast not visible: %q", result)
		}
		if !strings.HasSuffix(strings.TrimRight(result, " "), "╮") {
			t.Errorf("╮ missing after toast: %q", result)
		}
	})

	t.Run("toast clears", func(t *testing.T) {
		tl := NewTopLine(styles)
		tl = tl.SetToast("✓ done")
		tl = tl.ClearToast()
		result := stripANSI(tl.View(80, 30))
		if strings.Contains(result, "done") {
			t.Error("toast should be cleared")
		}
	})
}
