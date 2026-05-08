package compose

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/glw907/poplar/internal/catkin"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is the compose surface's projection of internal/ui.Styles.
type Styles struct {
	ErrorBanner         lipgloss.Style
	InfoBanner          lipgloss.Style
	DropdownRow         lipgloss.Style
	DropdownRowSelected lipgloss.Style
	DropdownOrg         lipgloss.Style
	FromChip            lipgloss.Style
	TidyChange          lipgloss.Style
	PickerCursor        lipgloss.Style
	PickerDim           lipgloss.Style
	PickerError         lipgloss.Style
	AttachLabel         lipgloss.Style
	AttachChip          lipgloss.Style
	AttachChipFocus     lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	chip := lipgloss.NewStyle().Foreground(t.FgDim).Background(t.BgBase)
	return Styles{
		ErrorBanner:         lipgloss.NewStyle().Foreground(t.ColorError),
		InfoBanner:          lipgloss.NewStyle().Foreground(t.FgDim),
		DropdownRow:         lipgloss.NewStyle().Foreground(t.FgBase),
		DropdownRowSelected: lipgloss.NewStyle().Foreground(t.FgBright).Background(t.AccentPrimary),
		DropdownOrg:         lipgloss.NewStyle().Foreground(t.FgDim),
		FromChip:            chip,
		TidyChange:          lipgloss.NewStyle().Foreground(t.AccentPrimary).Underline(true),
		PickerCursor:        lipgloss.NewStyle().Foreground(t.BgBase).Background(t.AccentPrimary),
		PickerDim:           lipgloss.NewStyle().Foreground(t.FgDim),
		PickerError:         lipgloss.NewStyle().Foreground(t.ColorError),
		AttachLabel:         lipgloss.NewStyle().Foreground(t.FgDim),
		AttachChip:          lipgloss.NewStyle().Foreground(t.FgBase),
		AttachChipFocus:     lipgloss.NewStyle().Foreground(t.BgBase).Background(t.AccentPrimary),
	}
}

// CatkinStyles projects compose Styles onto catkin's Styles. v1 only fills
// TidyChange so catkin emits plain text for the rest of the markdown surface.
func (s Styles) CatkinStyles() catkin.Styles {
	return catkin.Styles{TidyChange: s.TidyChange}
}
