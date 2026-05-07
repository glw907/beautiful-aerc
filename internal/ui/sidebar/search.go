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

// searchCycleMode binds Tab to the mode toggle while the shelf is in
// SearchTyping state.
var searchCycleMode = key.NewBinding(key.WithKeys("tab"))

// SearchState is the lifecycle state of the sidebar search UI.
type SearchState int

const (
	// SearchIdle: no filter, shelf shows hint row.
	SearchIdle SearchState = iota
	// SearchTyping: prompt focused. Printable runes append to query,
	// filter updates live on each keystroke.
	SearchTyping
	// SearchActive: query is live but prompt is unfocused. Normal
	// account-view key routing resumes.
	SearchActive
)

// SearchUpdatedMsg carries the live search query and mode from
// Search up to AccountTab whenever either changes in Typing state.
type SearchUpdatedMsg struct {
	Query string
	Mode  uicore.SearchMode
}

// Search is the 3-row shelf pinned to the bottom of the sidebar
// column. Owns the text input, mode toggle, and state machine for
// the search feature. Communicates with AccountTab via
// SearchUpdatedMsg during Typing. State transitions (Activate,
// Commit, Clear) are driven by direct method calls from AccountTab.
type Search struct {
	input   textinput.Model
	mode    uicore.SearchMode
	state   SearchState
	results int
	styles  Styles
	icons   uicore.IconSet
	width   int
}

// NewSearch constructs an idle search shelf at the given width. The
// textinput is created with "/" as its prompt so the rendered view
// shows "/query▏" directly without our shelf having to stitch a
// prefix in front of it.
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

// SetSize updates the shelf's width. Height is fixed at ShelfRows.
// Also clamps the embedded textinput so its View() never produces
// lines wider than the sidebar column.
func (s *Search) SetSize(width int) {
	s.width = width
	// promptOverhead is sized for the widest rendered states
	// (Typing/Active): leading "  " indent (2), search icon (2),
	// gap before prompt (1). See renderPromptRow. The idle state
	// omits the icon so it has a couple cells of unused slack, which
	// is harmless. Floor at 1 so textinput never gets a negative width.
	const promptOverhead = 5
	s.input.Width = max(1, width-promptOverhead)
}

// Activate transitions Idle → Typing and focuses the text input.
// Safe to call from any state: re-activates an Active shelf into
// Typing without losing the query.
func (s *Search) Activate() {
	s.state = SearchTyping
	s.input.Focus()
}

// Clear returns the shelf to Idle, empties the query, blurs the
// input, and resets the mode to uicore.SearchModeName.
func (s *Search) Clear() {
	s.state = SearchIdle
	s.input.Reset()
	s.input.Blur()
	s.mode = uicore.SearchModeName
	s.results = 0
}

// Commit transitions Typing → Active, leaving the query intact and
// blurring the input. Safe to call from Active (no-op).
func (s *Search) Commit() {
	s.state = SearchActive
	s.input.Blur()
}

// SetResultCount stores the most recent filter result count (thread
// count) for display in the info row.
func (s *Search) SetResultCount(n int) {
	s.results = n
}

// Update routes a bubbletea Msg through the textinput and returns
// the possibly-mutated shelf plus a Cmd that emits a
// SearchUpdatedMsg whenever the query or mode changed. Only
// meaningful in SearchTyping state.
//
// The textinput's own returned Cmd (cursor blink ticker) is dropped;
// the shelf doesn't need a blinking cursor and it makes tests
// 500ms slower per keystroke when drained synchronously.
func (s Search) Update(msg tea.Msg) (Search, tea.Cmd) {
	if s.state != SearchTyping {
		return s, nil
	}

	// Intercept Tab: cycle the mode without routing to textinput.
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

// View renders the shelf's 3 rows: blank separator, prompt/hint,
// mode/count row.
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

// renderBlankRow renders a full-width blank row using the sidebar
// background.
func (s Search) renderBlankRow() string {
	return s.styles.SidebarBg.Width(s.width).Render("")
}

// renderPromptRow renders the prompt line.
//   - Idle: shows icons.Search + " / to search" hint in dim color.
//   - Typing: shows icons.Search + textinput.View() which renders "/query▏"
//     (cursor ▏ drawn automatically because the input is Focused).
//   - Active: shows icons.Search + a manually-rendered "/query" with a
//     brighter foreground to signal "committed query." No cursor
//     because the input is Blurred.
func (s Search) renderPromptRow() string {
	if s.state == SearchIdle {
		// No icon in the idle state. In simple mode icons.Search == "/"
		// which would produce "/ / to search" (duplicated slash).
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

// renderInfoRow renders the mode badge and result count. Blank in
// idle state or when the query is empty. In typing/active with a
// non-empty query renders "[name]" or "[all]" on the left and the
// result count or "no results" on the right.
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

// formatResultCount returns the visible text for a result count.
func formatResultCount(n int) string {
	if n == 1 {
		return "1 result"
	}
	return strconv.Itoa(n) + " results"
}
