package ui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// newLoadedTab builds an AccountTab and runs the initial Cmd chain so
// that the sidebar and message list are populated. Use this in tests
// that want to exercise post-load state.
func newLoadedTab(t *testing.T, width, height int) AccountTab {
	t.Helper()
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: width, Height: height})

	// Resolve the Init Cmd to drive the tab into its post-load state.
	msg := runCmd(tab.Init())
	tab, cmd := tab.updateTab(msg)
	// selectionChangedCmds emits folderChangedCmd + openFolderCmd (batch).
	// Drain the full two-hop chain so the message list gets seeded:
	// openFolderCmd → folderQueryDoneMsg → fetchHeadersCmd → headersAppliedMsg.
	drain(t, &tab, cmd)
	return tab
}

// runCmd executes a tea.Cmd synchronously and returns its message.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// drain walks a tea.Cmd tree and feeds every resulting non-App message
// back into the tab's updateTab. It handles tea.BatchMsg fan-outs and
// follows the two-hop folder-load chain (openFolderCmd returns
// folderQueryDoneMsg, which causes fetchHeadersCmd, which returns
// headersAppliedMsg). The depth limit of 8 hops is generous; normal
// load paths are at most 2 hops deep.
func drain(t *testing.T, tab *AccountTab, cmd tea.Cmd) {
	t.Helper()
	drainDepth(t, tab, cmd, 8)
}

func drainDepth(t *testing.T, tab *AccountTab, cmd tea.Cmd, depth int) {
	t.Helper()
	if cmd == nil || depth == 0 {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			inner := sub()
			if _, isApp := inner.(FolderChangedMsg); isApp {
				// FolderChangedMsg is an App-level message; the tab
				// has nothing to do with it.
				continue
			}
			newTab, follow := tab.updateTab(inner)
			*tab = newTab
			drainDepth(t, tab, follow, depth-1)
		}
		return
	}
	if _, isApp := msg.(FolderChangedMsg); isApp {
		return
	}
	newTab, follow := tab.updateTab(msg)
	*tab = newTab
	drainDepth(t, tab, follow, depth-1)
}

func TestAccountTab(t *testing.T) {
	t.Run("title returns folder name after load", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		if tab.Title() != "Inbox" {
			t.Errorf("Title() = %q, want Inbox", tab.Title())
		}
	})

	t.Run("view renders two panels with divider", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		result := tab.View()
		if !strings.Contains(result, "│") {
			t.Error("missing panel divider")
		}
	})

	t.Run("view shows account name", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		view := stripANSI(tab.View())
		if !strings.Contains(view, "geoff@907.life") {
			t.Error("sidebar should show account name")
		}
	})

	t.Run("view renders folder names", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		view := tab.View()
		plain := stripANSI(view)
		for _, name := range []string{"Inbox", "Drafts", "Sent", "Archive", "Spam", "Trash"} {
			if !strings.Contains(plain, name) {
				t.Errorf("missing folder %q in sidebar", name)
			}
		}
	})

	t.Run("J/K navigates sidebar", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.sidebar.SelectedFolder() != "Drafts" {
			t.Errorf("after J, selected = %q, want Drafts", tab.sidebar.SelectedFolder())
		}
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
		if tab.sidebar.SelectedFolder() != "Inbox" {
			t.Errorf("after K, selected = %q, want Inbox", tab.sidebar.SelectedFolder())
		}
	})

	t.Run("title tracks selected folder", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 20)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.Title() != "Drafts" {
			t.Errorf("Title() = %q, want Drafts", tab.Title())
		}
	})

	t.Run("j/k navigates the message list", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		if tab.msglist.Selected() != 1 {
			t.Errorf("after j, msglist.Selected() = %d, want 1", tab.msglist.Selected())
		}
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		if tab.msglist.Selected() != 0 {
			t.Errorf("after k, msglist.Selected() = %d, want 0", tab.msglist.Selected())
		}
	})

	t.Run("G jumps message list to bottom", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		want := tab.msglist.Count() - 1
		if tab.msglist.Selected() != want {
			t.Errorf("after G, msglist.Selected() = %d, want %d",
				tab.msglist.Selected(), want)
		}
	})

	t.Run("g jumps message list to top", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
		if tab.msglist.Selected() != 0 {
			t.Errorf("after g, msglist.Selected() = %d, want 0", tab.msglist.Selected())
		}
	})

	t.Run("placeholder is gone", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		view := stripANSI(tab.View())
		if strings.Contains(view, "Message List") {
			t.Error("expected message list placeholder to be replaced with real component")
		}
	})

	t.Run("real message subjects appear", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		view := stripANSI(tab.View())
		if !strings.Contains(view, "Alice Johnson") {
			t.Error("expected first mock sender to be visible in account tab view")
		}
	})

	t.Run("window size", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 40)
		if tab.width != 120 || tab.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", tab.width, tab.height)
		}
	})
}

