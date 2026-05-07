package messagelist

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		MsgListBg:            lipgloss.NewStyle().Background(t.BgBase),
		MsgListSelected:      lipgloss.NewStyle().Background(t.BgSelection),
		MsgListCursor:        lipgloss.NewStyle().Foreground(t.AccentPrimary),
		MsgListUnreadSender:  lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		MsgListUnreadSubject: lipgloss.NewStyle().Foreground(t.FgBright),
		MsgListReadSender:    lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListReadSubject:   lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListDate:          lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListIconUnread:    lipgloss.NewStyle().Foreground(t.FgBright),
		MsgListIconRead:      lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListFlagFlagged:   lipgloss.NewStyle().Foreground(t.ColorWarning),
		MsgListThreadPrefix:  lipgloss.NewStyle().Foreground(t.FgDim),
		MsgListPlaceholder:   lipgloss.NewStyle().Foreground(t.FgDim),
	}
}
