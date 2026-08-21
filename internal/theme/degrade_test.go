package theme

import "testing"

// signal is the tuple of non-color mechanisms a degraded state can
// carry: a marker glyph and reverse video. Two states collide if
// their signals are identical.
type signal struct {
	marker  string
	reverse bool
}

// TestDegradeChannelsDistinct asserts UX-7's degrade-table
// requirement: under ANSI-16 and NO_COLOR, unread, selected,
// focused, and error each carry a distinct non-color signal (design
// language section 7's table, amended by pass 2 ruling I1: unread =
// its marker glyph, selected = reverse video and reverse video
// alone, focused = the edge bar's distinct focused glyph, error =
// the `!` gutter marker alone), so no two collapse to the same cue
// once color drops out.
func TestDegradeChannelsDistinct(t *testing.T) {
	for _, profile := range []Profile{ProfileANSI16, ProfileNoColor} {
		th := New(true, profile)
		glyphs := th.Glyphs()

		if glyphs.EdgeBarFocused == glyphs.EdgeBarBlurred {
			t.Errorf("profile %v: EdgeBarFocused and EdgeBarBlurred must differ", profile)
		}

		states := map[string]signal{
			"unread":   {marker: glyphs.Unread},
			"selected": {reverse: th.Style(RoleFg, GroundSelected).GetReverse()},
			"focused":  {marker: glyphs.EdgeBarFocused},
			"error":    {marker: glyphs.ErrorGutter, reverse: th.Style(RoleError, GroundBase).GetReverse()},
		}

		seen := map[signal]string{}
		for name, sig := range states {
			if other, ok := seen[sig]; ok {
				t.Errorf("profile %v: %q and %q share signal %+v", profile, name, other, sig)
			}
			seen[sig] = name
		}

		if !states["selected"].reverse {
			t.Errorf("profile %v: selected should carry reverse video", profile)
		}
		if states["error"].reverse {
			t.Errorf("profile %v: error should not carry reverse video (ruling I1: reverse is selection's exclusive channel)", profile)
		}
		if states["unread"].marker == "" {
			t.Errorf("profile %v: unread should carry a marker glyph", profile)
		}
		if states["error"].marker == "" {
			t.Errorf("profile %v: error should carry the gutter marker glyph", profile)
		}
	}
}

// TestTrueColorSkipsReverse asserts the degrade signals stay off at
// full color: selection reads through ground color alone, never
// reverse video, when the terminal can show it.
func TestTrueColorSkipsReverse(t *testing.T) {
	th := New(true, ProfileTrueColor)
	if th.Style(RoleFg, GroundSelected).GetReverse() {
		t.Error("truecolor selected should not use reverse video")
	}
	if th.Style(RoleError, GroundBase).GetReverse() {
		t.Error("truecolor error should not use reverse video")
	}
}

// TestErrorGroundsDiffer is the composition test pass 2 ruling I1
// names directly: RoleError on the selection ground must read
// differently from RoleError on the base ground, in every profile,
// even though reverse video no longer belongs to RoleError itself
// (selection's GroundSelected still carries it).
func TestErrorGroundsDiffer(t *testing.T) {
	for _, p := range []Profile{ProfileTrueColor, ProfileANSI16, ProfileNoColor} {
		th := New(true, p)
		selected := th.Style(RoleError, GroundSelected)
		base := th.Style(RoleError, GroundBase)
		if !stylesDiffer(selected, base) {
			t.Errorf("profile %v: Style(RoleError, GroundSelected) == Style(RoleError, GroundBase)", p)
		}
	}
}

// TestDegradeFaintOnMuted asserts pass 2 review finding I6, narrowed
// by the pass 2 gate's real-terminal finding: "dim" (design language
// section 7's type roles) has no color-only expression under
// ProfileNoColor, so RoleFgMuted and RoleFgSubtle carry Faint there;
// RoleFg never does. ProfileANSI16 does not carry Faint on those
// roles: its slot 8 (bright black) already dims on its own, and
// Faint stacked on top of it measured 1.29:1, invisible.
// ProfileTrueColor expresses "dim" through the muted color alone.
func TestDegradeFaintOnMuted(t *testing.T) {
	noColor := New(true, ProfileNoColor)
	if !noColor.Style(RoleFgMuted, GroundBase).GetFaint() {
		t.Error("nocolor: fgMuted should be faint")
	}
	if !noColor.Style(RoleFgSubtle, GroundBase).GetFaint() {
		t.Error("nocolor: fgSubtle should be faint")
	}
	if noColor.Style(RoleFg, GroundBase).GetFaint() {
		t.Error("nocolor: fg should not be faint")
	}

	ansi16 := New(true, ProfileANSI16)
	if ansi16.Style(RoleFgMuted, GroundBase).GetFaint() {
		t.Error("ansi16: fgMuted should not be faint, slot 8 already dims")
	}
	if ansi16.Style(RoleFgSubtle, GroundBase).GetFaint() {
		t.Error("ansi16: fgSubtle should not be faint, slot 8 already dims")
	}

	th := New(true, ProfileTrueColor)
	if th.Style(RoleFgMuted, GroundBase).GetFaint() {
		t.Error("truecolor fgMuted should not use Faint")
	}
}

// TestTypeLabelDiffersFromValueUnderNoColor is I6's composition
// test: TypeLabel and TypeValue must render distinguishably even
// with no color at all.
func TestTypeLabelDiffersFromValueUnderNoColor(t *testing.T) {
	th := New(true, ProfileNoColor)
	label := th.TypeStyle(TypeLabel, GroundBase)
	value := th.TypeStyle(TypeValue, GroundBase)
	if !stylesDiffer(label, value) {
		t.Error("TypeLabel and TypeValue should differ under NO_COLOR")
	}
}
