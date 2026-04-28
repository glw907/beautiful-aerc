// SPDX-License-Identifier: MIT

package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force color output in tests (no tty detected).
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestNewCompiledTheme(t *testing.T) {
	th := NewCompiledTheme("Test", nordPalette)

	if th.BgBase != lipgloss.Color("#2e3440") {
		t.Errorf("BgBase: got %v, want #2e3440", th.BgBase)
	}
	if th.AccentPrimary != lipgloss.Color("#81a1c1") {
		t.Errorf("AccentPrimary: got %v, want #81a1c1", th.AccentPrimary)
	}
}

func TestNewCompiledThemeStyles(t *testing.T) {
	th := NewCompiledTheme("Test", nordPalette)

	rendered := th.HeaderKey.Render("From:")
	if rendered == "" {
		t.Error("HeaderKey.Render produced empty string")
	}
	if rendered == "From:" {
		t.Error("HeaderKey.Render produced unstyled string")
	}
}

