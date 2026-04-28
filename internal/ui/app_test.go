// SPDX-License-Identifier: MIT

package ui

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func init() {
	// Fixture expectations were written with FancyIcons (SPUA-A glyphs).
	// spuaCellWidth must be 2 so displayCells measures them correctly.
	SetSPUACellWidth(2)
}

// stripANSI removes ANSI escape sequences to get plain text for positional checks.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// newLoadedApp constructs an App and drives its initial Cmd chain so
// the folder list and first folder's messages are populated.
func newLoadedApp(t *testing.T, width, height int) App {
	t.Helper()
	return newLoadedAppWithIcons(t, width, height, FancyIcons)
}

// newLoadedAppWithIcons is like newLoadedApp but accepts an explicit IconSet.
func newLoadedAppWithIcons(t *testing.T, width, height int, icons IconSet) App {
	t.Helper()
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), icons)
	app, _ = app.Update(tea.WindowSizeMsg{Width: width, Height: height})
	drainApp(t, &app, app.Init())
	return app
}

// cmdTimeout is the maximum time drainApp waits for a single Cmd to
// return. Blocking Cmds (e.g. pumpUpdatesCmd waiting on a channel)
// are skipped when the timeout expires.
const cmdTimeout = 50 * time.Millisecond

// execCmd runs cmd() in a goroutine and returns the result via a
// channel. Callers apply cmdTimeout to skip indefinitely-blocking Cmds.
func execCmd(cmd tea.Cmd) <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	return ch
}

// drainApp walks a Cmd (including one layer of batching) and feeds
// every resulting non-batch message back into the app's Update.
// Cmds that do not return within cmdTimeout are skipped — this handles
// the pumpUpdatesCmd which blocks indefinitely on a channel.
func drainApp(t *testing.T, app *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	var msg tea.Msg
	select {
	case msg = <-execCmd(cmd):
	case <-time.After(cmdTimeout):
		return // blocking Cmd (e.g. pump) — skip it
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			var inner tea.Msg
			select {
			case inner = <-execCmd(sub):
			case <-time.After(cmdTimeout):
				continue // blocking sub-Cmd — skip it
			}
			newApp, next := app.Update(inner)
			*app = newApp
			if next != nil {
				drainApp(t, app, next)
			}
		}
		return
	}
	newApp, next := app.Update(msg)
	*app = newApp
	if next != nil {
		drainApp(t, app, next)
	}
}

