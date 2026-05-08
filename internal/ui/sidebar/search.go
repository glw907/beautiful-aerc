package sidebar

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// searchCycleMode binds Tab to the [name]↔[all] mode toggle while typing.
var searchCycleMode = key.NewBinding(key.WithKeys("tab"))

// SearchState is the lifecycle state of the sidebar search UI.
type SearchState int

const (
	// SearchIdle has no filter; the shelf shows a hint row.
	SearchIdle SearchState = iota
	// SearchTyping focuses the prompt; the filter updates per keystroke.
	SearchTyping
	// SearchActive holds a live query with the prompt unfocused.
	SearchActive
)

// SearchUpdatedMsg carries query and mode up to AccountTab on every Typing change.
type SearchUpdatedMsg struct {
	Query string
	Mode  uicore.SearchMode
}

// Search is the 3-row shelf pinned to the bottom of the sidebar column.
// It signals AccountTab via SearchUpdatedMsg while Typing; state
// transitions come from direct method calls.
type Search struct {
	input   textinput.Model
	mode    uicore.SearchMode
	state   SearchState
	results int
	styles  Styles
	icons   uicore.IconSet
	width   int
}

// NewSearch constructs an idle search shelf. The textinput uses "/" as its
// prompt so the rendered view is "/query▏" without the shelf having to
// stitch a prefix in front.
func NewSearch(styles Styles, width int, icons uicore.IconSet) Search {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 0
	return Search{
		input:  ti,
		mode:   uicore.SearchModeName,
		state:  SearchIdle,
		styles: styles,
		icons:  icons,
		width:  width,
	}
}

func (s Search) State() SearchState      { return s.state }
func (s Search) Query() string           { return s.input.Value() }
func (s Search) Mode() uicore.SearchMode { return s.mode }
func (s Search) ResultCount() int        { return s.results }

// SetSize sets the width and clamps the textinput so View() never
// overflows the column. Height is fixed at ShelfRows.
func (s *Search) SetSize(width int) {
	s.width = width
	// Sized for the widest rendered states (Typing/Active): "  " indent (2)
	// + icon (2) + gap (1). Idle state has a couple cells of harmless slack.
	const promptOverhead = 5
	s.input.Width = max(1, width-promptOverhead)
}

// Activate moves the shelf to Typing and focuses the input; calling from Active preserves the query.
func (s *Search) Activate() {
	s.state = SearchTyping
	s.input.Focus()
}

// Clear empties the query and returns the shelf to Idle.
func (s *Search) Clear() {
	s.state = SearchIdle
	s.input.Reset()
	s.input.Blur()
	s.mode = uicore.SearchModeName
	s.results = 0
}

// Commit moves Typing → Active, blurring the input; calling from Active is a no-op.
func (s *Search) Commit() {
	s.state = SearchActive
	s.input.Blur()
}

// SetResultCount stores the latest thread-count for the info row.
func (s *Search) SetResultCount(n int) {
	s.results = n
}

// Update routes a Msg through the textinput while in Typing state and
// emits SearchUpdatedMsg on query or mode change. The textinput's
// blink-ticker Cmd is dropped; a blinking cursor isn't wanted here and
// draining it synchronously adds 500ms per keystroke in tests.
func (s Search) Update(msg tea.Msg) (Search, tea.Cmd) {
	if s.state != SearchTyping {
		return s, nil
	}

	if k, ok := msg.(tea.KeyMsg); ok && key.Matches(k, searchCycleMode) {
		if s.mode == uicore.SearchModeName {
			s.mode = uicore.SearchModeAll
		} else {
			s.mode = uicore.SearchModeName
		}
		query := s.input.Value()
		mode := s.mode
		return s, func() tea.Msg {
			return SearchUpdatedMsg{Query: query, Mode: mode}
		}
	}

	prev := s.input.Value()
	s.input, _ = s.input.Update(msg)
	cur := s.input.Value()
	if cur == prev {
		return s, nil
	}
	query := cur
	mode := s.mode
	return s, func() tea.Msg {
		return SearchUpdatedMsg{Query: query, Mode: mode}
	}
}

func (s Search) View() string {
	if s.width <= 0 {
		return ""
	}
	return strings.Join([]string{
		s.renderBlankRow(),
		s.renderPromptRow(),
		s.renderInfoRow(),
	}, "\n")
}

func (s Search) renderBlankRow() string {
	return s.styles.SidebarBg.Width(s.width).Render("")
}

// renderPromptRow draws the prompt line. Idle drops the icon because in
// simple mode icons.Search is "/" and would double up against the hint.
func (s Search) renderPromptRow() string {
	if s.state == SearchIdle {
		hint := uicore.ApplyBg(s.styles.SearchHint, s.styles.SidebarBg).Render(" / to search")
		content := s.styles.SidebarBg.Render("  ") + hint
		return uicore.FillRowToWidth(content, s.width, s.styles.SidebarBg)
	}

	iconStyle := s.styles.SearchIcon
	if s.state == SearchTyping {
		iconStyle = iconStyle.Foreground(s.styles.SearchResultCount.GetForeground())
	}
	icon := uicore.ApplyBg(iconStyle, s.styles.SidebarBg).Render(s.icons.Search)

	var prompt string
	if s.state == SearchTyping {
		prompt = uicore.ApplyBg(s.styles.SearchPrompt, s.styles.SidebarBg).Render(s.input.View())
	} else {
		text := "/" + s.input.Value()
		prompt = uicore.ApplyBg(s.styles.SidebarAccount, s.styles.SidebarBg).Render(text)
	}

	content := s.styles.SidebarBg.Render("  ") + icon + s.styles.SidebarBg.Render(" ") + prompt
	return uicore.FillRowToWidth(content, s.width, s.styles.SidebarBg)
}

// renderInfoRow draws the mode badge and result count. Blank when idle
// or query is empty.
func (s Search) renderInfoRow() string {
	if s.state == SearchIdle || s.Query() == "" {
		return s.renderBlankRow()
	}
	modeLabel := "[name]"
	if s.mode == uicore.SearchModeAll {
		modeLabel = "[all]"
	}
	mode := uicore.ApplyBg(s.styles.SearchModeBadge, s.styles.SidebarBg).Render(modeLabel)

	var countText string
	var countStyled string
	if s.results == 0 {
		countText = "no results"
		countStyled = uicore.ApplyBg(s.styles.SearchNoResults, s.styles.SidebarBg).Render(countText)
	} else {
		countText = formatResultCount(s.results)
		countStyled = uicore.ApplyBg(s.styles.SearchResultCount, s.styles.SidebarBg).Render(countText)
	}

	indent := s.styles.SidebarBg.Render("  ")
	margin := s.styles.SidebarBg.Render(" ")
	contentCells := 2 + lipgloss.Width(modeLabel) + lipgloss.Width(countText) + 1
	gap := max(1, s.width-contentCells)
	content := indent + mode + s.styles.SidebarBg.Render(strings.Repeat(" ", gap)) + countStyled + margin
	return uicore.FillRowToWidth(content, s.width, s.styles.SidebarBg)
}

func formatResultCount(n int) string {
	if n == 1 {
		return "1 result"
	}
	return strconv.Itoa(n) + " results"
}