// Cmd-dispatch tests: verify the Elm-style flow at the message level.

func TestAccountTabInit_ReturnsFoldersCmd(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	msg := runCmd(tab.Init())
	if _, ok := msg.(foldersLoadedMsg); !ok {
		t.Fatalf("expected foldersLoadedMsg from Init, got %T", msg)
	}
}

func TestAccountTab_foldersLoadedSeedsSidebar(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	folders, _ := backend.ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	if len(tab.sidebar.entries) == 0 {
		t.Fatal("expected sidebar to be seeded")
	}
	if cmd == nil {
		t.Fatal("expected a follow-up cmd to load the initial folder")
	}
	msg := runCmd(cmd)
	switch msg.(type) {
	case folderQueryDoneMsg, headersAppliedMsg, tea.BatchMsg, FolderChangedMsg:
	default:
		t.Fatalf("expected folderQueryDoneMsg/headersAppliedMsg/BatchMsg/FolderChangedMsg, got %T", msg)
	}
}

func TestAccountTab_headersAppliedSeedsMsglist(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
	msgs := []mail.MessageInfo{
		{UID: "1", Subject: "hello", From: "a", Date: "now"},
	}
	tab, _ = tab.updateTab(headersAppliedMsg{name: "Inbox", msgs: msgs})
	if tab.msglist.Count() != 1 {
		t.Fatalf("expected msglist count 1, got %d", tab.msglist.Count())
	}
}

func TestAccountTab_PerFolderThreadingOverride(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()

	uiCfg := config.DefaultUIConfig()
	uiCfg.Folders["Inbox"] = config.FolderConfig{
		Threading:    false,
		ThreadingSet: true,
	}

	tab := NewAccountTab(styles, theme.Nord, backend, uiCfg)
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
	folders, _ := backend.ListFolders()
	tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})

	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "a", Date: "Apr 5", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Subject: "re: a", Date: "Apr 6", Flags: mail.FlagSeen},
	}
	tab, _ = tab.updateTab(headersAppliedMsg{name: "Inbox", msgs: msgs})

	if got := visibleRowCount(tab.msglist); got != 2 {
		t.Fatalf("flat display visible rows = %d, want 2 (no thread tree)", got)
	}
	for i, r := range tab.msglist.rows {
		if !r.isThreadRoot {
			t.Errorf("rows[%d] isThreadRoot = false, want true under threading=false", i)
		}
	}
}

func TestAccountTabFoldKeys(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)

	if got, want := visibleRowCount(tab.msglist), 14; got != want {
		t.Fatalf("initial visible rows = %d, want %d", got, want)
	}

	t.Run("F folds all threads", func(t *testing.T) {
		tab2, _ := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		if got := visibleRowCount(tab2.msglist); got != 11 {
			t.Errorf("after F, visible = %d, want 11", got)
		}
	})

	t.Run("U unfolds all threads after F", func(t *testing.T) {
		tab2, _ := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
		if got := visibleRowCount(tab2.msglist); got != 14 {
			t.Errorf("after F then U, visible = %d, want 14", got)
		}
	})

	t.Run("Space toggles fold under cursor", func(t *testing.T) {
		// Use a fresh tab to avoid aliasing the folded map with the F subtest.
		tabS := newLoadedTab(t, 120, 30)
		var t1Idx int = -1
		for i, r := range tabS.msglist.rows {
			if r.isThreadRoot && r.msg.ThreadID == "T1" {
				t1Idx = i
				break
			}
		}
		if t1Idx < 0 {
			t.Fatal("T1 root not found in displayRows")
		}
		tab2 := tabS
		for i := 0; i < t1Idx; i++ {
			tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		}
		tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeySpace})
		if got := visibleRowCount(tab2.msglist); got != 11 {
			t.Errorf("after Space on T1 root, visible = %d, want 11", got)
		}
	})
}

