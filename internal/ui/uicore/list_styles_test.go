package uicore

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

func TestNewListStyles(t *testing.T) {
	ct := theme.Themes[theme.DefaultThemeName]
	s := NewListStyles(ct)

	cases := []struct {
		name     string
		style    lipgloss.Style
		wantBold bool
		wantIt   bool
	}{
		{name: "Title", style: s.Title, wantBold: true},
		{name: "StatusBar", style: s.StatusBar},
		{name: "NoItems", style: s.NoItems, wantIt: true},
		{name: "FilterPrompt", style: s.Filter.Focused.Prompt},
		{name: "DefaultFilterCharacterMatch", style: s.DefaultFilterCharacterMatch},
	}
	for _, c := range cases {
		if c.style.GetForeground() == nil {
			t.Errorf("%s.GetForeground() = nil, want non-nil", c.name)
		}
		if c.wantBold && !c.style.GetBold() {
			t.Errorf("%s.GetBold() = false, want true", c.name)
		}
		if c.wantIt && !c.style.GetItalic() {
			t.Errorf("%s.GetItalic() = false, want true", c.name)
		}
	}
}

func TestNewListStyles_deterministic(t *testing.T) {
	ct := theme.Themes[theme.DefaultThemeName]
	a := NewListStyles(ct)
	b := NewListStyles(ct)
	if a.Title.Render("x") != b.Title.Render("x") {
		t.Fatalf("NewListStyles is not deterministic for the same theme")
	}
}
