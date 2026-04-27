package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

// SidebarSearch is the 3-row shelf pinned to the bottom of the
// sidebar column. Owns the text input, mode toggle, and state
// machine for the search feature. Communicates with AccountTab via
// SearchUpdatedMsg during Typing; state transitions (Activate,
// Commit, Clear) are driven by direct method calls from AccountTab.
type SidebarSearch struct {
	input   textinput.Model
	mode    SearchMode
	state   SearchState
	results int
	styles  Styles
	width   int
}

// NewSidebarSearch constructs an idle search shelf at the given
// width. The textinput is created with "/" as its prompt so the
// rendered view shows "/query▏" directly without our shelf having
// to stitch a prefix in front of it.
func NewSidebarSearch(styles Styles, width int) SidebarSearch {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 0
	return SidebarSearch{
		input:  ti,
		mode:   SearchModeName,
		state:  SearchIdle,
		styles: styles,
		width:  width,
	}
}

func (s SidebarSearch) State() SearchState { return s.state }
func (s SidebarSearch) Query() string      { return s.input.Value() }
func (s SidebarSearch) Mode() SearchMode   { return s.mode }

// SetSize updates the shelf's width. Height is fixed at
// searchShelfRows. Also clamps the embedded textinput so its
// View() never produces lines wider than the sidebar column.
func (s *SidebarSearch) SetSize(width int) {
	s.width = width
	// Reserve cells for the leading "  " indent (2), the search
	// icon (2), and the gap before the prompt (1) — see
	// renderPromptRow. Floor at 1 so textinput never gets a
	// negative width.
	const promptOverhead = 5
	s.input.Width = max(1, width-promptOverhead)
}

// Activate transitions Idle → Typing and focuses the text input.
// Safe to call from any state: re-activates an Active shelf into
// Typing without losing the query.
func (s *SidebarSearch) Activate() {
	s.state = SearchTyping
	s.input.Focus()
}

// Clear returns the shelf to Idle, empties the query, blurs the
// input, and resets the mode to SearchModeName.
func (s *SidebarSearch) Clear() {
	s.state = SearchIdle
	s.input.Reset()
	s.input.Blur()
	s.mode = SearchModeName
	s.results = 0
}

// Commit transitions Typing → Active, leaving the query intact and
// blurring the input. Safe to call from Active (no-op).
func (s *SidebarSearch) Commit() {
	s.state = SearchActive
	s.input.Blur()
}

// SetResultCount stores the most recent filter result count (thread
// count) for display in the info row.
func (s *SidebarSearch) SetResultCount(n int) {
	s.results = n
}

// Update routes a bubbletea Msg through the textinput and returns
// the possibly-mutated shelf plus a Cmd that emits a
// SearchUpdatedMsg whenever the query or mode changed. Only
// meaningful in SearchTyping state.
//
// The textinput's own returned Cmd (cursor blink ticker) is dropped
// — the shelf doesn't need a blinking cursor and it makes tests
// 500ms slower per keystroke when drained synchronously.
func (s SidebarSearch) Update(msg tea.Msg) (SidebarSearch, tea.Cmd) {
	if s.state != SearchTyping {
		return s, nil
	}

	// Intercept Tab: cycle the mode without routing to textinput.
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyTab {
		if s.mode == SearchModeName {
			s.mode = SearchModeAll
		} else {
			s.mode = SearchModeName
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
func (s SidebarSearch) View() string {
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
func (s SidebarSearch) renderBlankRow() string {
	return s.styles.SidebarBg.Width(s.width).Render("")
}

// renderPromptRow renders the prompt line.
//   - Idle: shows "󰍉 / to search" hint in dim color.
//   - Typing: shows "󰍉" + textinput.View() which renders "/query▏"
//     (cursor ▏ drawn automatically because the input is Focused).
//   - Active: shows "󰍉" + a manually-rendered "/query" with a
//     brighter foreground to signal "committed query." No cursor
//     because the input is Blurred.
func (s SidebarSearch) renderPromptRow() string {
	if s.state == SearchIdle {
		icon := applyBg(s.styles.SearchIcon, s.styles.SidebarBg).Render("󰍉")
		hint := applyBg(s.styles.SearchHint, s.styles.SidebarBg).Render(" / to search")
		content := s.styles.SidebarBg.Render("  ") + icon + hint
		return fillRowToWidth(content, s.width, s.styles.SidebarBg)
	}

	iconStyle := s.styles.SearchIcon
	if s.state == SearchTyping {
		iconStyle = iconStyle.Foreground(s.styles.SearchResultCount.GetForeground())
	}
	icon := applyBg(iconStyle, s.styles.SidebarBg).Render("󰍉")

	var prompt string
	if s.state == SearchTyping {
		prompt = applyBg(s.styles.SearchPrompt, s.styles.SidebarBg).Render(s.input.View())
	} else {
		text := "/" + s.input.Value()
		prompt = applyBg(s.styles.SidebarAccount, s.styles.SidebarBg).Render(text)
	}

	content := s.styles.SidebarBg.Render("  ") + icon + s.styles.SidebarBg.Render(" ") + prompt
	return fillRowToWidth(content, s.width, s.styles.SidebarBg)
}

// renderInfoRow renders the mode badge and result count. Blank in
// idle state or when the query is empty; in typing/active with a
// non-empty query renders "[name]" or "[all]" on the left and the
// result count or "no results" on the right.
func (s SidebarSearch) renderInfoRow() string {
	if s.state == SearchIdle || s.Query() == "" {
		return s.renderBlankRow()
	}
	modeLabel := "[name]"
	if s.mode == SearchModeAll {
		modeLabel = "[all]"
	}
	mode := applyBg(s.styles.SearchModeBadge, s.styles.SidebarBg).Render(modeLabel)

	var countText string
	var countStyled string
	if s.Query() != "" && s.results == 0 {
		countText = "no results"
		countStyled = applyBg(s.styles.SearchNoResults, s.styles.SidebarBg).Render(countText)
	} else {
		countText = formatResultCount(s.results)
		countStyled = applyBg(s.styles.SearchResultCount, s.styles.SidebarBg).Render(countText)
	}

	indent := s.styles.SidebarBg.Render("  ")
	margin := s.styles.SidebarBg.Render(" ")
	contentCells := 2 + runewidth.StringWidth(modeLabel) + runewidth.StringWidth(countText) + 1
	gap := max(1, s.width-contentCells)
	content := indent + mode + s.styles.SidebarBg.Render(strings.Repeat(" ", gap)) + countStyled + margin
	return fillRowToWidth(content, s.width, s.styles.SidebarBg)
}

// formatResultCount returns the visible text for a result count.
func formatResultCount(n int) string {
	if n == 1 {
		return "1 result"
	}
	return strconv.Itoa(n) + " results"
}