func TestAccountTab_JDispatchesFolderLoad(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	_, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if cmd == nil {
		t.Fatal("expected J to dispatch a Cmd")
	}
	msg := runCmd(cmd)
	switch m := msg.(type) {
	case folderQueryDoneMsg, headersAppliedMsg, FolderChangedMsg:
	case tea.BatchMsg:
		if len(m) == 0 {
			t.Fatal("empty batch")
		}
	default:
		t.Fatalf("unexpected cmd result: %T", msg)
	}
}

// drainSearch unwraps a Cmd from SidebarSearch.Update through any
// tea.BatchMsg envelope and feeds the SearchUpdatedMsg back into the
// tab so the filter takes effect. Use after typing a key during
// search.
func drainSearch(t *testing.T, tab *AccountTab, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if upd, ok := msg.(SearchUpdatedMsg); ok {
		newTab, _ := tab.updateTab(upd)
		*tab = newTab
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			inner := sub()
			if upd, ok := inner.(SearchUpdatedMsg); ok {
				newTab, _ := tab.updateTab(upd)
				*tab = newTab
			}
		}
	}
}

func TestAccountTabSearchShelf(t *testing.T) {
	t.Run("view renders the search hint at the bottom of the sidebar", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		view := stripANSI(tab.View())
		if !strings.Contains(view, "/ to search") {
			t.Error("sidebar should show '/ to search' hint")
		}
	})

	t.Run("search hint is in the last few rows of the sidebar column", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		lines := strings.Split(stripANSI(tab.View()), "\n")
		hintRow := -1
		for i, line := range lines {
			if strings.Contains(line, "/ to search") {
				hintRow = i
				break
			}
		}
		if hintRow < 0 {
			t.Fatal("hint not found in view")
		}
		contentRows := len(lines)
		if hintRow < contentRows-3 || hintRow >= contentRows {
			t.Errorf("hint row %d not in bottom shelf (content rows: %d)", hintRow, contentRows)
		}
	})
}

func TestAccountTabSearchActivation(t *testing.T) {
	t.Run("/ in Idle activates search", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		if tab.sidebarSearch.State() != SearchTyping {
			t.Errorf("state after / = %v, want SearchTyping", tab.sidebarSearch.State())
		}
	})

	t.Run("/ in Idle does not start filtering yet (empty query)", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		rowCountBefore := len(tab.msglist.rows)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		if got := len(tab.msglist.rows); got != rowCountBefore {
			t.Errorf("row count after / = %d, want %d (no filter yet)", got, rowCountBefore)
		}
	})
}

func TestAccountTabSearchFilter(t *testing.T) {
	t.Run("typing during search filters the message list", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		rowsBefore := len(tab.msglist.rows)

		for _, r := range []rune{'a', 'l', 'i'} {
			var cmd tea.Cmd
			tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			drainSearch(t, &tab, cmd)
		}

		if got := len(tab.msglist.rows); got >= rowsBefore {
			t.Errorf("row count after typing = %d, want < %d", got, rowsBefore)
		}
	})

	t.Run("SearchUpdatedMsg directly sets the filter", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		rowsBefore := len(tab.msglist.rows)
		tab, _ = tab.updateTab(SearchUpdatedMsg{Query: "alice", Mode: SearchModeName})
		if got := len(tab.msglist.rows); got >= rowsBefore {
			t.Errorf("row count after SearchUpdatedMsg = %d, want < %d", got, rowsBefore)
		}
	})

	t.Run("SearchUpdatedMsg feeds the count back to SidebarSearch", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(SearchUpdatedMsg{Query: "alice", Mode: SearchModeName})
		if tab.sidebarSearch.results != tab.msglist.FilterResultCount() {
			t.Errorf("sidebarSearch.results = %d, want %d (mirrors FilterResultCount)",
				tab.sidebarSearch.results, tab.msglist.FilterResultCount())
		}
	})
}

