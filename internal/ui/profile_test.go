package ui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/glw907/poplar/internal/theme"
)

// TestResolveProfile is QA-7's env-combination table: profiles are
// test inputs given as a literal lookup table, never sniffed from
// the running process's environment.
func TestResolveProfile(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		opts []Option
		want theme.Profile
	}{
		{
			name: "named terminal with no other signal defaults to ANSI16",
			env:  map[string]string{"TERM": "xterm"},
			want: theme.ProfileANSI16,
		},
		{
			name: "unset TERM degrades to NoColor",
			env:  map[string]string{},
			want: theme.ProfileNoColor,
		},
		{
			name: "TERM=dumb degrades to NoColor even with COLORTERM set",
			env:  map[string]string{"TERM": "dumb", "COLORTERM": "truecolor"},
			want: theme.ProfileNoColor,
		},
		{
			name: "COLORTERM=truecolor upgrades to TrueColor",
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "truecolor"},
			want: theme.ProfileTrueColor,
		},
		{
			name: "COLORTERM=24bit upgrades to TrueColor",
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "24bit"},
			want: theme.ProfileTrueColor,
		},
		{
			name: "COLORTERM=TrueColor upgrades case-insensitively",
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "TrueColor"},
			want: theme.ProfileTrueColor,
		},
		{
			name: "COLORTERM with an unrecognized value does not upgrade",
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "yes"},
			want: theme.ProfileANSI16,
		},
		{
			name: "NO_COLOR wins over COLORTERM=truecolor",
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "truecolor", "NO_COLOR": "1"},
			want: theme.ProfileNoColor,
		},
		{
			name: "NO_COLOR set but empty does not disable color",
			env:  map[string]string{"TERM": "xterm", "NO_COLOR": ""},
			want: theme.ProfileANSI16,
		},
		{
			name: "NO_COLOR=0 still disables color",
			env:  map[string]string{"TERM": "xterm", "NO_COLOR": "0"},
			want: theme.ProfileNoColor,
		},
		{
			name: "an override wins over every environment signal",
			env:  map[string]string{"NO_COLOR": "1"},
			opts: []Option{WithProfileOverride(theme.ProfileTrueColor)},
			want: theme.ProfileTrueColor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				v, ok := tt.env[key]
				return v, ok
			}
			profile, isDark := ResolveProfile(lookup, tt.opts...)
			if profile != tt.want {
				t.Errorf("ResolveProfile() profile = %v, want %v", profile, tt.want)
			}
			if isDark != DefaultDark {
				t.Errorf("ResolveProfile() isDark = %v, want DefaultDark (%v)", isDark, DefaultDark)
			}
		})
	}
}

// TestMapColorProfile pins CARRY 1's mapping, all three cases: without
// it, tea.WithColorProfile is never told what ResolveProfile already
// resolved, and bubbletea's terminal auto-detection re-downsamples
// the theme's explicit values.
func TestMapColorProfile(t *testing.T) {
	tests := []struct {
		in   theme.Profile
		want colorprofile.Profile
	}{
		{theme.ProfileTrueColor, colorprofile.TrueColor},
		{theme.ProfileANSI16, colorprofile.ANSI},
		{theme.ProfileNoColor, colorprofile.Ascii},
	}
	for _, tt := range tests {
		if got := mapColorProfile(tt.in); got != tt.want {
			t.Errorf("mapColorProfile(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