func TestApp(t *testing.T) {
	backend := mail.NewMockBackend()

	t.Run("quit on q", func(t *testing.T) {
		app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
	})

	t.Run("window size stored", func(t *testing.T) {
		app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if app.width != 120 || app.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", app.width, app.height)
		}
	})

	t.Run("view has top line with ╮", func(t *testing.T) {
		app := newLoadedApp(t, 80, 24)
		view := app.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")
		if len(lines) < 1 {
			t.Fatal("no lines rendered")
		}
		trimmed := strings.TrimRight(lines[0], " ")
		if !strings.HasSuffix(trimmed, "╮") {
			t.Errorf("first line should end with ╮: %q", trimmed)
		}
	})

	t.Run("view has status bar with ╯", func(t *testing.T) {
		app := newLoadedApp(t, 80, 24)
		view := app.View()
		plain := stripANSI(view)
		found := false
		for _, line := range strings.Split(plain, "\n") {
			if strings.HasSuffix(strings.TrimRight(line, " "), "╯") {
				found = true
				break
			}
		}
		if !found {
			t.Error("no line ends with ╯ (status bar missing)")
		}
	})

	t.Run("no tab bar", func(t *testing.T) {
		app := newLoadedApp(t, 80, 24)
		view := app.View()
		plain := stripANSI(view)
		if strings.Contains(plain, "╭") {
			t.Error("should not contain ╭ (tab bar removed)")
		}
	})

	t.Run("content height is height minus 3 chrome rows", func(t *testing.T) {
		app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
		app.width = 80
		app.height = 24
		if app.contentHeight() != 21 {
			t.Errorf("contentHeight = %d, want 21", app.contentHeight())
		}
	})

	t.Run("sidebar renders in composite layout", func(t *testing.T) {
		app := newLoadedApp(t, 80, 20)
		view := app.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")

		for _, name := range []string{"Inbox", "Drafts", "Sent", "Archive", "Spam", "Trash"} {
			found := false
			for _, line := range lines {
				runes := []rune(line)
				if len(runes) >= 30 {
					sidebarPart := string(runes[:30])
					if strings.Contains(sidebarPart, name) {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("folder %q not found in sidebar region", name)
			}
		}
	})

	t.Run("footer starts in account context", func(t *testing.T) {
		app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
		if app.footer.context != AccountContext {
			t.Errorf("footer context = %d, want AccountContext", app.footer.context)
		}
	})

	t.Run("status bar updates on sidebar navigation", func(t *testing.T) {
		app := newLoadedApp(t, 80, 20)
		// Navigate to Spam (index 4: Inbox->Drafts->Sent->Archive->Spam).
		// Each J dispatches a FolderChangedMsg + load — drain the chain.
		for range 4 {
			var cmd tea.Cmd
			app, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
			drainApp(t, &app, cmd)
		}
		view := app.View()
		plain := stripANSI(view)
		// Spam has 12 unseen
		if !strings.Contains(plain, "12 unread") {
			t.Error("status bar should show 12 unread after navigating to Spam")
		}
	})
}

func TestAppQuitStolenDuringSearch(t *testing.T) {
	t.Run("q during Active clears search, does not quit", func(t *testing.T) {
		app := newLoadedApp(t, 80, 30)

		// Activate search and type a character, commit, then press q.
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		app, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		if cmd != nil {
			drainApp(t, &app, cmd)
		}
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if app.acct.sidebarSearch.State() != SearchActive {
			t.Fatalf("setup: state = %v, want SearchActive", app.acct.sidebarSearch.State())
		}

		_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd != nil {
			msg := cmd()
			if _, isQuit := msg.(tea.QuitMsg); isQuit {
				t.Error("q during Active returned tea.Quit; should have cleared search")
			}
		}
	})
}

func TestApp_ViewerOpenedSwitchesFooterContext(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	if app.footer.context != AccountContext {
		t.Fatalf("initial footer context = %v, want AccountContext", app.footer.context)
	}
	app, _ = app.Update(ViewerOpenedMsg{})
	if app.footer.context != ViewerContext {
		t.Errorf("after ViewerOpenedMsg, footer context = %v, want ViewerContext", app.footer.context)
	}
	if app.statusBar.mode != StatusViewer {
		t.Errorf("statusBar mode = %v, want StatusViewer", app.statusBar.mode)
	}
}

func TestApp_ViewerClosedRestoresFooterContext(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	app, _ = app.Update(ViewerOpenedMsg{})
	app, _ = app.Update(ViewerClosedMsg{})
	if app.footer.context != AccountContext {
		t.Errorf("footer context = %v, want AccountContext", app.footer.context)
	}
	if app.statusBar.mode != StatusAccount {
		t.Errorf("statusBar mode = %v, want StatusAccount", app.statusBar.mode)
	}
}

func TestApp_ViewerScrollUpdatesStatusBar(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	app, _ = app.Update(ViewerOpenedMsg{})
	app, _ = app.Update(ViewerScrollMsg{Pct: 47})
	if app.statusBar.scrollPct != 47 {
		t.Errorf("statusBar scrollPct = %d, want 47", app.statusBar.scrollPct)
	}
	view := stripANSI(app.statusBar.View(120, 30))
	if !strings.Contains(view, "47%") {
		t.Errorf("status bar view missing 47%% in viewer mode: %q", view)
	}
}

func TestApp_HelpOpenAndCloseWithQuestionMark(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	if app.helpOpen {
		t.Fatal("setup: helpOpen should be false initially")
	}

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("after ?: helpOpen should be true")
	}

	view := stripANSI(app.View())
	if !strings.Contains(view, "Message List") {
		t.Errorf("popover view missing 'Message List' title:\n%s", view)
	}

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if app.helpOpen {
		t.Error("after second ?: helpOpen should be false")
	}
}

func TestApp_HelpDismissedByEsc(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.helpOpen {
		t.Error("Esc should close help")
	}
}

func TestApp_HelpStealsKeys(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	startMsgSelected := app.acct.msglist.Selected()
	startFolderSelected := app.acct.sidebar.Selected()

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	// Send a battery of keys that would normally do something.
	stealKeys := []rune{'j', 'J', 'd', 'r', '/'}
	for _, k := range stealKeys {
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
	}

	if app.acct.msglist.Selected() != startMsgSelected {
		t.Errorf("msglist selection moved while help open: %d → %d",
			startMsgSelected, app.acct.msglist.Selected())
	}
	if app.acct.sidebar.Selected() != startFolderSelected {
		t.Errorf("sidebar selection moved while help open: %d → %d",
			startFolderSelected, app.acct.sidebar.Selected())
	}
	if app.acct.sidebarSearch.State() != SearchIdle {
		t.Errorf("search state changed while help open: got %v",
			app.acct.sidebarSearch.State())
	}
	if !app.helpOpen {
		t.Error("help closed unexpectedly during key barrage")
	}
}

func TestApp_HelpQuitSwallowed(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Error("q during help returned tea.Quit; should be swallowed")
		}
	}
	if !app.helpOpen {
		t.Error("q during help closed the popover; should be swallowed")
	}
}