func TestAccountTabSearchCommitClear(t *testing.T) {
	t.Run("Enter in Typing transitions shelf to Active", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		if tab.sidebarSearch.State() != SearchActive {
			t.Errorf("state after Enter = %v, want SearchActive", tab.sidebarSearch.State())
		}
	})

	t.Run("Enter keeps the filter live (query preserved)", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		filteredRows := len(tab.msglist.rows)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		if got := len(tab.msglist.rows); got != filteredRows {
			t.Errorf("row count after Enter = %d, want %d (filter preserved)", got, filteredRows)
		}
	})

	t.Run("Esc in Typing clears the filter and returns to Idle", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		rowsBefore := len(tab.msglist.rows)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEsc})
		if tab.sidebarSearch.State() != SearchIdle {
			t.Errorf("state after Esc = %v, want SearchIdle", tab.sidebarSearch.State())
		}
		if got := len(tab.msglist.rows); got != rowsBefore {
			t.Errorf("row count after Esc = %d, want %d (full restore)", got, rowsBefore)
		}
	})

	t.Run("Esc in Active clears the filter", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		rowsBefore := len(tab.msglist.rows)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEsc})
		if tab.sidebarSearch.State() != SearchIdle {
			t.Errorf("state after Esc in Active = %v, want SearchIdle", tab.sidebarSearch.State())
		}
		if got := len(tab.msglist.rows); got != rowsBefore {
			t.Errorf("row count after Esc in Active = %d, want %d", got, rowsBefore)
		}
	})
}

func TestAccountTabSearchFolderJump(t *testing.T) {
	t.Run("J during Active clears the search", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		if tab.sidebarSearch.State() != SearchActive {
			t.Fatalf("setup: state = %v, want SearchActive", tab.sidebarSearch.State())
		}
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.sidebarSearch.State() != SearchIdle {
			t.Errorf("state after J = %v, want SearchIdle", tab.sidebarSearch.State())
		}
	})

	t.Run("K during Active clears the search", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
		if tab.sidebarSearch.State() != SearchIdle {
			t.Errorf("state after K = %v, want SearchIdle", tab.sidebarSearch.State())
		}
	})
}

func TestAccountTabSearchFoldNoOp(t *testing.T) {
	t.Run("Space during Active does not crash and does not exit search", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		if tab.sidebarSearch.State() != SearchActive {
			t.Errorf("state after Space = %v, want SearchActive", tab.sidebarSearch.State())
		}
	})

	t.Run("F during Active does not crash", func(t *testing.T) {
		tab := newLoadedTab(t, 80, 30)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		var cmd tea.Cmd
		tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		drainSearch(t, &tab, cmd)
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		if tab.sidebarSearch.State() != SearchActive {
			t.Errorf("state after F = %v, want SearchActive", tab.sidebarSearch.State())
		}
	})
}

// Viewer integration: enter opens, search/folder-jumps inert while open.

func TestAccountTab_EnterOpensViewer(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	if tab.viewer.IsOpen() {
		t.Fatal("viewer should start closed")
	}
	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	if !tab.viewer.IsOpen() {
		t.Fatal("Enter must open the viewer")
	}
	if cmd == nil {
		t.Fatal("Enter must produce a Cmd batch (load + opened + spinner)")
	}
}

func TestAccountTab_EnterMarksRead(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	// First message in mock fixture (UID 1) is unread.
	first, ok := tab.msglist.SelectedMessage()
	if !ok {
		t.Fatal("expected a selection")
	}
	if first.Flags&mail.FlagSeen != 0 {
		t.Fatalf("test fixture broke: UID %s should be unread", first.UID)
	}
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	after, _ := tab.msglist.SelectedMessage()
	if after.Flags&mail.FlagSeen == 0 {
		t.Errorf("Enter should optimistically mark seen on UID %s", after.UID)
	}
}

