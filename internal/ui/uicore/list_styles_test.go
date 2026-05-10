package uicore

import (
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestNewListStyles(t *testing.T) {
	ct := theme.Themes[theme.DefaultThemeName]
	s := NewListStyles(ct)

	cases := []struct {
		name string
		out  string
	}{
		{"Title", s.Title.Render("title")},
		{"StatusBar", s.StatusBar.Render("status")},
		{"NoItems", s.NoItems.Render("nothing")},
		{"FilterPrompt", s.Filter.Focused.Prompt.Render(">")},
		{"DefaultFilterCharacterMatch", s.DefaultFilterCharacterMatch.Render("a")},
	}
	for _, c := range cases {
		if c.out == "" {
			t.Errorf("%s rendered empty string", c.name)
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
