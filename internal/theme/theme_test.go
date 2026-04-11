package theme

import "testing"

func TestPaletteHex(t *testing.T) {
	tests := []struct {
		name string
		slot string
		want string
	}{
		{"bg_base", "bg_base", "#2e3440"},
		{"accent_primary", "accent_primary", "#81a1c1"},
		{"unknown", "nonexistent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Nord.PaletteHex(tt.slot)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThemeRegistryComplete(t *testing.T) {
	if len(Themes) != 15 {
		t.Fatalf("Themes map has %d entries, want 15", len(Themes))
	}

	// ThemeNames derives from Themes map — verify consistency.
	names := ThemeNames()
	if len(names) != len(Themes) {
		t.Fatalf("ThemeNames() returned %d names, Themes has %d entries",
			len(names), len(Themes))
	}
	for _, name := range names {
		if _, ok := Themes[name]; !ok {
			t.Errorf("ThemeNames() includes %q but Themes map does not", name)
		}
	}

	// Verify alphabetical order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ThemeNames() not sorted: %q before %q", names[i-1], names[i])
		}
	}

	// Verify default theme exists.
	if _, ok := Themes[DefaultThemeName]; !ok {
		t.Errorf("default theme %q not in Themes map", DefaultThemeName)
	}

	// Verify all themes produce styled output.
	for name, th := range Themes {
		if th.Heading.Render("x") == "x" {
			t.Errorf("theme %q Heading style is unstyled", name)
		}
	}

	// Verify all themes have distinct bg_base (no copy-paste errors).
	seen := map[string]string{}
	for name, th := range Themes {
		bg := th.PaletteHex("bg_base")
		if bg == "" {
			t.Errorf("theme %q has empty bg_base", name)
		}
		if other, ok := seen[bg]; ok {
			// Solarized Light and Everforest Light share #fdf6e3 — skip that known pair.
			if !((name == "solarized-light" && other == "everforest-light") ||
				(name == "everforest-light" && other == "solarized-light")) {
				t.Errorf("themes %q and %q share bg_base %s", name, other, bg)
			}
		}
		seen[bg] = name
	}
}