func TestAccountTab_EnterEmptyFolderNoOp(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
	tab, _ = tab.updateTab(headersAppliedMsg{name: "Inbox", msgs: nil})
	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.viewer.IsOpen() {
		t.Error("Enter on empty folder must not open viewer")
	}
	if cmd != nil {
		t.Errorf("Enter on empty folder must not emit a Cmd")
	}
}

func TestAccountTab_SearchKeysInertWhileViewerOpen(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	if !tab.viewer.IsOpen() {
		t.Fatal("viewer should be open")
	}
	// `/` while viewer open should not activate the search shelf.
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if tab.sidebarSearch.State() != SearchIdle {
		t.Errorf("/ while viewer open activated search; state = %v", tab.sidebarSearch.State())
	}
}

func TestAccountTab_FolderJumpInertWhileViewerOpen(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	startFolder := tab.Title()
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	if tab.Title() != startFolder {
		t.Errorf("J while viewer open changed folder to %q (was %q)", tab.Title(), startFolder)
	}
}

func TestAccountTab_FolderJumpKeys(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	cases := []struct {
		key       string
		canonical string
	}{
		{"D", "Drafts"},
		{"S", "Sent"},
		{"A", "Archive"},
		{"T", "Trash"},
		{"X", "Spam"},
		{"I", "Inbox"},
	}
	for _, tc := range cases {
		t.Run(tc.key+" jumps to "+tc.canonical, func(t *testing.T) {
			tab2, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
			got := tab2.sidebar.entries[tab2.sidebar.selected].cf.Canonical
			if got != tc.canonical {
				t.Errorf("after %q, selected canonical = %q, want %q", tc.key, got, tc.canonical)
			}
			if cmd == nil {
				t.Errorf("after %q, expected a Cmd to load the new folder", tc.key)
			}
			tab = tab2
		})
	}
}

func TestAccountTab_FolderJumpUnknownFolderNoOp(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
	tab, _ = tab.updateTab(foldersLoadedMsg{folders: []mail.Folder{
		{Name: "Inbox", Role: "inbox"},
	}})
	startFolder := tab.Title()
	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if tab.Title() != startFolder {
		t.Errorf("D with no Drafts folder changed Title to %q (was %q)", tab.Title(), startFolder)
	}
	if cmd != nil {
		t.Errorf("D with no Drafts folder should not emit a Cmd; got %T", cmd)
	}
}

func TestAccountTab_FolderJumpClearsSearch(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)

	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	drainSearch(t, &tab, cmd)
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.sidebarSearch.State() != SearchActive {
		t.Fatalf("expected SearchActive after Enter; got %v", tab.sidebarSearch.State())
	}

	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if tab.sidebarSearch.State() != SearchIdle {
		t.Errorf("after D folder jump, search state = %v, want SearchIdle", tab.sidebarSearch.State())
	}
}

func TestAccountTab_StaleBodyLoadedDropped(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	openUID := tab.viewer.CurrentUID()
	// Deliver a body for a UID we never opened — must be ignored.
	tab, _ = tab.updateTab(bodyLoadedMsg{uid: mail.UID("nonsense"), blocks: nil})
	if tab.viewer.phase == viewerReady {
		t.Errorf("viewer for UID %s readied on stale bodyLoaded", openUID)
	}
}

func TestAccountTab_QClosesViewer(t *testing.T) {
	tab := newLoadedTab(t, 120, 30)
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyEnter})
	if !tab.viewer.IsOpen() {
		t.Fatal("viewer should be open")
	}
	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if tab.viewer.IsOpen() {
		t.Error("q must close viewer")
	}
	if cmd == nil {
		t.Error("close must emit ViewerClosedMsg cmd")
	}
}

// pagingFakeBackend is a test-only mail.Backend that serves a
// configurable number of messages for pagination tests. Only the
// methods exercised by the load flow are implemented.
type pagingFakeBackend struct {
	msgs []mail.MessageInfo
}

func newPagingFakeBackend(count int) *pagingFakeBackend {
	msgs := make([]mail.MessageInfo, count)
	for i := range msgs {
		uid := mail.UID(fmt.Sprintf("%d", i+1))
		msgs[i] = mail.MessageInfo{UID: uid, Subject: "msg", From: "a@b", ThreadID: uid}
	}
	return &pagingFakeBackend{msgs: msgs}
}

