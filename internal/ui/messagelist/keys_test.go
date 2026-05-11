package messagelist

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestDefaultKeyMap_Bindings(t *testing.T) {
	km := DefaultKeyMap()
	cases := []struct {
		name    string
		binding key.Binding
		text    string
		want    bool
	}{
		{"down j", km.Down, "j", true},
		{"down arrow", km.Down, "down", true},
		{"up k", km.Up, "k", true},
		{"up arrow", km.Up, "up", true},
		{"top g", km.Top, "g", true},
		{"bottom G", km.Bottom, "G", true},
		{"down rejects K", km.Down, "K", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := keyPress(tc.text)
			if got := key.Matches(msg, tc.binding); got != tc.want {
				t.Errorf("key.Matches(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// keyPress returns a tea.KeyPressMsg for the given key text. Modifier
// chords use the "ctrl+x" / "alt+x" / "shift+x" form; the prefix is
// parsed onto KeyPressMsg.Mod and the trailing rune populates Code+Text.
func keyPress(s string) tea.KeyPressMsg {
	var mod tea.KeyMod
	for {
		switch {
		case len(s) > 5 && s[:5] == "ctrl+":
			mod |= tea.ModCtrl
			s = s[5:]
		case len(s) > 4 && s[:4] == "alt+":
			mod |= tea.ModAlt
			s = s[4:]
		case len(s) > 6 && s[:6] == "shift+":
			mod |= tea.ModShift
			s = s[6:]
		default:
			goto done
		}
	}
done:
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: mod}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp, Mod: mod}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: mod}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc, Mod: mod}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: mod}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s, Mod: mod}
}
