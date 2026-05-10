package wizard

import (
	_ "embed"
	"strings"

	"charm.land/lipgloss/v2"
)

// LogoART is the cbonsai artifact, committed for a future logo swap.
// renderLogo currently emits a typographic wordmark; flipping the
// call site to render LogoART (or another asset) is a one-line
// change.
//
//go:embed logo_artifact.ans
var LogoART string

func renderLogo(s Styles) string {
	rule := s.Rule.Render(strings.Repeat("─", 24))
	wordmark := s.Wordmark.Render("  p o p l a r  ")
	tagline := s.Tagline.Render("a terminal email client")
	return lipgloss.JoinVertical(lipgloss.Center, rule, wordmark, rule, "", tagline)
}