func (b *pagingFakeBackend) AccountName() string              { return "test" }
func (b *pagingFakeBackend) Connect(_ context.Context) error  { return nil }
func (b *pagingFakeBackend) Disconnect() error                { return nil }
func (b *pagingFakeBackend) ListFolders() ([]mail.Folder, error) {
	return []mail.Folder{{Name: "Inbox", Role: "inbox"}}, nil
}
func (b *pagingFakeBackend) OpenFolder(_ string) error { return nil }
func (b *pagingFakeBackend) QueryFolder(_ string, offset, limit int) ([]mail.UID, int, error) {
	total := len(b.msgs)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	uids := make([]mail.UID, end-offset)
	for i, m := range b.msgs[offset:end] {
		uids[i] = m.UID
	}
	return uids, total, nil
}
func (b *pagingFakeBackend) FetchHeaders(uids []mail.UID) ([]mail.MessageInfo, error) {
	set := make(map[mail.UID]mail.MessageInfo, len(b.msgs))
	for _, m := range b.msgs {
		set[m.UID] = m
	}
	result := make([]mail.MessageInfo, 0, len(uids))
	for _, uid := range uids {
		if m, ok := set[uid]; ok {
			result = append(result, m)
		}
	}
	return result, nil
}
func (b *pagingFakeBackend) FetchBody(_ mail.UID) (io.Reader, error)         { return nil, nil }
func (b *pagingFakeBackend) Search(_ mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }
func (b *pagingFakeBackend) Move(_ []mail.UID, _ string) error                { return nil }
func (b *pagingFakeBackend) Copy(_ []mail.UID, _ string) error                { return nil }
func (b *pagingFakeBackend) Delete(_ []mail.UID) error                        { return nil }
func (b *pagingFakeBackend) Flag(_ []mail.UID, _ mail.Flag, _ bool) error     { return nil }
func (b *pagingFakeBackend) MarkRead(_ []mail.UID) error                      { return nil }
func (b *pagingFakeBackend) MarkAnswered(_ []mail.UID) error                  { return nil }
func (b *pagingFakeBackend) Send(_ string, _ []string, _ io.Reader) error     { return nil }
func (b *pagingFakeBackend) Updates() <-chan mail.Update                       { return nil }

func TestAccountTab_PaginationInitialLoad(t *testing.T) {
	// 600 messages — first window fetches 500.
	backend := newPagingFakeBackend(600)
	styles := NewStyles(theme.Nord)
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Simulate: folders loaded → selectionChangedCmds → openFolderCmd chain.
	folders, _ := backend.ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	drain(t, &tab, cmd)

	page := tab.pageFor("Inbox")
	if page.loaded != 500 {
		t.Errorf("after initial load: page.loaded = %d, want 500", page.loaded)
	}
	if page.total != 600 {
		t.Errorf("after initial load: page.total = %d, want 600", page.total)
	}
	if tab.msglist.Count() != 500 {
		t.Errorf("msglist.Count() = %d, want 500", tab.msglist.Count())
	}
}

func TestAccountTab_MaybeLoadMore_NearBottom(t *testing.T) {
	// 600 messages; after initial load of 500, cursor near bottom should trigger load-more.
	backend := newPagingFakeBackend(600)
	styles := NewStyles(theme.Nord)
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})

	folders, _ := backend.ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	drain(t, &tab, cmd)

	// Move cursor to bottom to trigger load-more.
	tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if cmd == nil {
		t.Fatal("G near bottom with more pages available must emit a load-more Cmd")
	}
	page := tab.pageFor("Inbox")
	if !page.loadMoreInFlight {
		t.Error("loadMoreInFlight must be true after maybeLoadMore triggers")
	}
}

func TestAccountTab_MaybeLoadMore_InFlightNoDuplicate(t *testing.T) {
	backend := newPagingFakeBackend(600)
	styles := NewStyles(theme.Nord)
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})

	folders, _ := backend.ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	drain(t, &tab, cmd)

	// Move to bottom — sets loadMoreInFlight.
	tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	// A second navigation while in-flight must NOT re-dispatch.
	tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Errorf("second near-bottom nav while in-flight must not emit a Cmd, got %T", cmd)
	}
}