func TestApp_HelpContextSwitchesWithViewer(t *testing.T) {
	app := newLoadedApp(t, 120, 30)

	// Open help in account context — title is "Message List".
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view := stripANSI(app.View())
	if !strings.Contains(view, "Message List") {
		t.Errorf("account-context help should title 'Message List':\n%s", view)
	}
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}) // close

	// Open the viewer.
	app, _ = app.Update(ViewerOpenedMsg{})
	if !app.viewerOpen {
		t.Fatal("setup: viewer did not open")
	}

	// Open help — now the title should be "Message Viewer".
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view = stripANSI(app.View())
	if !strings.Contains(view, "Message Viewer") {
		t.Errorf("viewer-context help should title 'Message Viewer':\n%s", view)
	}
}

func TestApp_CapturesErrorMsg(t *testing.T) {
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(ErrorMsg{Op: "mark read", Err: errors.New("timeout")})

	if app.lastErr.Err == nil {
		t.Fatal("App.lastErr.Err is nil after ErrorMsg")
	}
	if app.lastErr.Op != "mark read" {
		t.Errorf("Op: got %q, want %q", app.lastErr.Op, "mark read")
	}
}

func TestApp_BannerRendersAboveStatus(t *testing.T) {
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(ErrorMsg{Op: "fetch body", Err: errors.New("EOF")})

	view := stripANSI(app.View())
	if !strings.Contains(view, "⚠") {
		t.Error("View missing warning glyph")
	}
	if !strings.Contains(view, "fetch body") {
		t.Error("View missing op")
	}
}

func TestApp_BannerLastWriteWins(t *testing.T) {
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(ErrorMsg{Op: "first", Err: errors.New("a")})
	app, _ = app.Update(ErrorMsg{Op: "second", Err: errors.New("b")})

	if app.lastErr.Op != "second" {
		t.Errorf("Op: got %q, want %q (last-write-wins)", app.lastErr.Op, "second")
	}
	view := stripANSI(app.View())
	if strings.Contains(view, "first") {
		t.Errorf("View still contains the first error after replacement: %q", view)
	}
	if !strings.Contains(view, "second") {
		t.Errorf("View missing the second (current) error: %q", view)
	}
}

func TestApp_PopoverOverlaysErrorBanner(t *testing.T) {
	// With the dimmed-overlay design the error banner is part of the dimmed
	// background that is visible behind the popover. The popover must also
	// appear in the output — the overlay composites over, not replaces, the
	// frame. The banner does not steal keys; that's enforced by Update, not
	// View.
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(ErrorMsg{Op: "fetch body", Err: errors.New("EOF")})
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	view := stripANSI(app.View())
	// Popover must be present.
	if !strings.Contains(view, "Message List") {
		t.Errorf("popover missing from view with error banner open: %q", view)
	}
	// The banner text is in the dimmed background (overlay design).
	if !strings.Contains(view, "fetch body") {
		t.Errorf("banner background missing from view: %q", view)
	}
}

// TestApp_HelpOverlayDimsBg is the F3b.3 composite test. It verifies:
//  1. Both the popover box content and the underlying account-view content
//     appear in the ANSI-stripped output (overlay, not replacement).
//  2. The raw (ANSI-preserved) output contains ESC[2m or ESC[2; somewhere,
//     confirming the background frame was passed through DimANSI.
func TestApp_HelpOverlayDimsBg(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	raw := app.View()
	plain := stripANSI(raw)

	// Popover box must appear.
	if !strings.Contains(plain, "Navigate") {
		t.Errorf("popover content 'Navigate' missing from overlay view:\n%s", plain)
	}

	// Underlying account-view content must appear (folder name in sidebar).
	if !strings.Contains(plain, "Inbox") {
		t.Errorf("background content 'Inbox' missing from overlay view:\n%s", plain)
	}

	// The raw output must carry dim ANSI codes injected by DimANSI.
	hasDim := strings.Contains(raw, "\x1b[2m") || strings.Contains(raw, "\x1b[2;")
	if !hasDim {
		t.Error("raw view missing ESC[2m or ESC[2; dim marker — background was not dimmed")
	}
}

func TestApp_BannerShrinksContentByOneRow(t *testing.T) {
	app := newLoadedApp(t, 100, 30)
	without := strings.Count(app.View(), "\n")

	app, _ = app.Update(ErrorMsg{Op: "x", Err: errors.New("y")})
	with := strings.Count(app.View(), "\n")

	if with != without {
		t.Errorf("total view height changed: without=%d, with=%d", without, with)
	}
}

func TestApp_InitialConnStateIsOffline(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
	if got := app.statusBar.ConnectionState(); got != Offline {
		t.Errorf("initial connState = %v, want Offline", got)
	}
}

