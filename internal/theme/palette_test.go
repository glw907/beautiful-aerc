package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

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