func TestAccountTab_MaybeLoadMore_LoadedEqualsTotal(t *testing.T) {
	// 14 messages (the mock count) — loaded == total from the start.
	backend := mail.NewMockBackend()
	styles := NewStyles(theme.Nord)
	tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})

	folders, _ := backend.ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	drain(t, &tab, cmd)

	tab, cmd = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if cmd != nil {
		t.Errorf("at bottom with loaded == total, must not emit a Cmd; got %T", cmd)
	}
}

// TestAccountTab_LoadingSpinner verifies the folder-open loading state.
func TestAccountTab_LoadingSpinner(t *testing.T) {
	t.Run("loading is true after selectionChangedCmds, before headersApplied", func(t *testing.T) {
		styles := NewStyles(theme.Nord)
		backend := mail.NewMockBackend()
		tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		folders, _ := backend.ListFolders()
		// foldersLoadedMsg calls selectionChangedCmds which sets loading=true.
		tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})
		if !tab.loading {
			t.Error("loading should be true after foldersLoadedMsg triggers selectionChangedCmds")
		}
	})

	t.Run("loading is false after headersAppliedMsg", func(t *testing.T) {
		styles := NewStyles(theme.Nord)
		backend := mail.NewMockBackend()
		tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		folders, _ := backend.ListFolders()
		tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})
		// Deliver headersAppliedMsg — loading clears.
		tab, _ = tab.updateTab(headersAppliedMsg{name: "Inbox", msgs: []mail.MessageInfo{
			{UID: "1", Subject: "hello", From: "a", ThreadID: "1"},
		}})
		if tab.loading {
			t.Error("loading should be false after headersAppliedMsg")
		}
	})

	t.Run("view contains Loading placeholder while loading and msglist empty", func(t *testing.T) {
		styles := NewStyles(theme.Nord)
		backend := mail.NewMockBackend()
		tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		folders, _ := backend.ListFolders()
		// After foldersLoadedMsg, loading=true and msglist is still empty.
		tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})
		view := stripANSI(tab.View())
		if !strings.Contains(view, "Loading") {
			t.Error("view should contain Loading placeholder while loading and msglist is empty")
		}
	})

	t.Run("view does not contain Loading after headers arrive", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		view := stripANSI(tab.View())
		if strings.Contains(view, "Loading messages") {
			t.Error("view should not contain Loading placeholder after headers have been applied")
		}
	})
}

func TestAccountTab_WindowCounter(t *testing.T) {
	t.Run("returns empty when no page loaded", func(t *testing.T) {
		styles := NewStyles(theme.Nord)
		backend := mail.NewMockBackend()
		tab := NewAccountTab(styles, theme.Nord, backend, config.DefaultUIConfig())
		got := tab.WindowCounter()
		if got != "" {
			t.Errorf("WindowCounter() = %q, want empty", got)
		}
	})

	t.Run("returns empty when loaded equals total", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		// Overwrite the page to a loaded==total state.
		name := tab.currentFolderName()
		tab.pages[name] = &folderPage{loaded: 100, total: 100}
		got := tab.WindowCounter()
		if got != "" {
			t.Errorf("WindowCounter() = %q, want empty when loaded==total", got)
		}
	})

	t.Run("returns empty when total is zero", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		name := tab.currentFolderName()
		tab.pages[name] = &folderPage{loaded: 0, total: 0}
		got := tab.WindowCounter()
		if got != "" {
			t.Errorf("WindowCounter() = %q, want empty when total==0", got)
		}
	})

	t.Run("returns counter string on partial load", func(t *testing.T) {
		tab := newLoadedTab(t, 120, 30)
		name := tab.currentFolderName()
		tab.pages[name] = &folderPage{loaded: 500, total: 2347}
		got := tab.WindowCounter()
		if got != "500/2347" {
			t.Errorf("WindowCounter() = %q, want 500/2347", got)
		}
	})
}
