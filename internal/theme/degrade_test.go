package theme

import "testing"

// signal is the tuple of non-color mechanisms a degraded state can
// carry: a marker glyph, reverse video, and a heavier border. Two
// states collide if their signals are identical.
type signal struct {
	marker  string
	reverse bool
	heavy   bool
}

// TestDegradeChannelsDistinct asserts UX-7's degrade-table
// requirement: under ANSI-16 and NO_COLOR, unread, selected,
// focused, and error each carry a distinct non-color signal (design
// language section 7's table: marker, reverse, heavy border, `!`
// gutter plus reverse), so no two collapse to the same cue once
// color drops out.
func TestDegradeChannelsDistinct(t *testing.T) {
	for _, profile := range []Profile{ProfileANSI16, ProfileNoColor} {
		th := New(true, profile)
		glyphs := th.Glyphs()

		states := map[string]signal{
			"unread":   {marker: glyphs.Unread},
			"selected": {reverse: th.Style(RoleFg, GroundSelected).GetReverse()},
			"focused":  {heavy: th.Border(BorderFocused) != th.Border(BorderDivider)},
			"error":    {marker: glyphs.ErrorGutter, reverse: th.Style(RoleError, GroundBase).GetReverse()},
		}

		seen := map[signal]string{}
		for name, sig := range states {
			if other, ok := seen[sig]; ok {
				t.Errorf("profile %v: %q and %q share signal %+v", profile, name, other, sig)
			}
			seen[sig] = name
		}

		if states["selected"].reverse != true {
			t.Errorf("profile %v: selected should carry reverse video", profile)
		}
		if states["error"].reverse != true {
			t.Errorf("profile %v: error should carry reverse video", profile)
		}
		if states["unread"].marker == "" {
			t.Errorf("profile %v: unread should carry a marker glyph", profile)
		}
		if !states["focused"].heavy {
			t.Errorf("profile %v: focused should carry a heavier border than the divider", profile)
		}
	}
}

// TestTrueColorSkipsReverse asserts the degrade signals stay off at
// full color: selection and error read through ground/role color
// alone, never reverse video, when the terminal can show it.
func TestTrueColorSkipsReverse(t *testing.T) {
	th := New(true, ProfileTrueColor)
	if th.Style(RoleFg, GroundSelected).GetReverse() {
		t.Error("truecolor selected should not use reverse video")
	}
	if th.Style(RoleError, GroundBase).GetReverse() {
		t.Error("truecolor error should not use reverse video")
	}
}
