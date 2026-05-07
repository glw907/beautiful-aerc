// SPDX-License-Identifier: MIT

package contacts

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds the lipgloss styles for every contacts surface: the
// i-popover, sidebar column, list, form, and detail card.
type Styles struct {
	Name       lipgloss.Style // card display name — FgBright bold
	TitleOrg   lipgloss.Style // job title / org line below name — FgBase
	Body       lipgloss.Style // address/phone values in the detail card — FgBase
	Dim        lipgloss.Style // parenthetical metadata and label suffixes — FgDim
	Rule       lipgloss.Style // separator under the name block — FgDim
	CursorRow  lipgloss.Style // selected row in list and sidebar — AccentPrimary
	GroupLabel lipgloss.Style // T9 group letter tick (e.g. "A") — AccentPrimary
	GroupCount lipgloss.Style // contact count per group — FgDim
	LetterTick lipgloss.Style // per-letter cursor tick within a T9 group — AccentPrimary
	Border     lipgloss.Style // form input border (blur state) — FgDim
	FieldFocus lipgloss.Style // form field border (focus state) — AccentPrimary
	FieldBlur  lipgloss.Style // form field border (blur state) — FgDim
	KindOn     lipgloss.Style // kind toggle selected state — AccentPrimary bold
	KindOff    lipgloss.Style // kind toggle unselected state — FgDim
	Warn       lipgloss.Style // validation error text — ColorWarning
}

// NewStyles compiles contacts Styles from a theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Name:       lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		TitleOrg:   lipgloss.NewStyle().Foreground(t.FgBase),
		Body:       lipgloss.NewStyle().Foreground(t.FgBase),
		Dim:        lipgloss.NewStyle().Foreground(t.FgDim),
		Rule:       lipgloss.NewStyle().Foreground(t.FgDim),
		CursorRow:  lipgloss.NewStyle().Foreground(t.AccentPrimary),
		GroupLabel: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		GroupCount: lipgloss.NewStyle().Foreground(t.FgDim),
		LetterTick: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Border:     lipgloss.NewStyle().Foreground(t.FgDim),
		FieldFocus: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		FieldBlur:  lipgloss.NewStyle().Foreground(t.FgDim),
		KindOn:     lipgloss.NewStyle().Foreground(t.AccentPrimary).Bold(true),
		KindOff:    lipgloss.NewStyle().Foreground(t.FgDim),
		Warn:       lipgloss.NewStyle().Foreground(t.ColorWarning),
	}
}
