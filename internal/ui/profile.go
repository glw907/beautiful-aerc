// Package ui is poplar's bubbletea v2 UI layer (technical design
// section 12), built out across pass 2. So far it holds the runtime
// capability-profile resolver, the background-color query pieces a
// root model's Init and Update absorb, the root model itself, and the
// four surface placeholders. Logging runs through plain log/slog
// calls against the uerr-installed process-wide default (cmd/poplar's
// startup path calls uerr.SetDefault before anything in this package
// can log); internal/ui never imports internal/uerr itself.
package ui

import (
	"strings"

	"github.com/charmbracelet/colorprofile"

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
// Precedence: an override wins outright. Otherwise NO_COLOR set to a
// non-empty value forces ProfileNoColor (no-color.org: present AND
// non-empty; a variable exported but left empty does not count).
// TERM unset or "dumb" also forces ProfileNoColor, since neither
// promises any color support. COLORTERM of "truecolor" or "24bit",
// matched case-insensitively as bubbletea's own detector does,
// upgrades to ProfileTrueColor. Anything else resolves to
// ProfileANSI16, the baseline poplar assumes for a named, non-dumb
// terminal.
func ResolveProfile(env func(string) (string, bool), opts ...Option) (theme.Profile, bool) {
	var cfg resolveConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.hasOverride {
		return cfg.override, DefaultDark
	}

	if v, ok := env("NO_COLOR"); ok && v != "" {
		return theme.ProfileNoColor, DefaultDark
	}

	term, _ := env("TERM")
	if term == "" || term == "dumb" {
		return theme.ProfileNoColor, DefaultDark
	}

	colorterm, _ := env("COLORTERM")
	switch strings.ToLower(colorterm) {
	case "truecolor", "24bit":
		return theme.ProfileTrueColor, DefaultDark
	}

	return theme.ProfileANSI16, DefaultDark
}

// mapColorProfile maps profile onto the colorprofile.Profile
// tea.WithColorProfile expects: NewProgram's own answer to "what
// color profile does the terminal actually get told to render at".
func mapColorProfile(profile theme.Profile) colorprofile.Profile {
	switch profile {
	case theme.ProfileTrueColor:
		return colorprofile.TrueColor
	case theme.ProfileNoColor:
		return colorprofile.Ascii
	default:
		return colorprofile.ANSI
	}
}
