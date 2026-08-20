// Package ui is poplar's bubbletea v2 UI layer: the root model, the
// screen registry, and the screens (technical design section 12).
// It reads the store through commands and mutates only by
// enqueueing intents; it performs no network I/O and no store
// writes.
package ui

import (
	"github.com/glw907/poplar/internal/theme"
)

// Option configures ResolveProfile beyond environment detection.
type Option func(*resolveConfig)

type resolveConfig struct {
	override    theme.Profile
	hasOverride bool
}

// WithProfileOverride forces the profile ResolveProfile returns,
// bypassing NO_COLOR/TERM/COLORTERM detection. This is the ST-3
// config-override seam: pass 2b wires the loaded config's profile
// setting through it.
func WithProfileOverride(p theme.Profile) Option {
	return func(c *resolveConfig) {
		c.override = p
		c.hasOverride = true
	}
}

// ResolveProfile resolves the runtime capability profile from env's
// NO_COLOR, TERM, and COLORTERM, in that precedence, and opts'
// override seam. It also returns DefaultDark, the isDark policy the
// first frame renders on before the terminal's own
// tea.BackgroundColorMsg answers (technical design section 12).
//
// env is a lookup function shaped like os.LookupEnv, so a caller at
// program construction passes the real process environment and a
// test passes a literal table: QA-7 takes profiles as inputs, never
// sniffed from the running process.
//
// Precedence: an override wins outright. Otherwise NO_COLOR's mere
// presence forces ProfileNoColor. TERM unset or "dumb" also forces
// ProfileNoColor, since neither promises any color support.
// COLORTERM of "truecolor" or "24bit" upgrades to ProfileTrueColor.
// Anything else resolves to ProfileANSI16, the baseline poplar
// assumes for a named, non-dumb terminal.
func ResolveProfile(env func(string) (string, bool), opts ...Option) (theme.Profile, bool) {
	var cfg resolveConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.hasOverride {
		return cfg.override, DefaultDark
	}

	if _, ok := env("NO_COLOR"); ok {
		return theme.ProfileNoColor, DefaultDark
	}

	term, _ := env("TERM")
	if term == "" || term == "dumb" {
		return theme.ProfileNoColor, DefaultDark
	}

	if colorterm, _ := env("COLORTERM"); colorterm == "truecolor" || colorterm == "24bit" {
		return theme.ProfileTrueColor, DefaultDark
	}

	return theme.ProfileANSI16, DefaultDark
}
