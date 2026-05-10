package uicore

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/glw907/poplar/internal/theme"
)

func TestNewSpinner(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	sp := NewSpinner(th)
	if got := len(sp.Spinner.Frames); got != len(spinner.Dot.Frames) {
		t.Errorf("frames: got %d, want %d (spinner.Dot)", got, len(spinner.Dot.Frames))
	}
	if sp.Style.GetForeground() == nil {
		t.Error("NewSpinner returned a model with no foreground color")
	}
	if !strings.Contains(sp.Style.Render("x"), "x") {
		t.Errorf("Style.Render dropped its content")
	}
}
