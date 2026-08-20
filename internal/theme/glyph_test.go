package theme

import (
	"reflect"
	"testing"
)

// TestGlyphsProfileSelection asserts Glyphs resolves to the full
// Unicode set at ProfileTrueColor and the ASCII fallback at
// ProfileANSI16 and ProfileNoColor.
func TestGlyphsProfileSelection(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    GlyphSet
	}{
		{"truecolor", ProfileTrueColor, fullGlyphs},
		{"ansi16", ProfileANSI16, asciiGlyphs},
		{"nocolor", ProfileNoColor, asciiGlyphs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(true, tt.profile).Glyphs(); got != tt.want {
				t.Errorf("Glyphs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestEveryGlyphHasASCIIFallback walks GlyphSet's fields and asserts
// each carries a non-empty, all-ASCII fallback string (the
// acceptance criterion: every glyph token degrades under
// ANSI-16/NO_COLOR).
func TestEveryGlyphHasASCIIFallback(t *testing.T) {
	rv := reflect.ValueOf(asciiGlyphs)
	rt := rv.Type()
	for i := range rv.NumField() {
		name := rt.Field(i).Name
		value := rv.Field(i).String()
		if value == "" {
			t.Errorf("field %s: empty ASCII fallback", name)
			continue
		}
		for _, r := range value {
			if r > 0x7f {
				t.Errorf("field %s: fallback %q is not ASCII", name, value)
				break
			}
		}
	}
}

// TestFullGlyphsCarryUnicode asserts the truecolor tier is richer
// than the ASCII fallback for the glyphs the design language gives
// a distinct Unicode form.
func TestFullGlyphsCarryUnicode(t *testing.T) {
	rv := reflect.ValueOf(fullGlyphs)
	rt := rv.Type()
	asciiRV := reflect.ValueOf(asciiGlyphs)
	for i := range rv.NumField() {
		name := rt.Field(i).Name
		full := rv.Field(i).String()
		if full == asciiRV.Field(i).String() {
			// ErrorGutter is plain "!" at both tiers by design.
			continue
		}
		hasNonASCII := false
		for _, r := range full {
			if r > 0x7f {
				hasNonASCII = true
				break
			}
		}
		if !hasNonASCII {
			t.Errorf("field %s: full glyph %q carries no non-ASCII rune despite differing from its fallback", name, full)
		}
	}
}
