package theme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// stylesDiffer reports whether a and b would render visibly
// differently, over the attribute surface Style and TypeStyle
// actually set (foreground, background, reverse, bold, italic,
// underline, faint).
func stylesDiffer(a, b lipgloss.Style) bool {
	return !sameColor(a.GetForeground(), b.GetForeground()) ||
		!sameColor(a.GetBackground(), b.GetBackground()) ||
		a.GetReverse() != b.GetReverse() ||
		a.GetBold() != b.GetBold() ||
		a.GetItalic() != b.GetItalic() ||
		a.GetUnderline() != b.GetUnderline() ||
		a.GetFaint() != b.GetFaint()
}

func TestStyleTrueColor(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		ground     Ground
		isDark     bool
		wantFgHex  string
		wantBgHex  string
		wantBold   bool
		wantItalic bool
		wantUnder  bool
	}{
		{"fg on base, dark", RoleFg, GroundBase, true, "D4D8DF", "16181D", false, false, false},
		{"fg on panel, light", RoleFg, GroundPanel, false, "262B33", "FFFFFF", false, false, false},
		{"unread is bold", RoleUnread, GroundBase, true, "ECEEF2", "16181D", true, false, false},
		{"quote is italic", RoleQuote, GroundPanel, true, "A0A5AE", "262B36", false, true, false},
		{"link is underlined, aliases accent", RoleLink, GroundBase, true, "85B3D1", "16181D", false, false, true},
		{"flag aliases warn", RoleFlag, GroundCode, true, "D4B36A", "1D2026", false, false, false},
		{"diffAdd aliases success", RoleDiffAdd, GroundBase, true, "97BE8C", "16181D", false, false, false},
		{"diffDel aliases error", RoleDiffDel, GroundBase, true, "DF8484", "16181D", false, false, false},
		{"focusedBorder aliases accent", RoleFocusedBorder, GroundSelected, false, "285370", "C4CDDB", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.isDark, ProfileTrueColor).Style(tt.role, tt.ground)
			if !sameColor(s.GetForeground(), hexColor(tt.wantFgHex)) {
				t.Errorf("foreground = %v, want #%s", s.GetForeground(), tt.wantFgHex)
			}
			if !sameColor(s.GetBackground(), hexColor(tt.wantBgHex)) {
				t.Errorf("background = %v, want #%s", s.GetBackground(), tt.wantBgHex)
			}
			if s.GetBold() != tt.wantBold {
				t.Errorf("bold = %v, want %v", s.GetBold(), tt.wantBold)
			}
			if s.GetItalic() != tt.wantItalic {
				t.Errorf("italic = %v, want %v", s.GetItalic(), tt.wantItalic)
			}
			if s.GetUnderline() != tt.wantUnder {
				t.Errorf("underline = %v, want %v", s.GetUnderline(), tt.wantUnder)
			}
		})
	}
}

func TestTypeStyle(t *testing.T) {
	th := New(true, ProfileTrueColor)
	tests := []struct {
		name       string
		role       TypeRole
		wantFgHex  string
		wantBold   bool
		wantItalic bool
	}{
		{"title is bold fg", TypeTitle, "D4D8DF", true, false},
		{"label is fgMuted", TypeLabel, "969DA9", false, false},
		{"value is fg", TypeValue, "D4D8DF", false, false},
		{"hint is italic fgSubtle", TypeHint, "7A8290", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := th.TypeStyle(tt.role, GroundBase)
			if !sameColor(s.GetForeground(), hexColor(tt.wantFgHex)) {
				t.Errorf("foreground = %v, want #%s", s.GetForeground(), tt.wantFgHex)
			}
			if s.GetBold() != tt.wantBold {
				t.Errorf("bold = %v, want %v", s.GetBold(), tt.wantBold)
			}
			if s.GetItalic() != tt.wantItalic {
				t.Errorf("italic = %v, want %v", s.GetItalic(), tt.wantItalic)
			}
		})
	}
}

func TestCalendarSlotCycles(t *testing.T) {
	th := New(true, ProfileTrueColor)
	first := th.CalendarSlot(0, GroundBase).GetForeground()
	wrapped := th.CalendarSlot(8, GroundBase).GetForeground()
	if !sameColor(first, wrapped) {
		t.Errorf("slot 8 = %v, want slot 0's color %v (cycle past 8)", wrapped, first)
	}
	negative := th.CalendarSlot(-1, GroundBase).GetForeground()
	last := th.CalendarSlot(7, GroundBase).GetForeground()
	if !sameColor(negative, last) {
		t.Errorf("slot -1 = %v, want slot 7's color %v", negative, last)
	}
}

func TestBorderKinds(t *testing.T) {
	th := New(true, ProfileTrueColor)
	divider := th.Border(BorderDivider)
	modal := th.Border(BorderModal)
	focused := th.Border(BorderFocused)
	if divider == modal || divider == focused || modal == focused {
		t.Errorf("border kinds are not pairwise distinct: divider=%+v modal=%+v focused=%+v", divider, modal, focused)
	}
}

// TestBorderCollapsesUnderDegrade asserts C3a: every BorderKind
// returns lipgloss.ASCIIBorder at ProfileANSI16 and ProfileNoColor,
// since box-drawing weight is not a channel those profiles carry
// reliably (the focused state's own degrade channel is the edge
// bar's glyph weight, not the border).
func TestBorderCollapsesUnderDegrade(t *testing.T) {
	ascii := lipgloss.ASCIIBorder()
	for _, p := range []Profile{ProfileANSI16, ProfileNoColor} {
		th := New(true, p)
		for _, kind := range []BorderKind{BorderDivider, BorderModal, BorderFocused} {
			if got := th.Border(kind); got != ascii {
				t.Errorf("profile %v kind %v: Border() = %+v, want ASCIIBorder", p, kind, got)
			}
		}
	}
}

func TestDrawsDividers(t *testing.T) {
	tests := []struct {
		profile Profile
		want    bool
	}{
		{ProfileTrueColor, false},
		{ProfileANSI16, true},
		{ProfileNoColor, true},
	}
	for _, tt := range tests {
		if got := New(true, tt.profile).DrawsDividers(); got != tt.want {
			t.Errorf("DrawsDividers() at profile %v = %v, want %v", tt.profile, got, tt.want)
		}
	}
}

func TestSpinner(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    []string
	}{
		{"truecolor is braille", ProfileTrueColor, brailleFrames},
		{"ansi16 is ascii", ProfileANSI16, asciiSpinnerFrames},
		{"nocolor is ascii", ProfileNoColor, asciiSpinnerFrames},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(true, tt.profile).Spinner()
			if len(got) != len(tt.want) {
				t.Fatalf("Spinner() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Spinner()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