func TestApp_BackendUpdateConnState(t *testing.T) {
	cases := []struct {
		name      string
		connState mail.ConnState
		want      ConnectionState
	}{
		{"connected", mail.ConnConnected, Connected},
		{"reconnecting", mail.ConnReconnecting, Reconnecting},
		{"offline", mail.ConnOffline, Offline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := mail.NewMockBackend()
			app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
			msg := backendUpdateMsg{update: mail.Update{
				Type:      mail.UpdateConnState,
				ConnState: tc.connState,
			}}
			app, _ = app.Update(msg)
			if got := app.statusBar.ConnectionState(); got != tc.want {
				t.Errorf("connState = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApp_BackendUpdateClosedChannelGoesOffline(t *testing.T) {
	// Simulate the closed-channel case: pumpUpdatesCmd delivers a
	// backendUpdateMsg with ConnState=ConnOffline.
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
	// Force to Connected first so we can confirm the transition.
	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnConnected,
	}})
	if got := app.statusBar.ConnectionState(); got != Connected {
		t.Fatalf("setup: connState = %v, want Connected", got)
	}
	// Now deliver the closed-channel sentinel.
	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnOffline,
	}})
	if got := app.statusBar.ConnectionState(); got != Offline {
		t.Errorf("after closed-channel sentinel: connState = %v, want Offline", got)
	}
}

func TestApp_BackendUpdateReArmspump(t *testing.T) {
	// Verify that handling a backendUpdateMsg returns a non-nil Cmd
	// (the re-armed pump). We can't execute it without blocking, but
	// we confirm the Cmd is present.
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, backend, config.DefaultUIConfig(), FancyIcons)
	msg := backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnConnected,
	}}
	_, cmd := app.Update(msg)
	if cmd == nil {
		t.Error("backendUpdateMsg handler returned nil Cmd; pump would die")
	}
}

func TestApp_RightBorderAlignment(t *testing.T) {
	// The right border │ must land at the same terminal column for every
	// content row, regardless of whether the row contains SPUA-A Nerd Font
	// glyphs (sidebar folder icons, message flag icons). App.View uses
	// displayCells (not lipgloss.Width) so each row is padded correctly
	// before the border is appended.
	//
	// Parameterized across three icon/width modes to lock in the invariant
	// that displayCells(row) == terminal width regardless of SetSPUACellWidth.
	for _, mode := range []struct {
		name    string
		width   int
		iconSet IconSet
	}{
		{"simple_w1", 1, SimpleIcons},
		{"fancy_w1", 1, FancyIcons},
		{"fancy_w2", 2, FancyIcons},
	} {
		t.Run(mode.name, func(t *testing.T) {
			SetSPUACellWidth(mode.width)
			defer SetSPUACellWidth(2) // restore the package init() default

			borderRune := '│'
			for _, w := range []int{80, 100, 120, 160} {
				app := newLoadedAppWithIcons(t, w, 30, mode.iconSet)
				view := app.View()
				lines := strings.Split(view, "\n")
				for lineIdx, line := range lines {
					if line == "" {
						continue
					}
					plain := stripANSI(line)
					if !strings.ContainsRune(plain, borderRune) {
						continue
					}
					dw := displayCells(line)
					if dw != w {
						t.Errorf("mode=%s w=%d line %d: displayCells=%d, want %d: %q",
							mode.name, w, lineIdx, dw, w, plain)
					}
				}
			}
		})
	}
}

func TestAppLinkPickerRoundTrip(t *testing.T) {
	captured := ""
	prev := openURL
	openURL = func(url string) error { captured = url; return nil }
	defer func() { openURL = prev }()

	app := newLoadedApp(t, 120, 30)

	// Open viewer on a message with one link.
	app, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("expected viewer open after Enter")
	}

	// Inject a body with one harvested link directly into the viewer
	// so the harvest path is exercised and v.links populates.
	app.acct.viewer = app.acct.viewer.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{
			content.Link{Text: "click", URL: "https://example.com"},
		}},
	})

	// Tab → viewer emits LinkPickerOpenMsg → App opens picker.
	app, cmd = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	drainApp(t, &app, cmd)
	if !app.IsLinkPickerOpen() {
		t.Fatal("expected picker open after Tab")
	}

	// Enter → picker emits LaunchURLMsg + LinkPickerClosedMsg → App
	// fires launchURLCmd (which calls openURL hook) and closes picker.
	app, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drainApp(t, &app, cmd)

	if captured != "https://example.com" {
		t.Fatalf("expected openURL called with https://example.com, got %q", captured)
	}
	if app.IsLinkPickerOpen() {
		t.Fatal("expected picker closed after Enter launch")
	}
}
