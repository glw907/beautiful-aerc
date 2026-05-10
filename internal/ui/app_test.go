package ui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/sidebar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func init() {
	// Fixture expectations were written with uicore.FancyIcons (SPUA-A glyphs).
	// spuaCellWidth must be 2 so ansix.Width measures them correctly.
	ansix.SetSPUACellWidth(2)
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
	return newLoadedAppWithIcons(t, width, height, uicore.FancyIcons)
}

// newLoadedAppWithIcons is like newLoadedApp but accepts an explicit uicore.IconSet.
func newLoadedAppWithIcons(t *testing.T, width, height int, icons uicore.IconSet) App {
	t.Helper()
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), icons, nil, nil)
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
// Cmds that do not return within cmdTimeout are skipped. This handles
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
		return // blocking Cmd (e.g. pump), skip it
	}
	feed(t, app, msg)
}

// feed routes one message into App.Update, recursing into BatchMsg
// children so nested batches (e.g. App.Init wrapping AccountTab.Init)
// expand fully.
func feed(t *testing.T, app *App, msg tea.Msg) {
	t.Helper()
	if msg == nil {
		return
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
				continue
			}
			feed(t, app, inner)
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
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'c'})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
	})

	t.Run("window size stored", func(t *testing.T) {
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if app.width != 120 || app.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", app.width, app.height)
		}
	})

	t.Run("view has top line with ╮", func(t *testing.T) {
		app := newLoadedApp(t, 80, 24)
		view := app.View().Content
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
		view := app.View().Content
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
		view := app.View().Content
		plain := stripANSI(view)
		if strings.Contains(plain, "╭") {
			t.Error("should not contain ╭ (tab bar removed)")
		}
	})

	t.Run("content height (chrome subtracted)", func(t *testing.T) {
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		app.width = 80
		app.height = 24
		if app.contentHeight() != 21 {
			t.Errorf("contentHeight = %d, want 21", app.contentHeight())
		}
	})

	t.Run("sidebar composite", func(t *testing.T) {
		app := newLoadedApp(t, 80, 20)
		view := app.View().Content
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
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		if app.footer.context != AccountContext {
			t.Errorf("footer context = %d, want AccountContext", app.footer.context)
		}
	})

	t.Run("status bar tracks nav", func(t *testing.T) {
		app := newLoadedApp(t, 80, 20)
		// Navigate to Spam (index 4: Inbox->Drafts->Sent->Archive->Spam).
		// Each J dispatches a load. Drain the chain.
		for range 4 {
			var cmd tea.Cmd
			app, cmd = app.Update(tea.KeyPressMsg{Code: 'J', Text: "J"})
			drainApp(t, &app, cmd)
		}
		view := app.View().Content
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
		app, _ = app.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
		app, cmd := app.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
		if cmd != nil {
			drainApp(t, &app, cmd)
		}
		app, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if app.acct.SidebarColumnValue().SidebarSearch().State() != sidebar.SearchActive {
			t.Fatalf("setup: state = %v, want SearchActive", app.acct.SidebarColumnValue().SidebarSearch().State())
		}

		_, cmd = app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
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
	// Open the viewer by pressing Enter. deriveChromeFromAcct reads the
	// new viewer state after delegation and updates footer + statusBar.
	app, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("viewerOpen should be true after Enter")
	}
	if app.footer.context != ViewerContext {
		t.Errorf("after Enter, footer context = %v, want ViewerContext", app.footer.context)
	}
	if app.statusBar.mode != StatusViewer {
		t.Errorf("statusBar mode = %v, want StatusViewer", app.statusBar.mode)
	}
}

func TestApp_ViewerClosedRestoresFooterContext(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	// Open via Enter, then close via q.
	app, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("setup: viewerOpen should be true after Enter")
	}
	app, _ = app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if app.footer.context != AccountContext {
		t.Errorf("footer context = %v, want AccountContext", app.footer.context)
	}
	if app.statusBar.mode != StatusAccount {
		t.Errorf("statusBar mode = %v, want StatusAccount", app.statusBar.mode)
	}
}

func TestApp_ViewerScrollUpdatesStatusBar(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	// Open the viewer and give it a body long enough to scroll.
	app, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("viewerOpen should be true after Enter")
	}
	// Inject a long body so G (go-to-bottom) produces a non-zero scroll pct.
	long := strings.Repeat("alpha bravo charlie delta epsilon ", 50)
	app.acct = app.acct.WithViewer(app.acct.Viewer().SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: long}}},
	}, content.Unsubscribe{}, nil))
	// Press G. deriveChromeFromAcct reads ViewerScrollPct after delegation.
	app, _ = app.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if app.statusBar.scrollPct != 100 {
		t.Errorf("statusBar scrollPct = %d, want 100 after G", app.statusBar.scrollPct)
	}
	view := stripANSI(app.statusBar.View(120, 30))
	if !strings.Contains(view, "100%") {
		t.Errorf("status bar view missing 100%% in viewer mode: %q", view)
	}
}

func TestApp_HelpOpenAndCloseWithQuestionMark(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	if app.helpOpen {
		t.Fatal("setup: helpOpen should be false initially")
	}

	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.helpOpen {
		t.Fatal("after ?: helpOpen should be true")
	}

	view := stripANSI(app.View().Content)
	if !strings.Contains(view, "Message List") {
		t.Errorf("popover view missing 'Message List' title:\n%s", view)
	}

	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if app.helpOpen {
		t.Error("after second ?: helpOpen should be false")
	}
}

func TestApp_HelpDismissedByEsc(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}
	app, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if app.helpOpen {
		t.Error("Esc should close help")
	}
}

func TestApp_HelpStealsKeys(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	startMsgSelected := app.acct.MsgList().Selected()
	startFolderSelected := app.acct.SidebarColumnValue().Sidebar().Selected()

	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	// Send a battery of keys that would normally do something.
	stealKeys := []rune{'j', 'J', 'd', 'r', '/'}
	for _, k := range stealKeys {
		app, _ = app.Update(tea.KeyPressMsg{Code: k, Text: string(k)})
	}

	if app.acct.MsgList().Selected() != startMsgSelected {
		t.Errorf("msglist selection moved while help open: %d → %d",
			startMsgSelected, app.acct.MsgList().Selected())
	}
	if app.acct.SidebarColumnValue().Sidebar().Selected() != startFolderSelected {
		t.Errorf("sidebar selection moved while help open: %d → %d",
			startFolderSelected, app.acct.SidebarColumnValue().Sidebar().Selected())
	}
	if app.acct.SidebarColumnValue().SidebarSearch().State() != sidebar.SearchIdle {
		t.Errorf("search state changed while help open: got %v",
			app.acct.SidebarColumnValue().SidebarSearch().State())
	}
	if !app.helpOpen {
		t.Error("help closed unexpectedly during key barrage")
	}
}

func TestApp_HelpQuitSwallowed(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	_, cmd := app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
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

	// Open help in account context. Title is "Message List".
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := stripANSI(app.View().Content)
	if !strings.Contains(view, "Message List") {
		t.Errorf("account-context help should title 'Message List':\n%s", view)
	}
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"}) // close

	// Open the viewer via Enter. deriveChromeFromAcct sets viewerOpen.
	var cmd tea.Cmd
	app, cmd = app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("setup: viewer did not open after Enter")
	}

	// Open help. The title should now be "Message Viewer".
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view = stripANSI(app.View().Content)
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

	view := stripANSI(app.View().Content)
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
	view := stripANSI(app.View().Content)
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
	// appear in the output. The overlay composites over, not replaces, the
	// frame. The banner does not steal keys. That's enforced by Update, not
	// View.
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(ErrorMsg{Op: "fetch body", Err: errors.New("EOF")})
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	view := stripANSI(app.View().Content)
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
//  2. The raw (ANSI-preserved) output contains ESC[2m or ESC[2;X somewhere,
//     confirming the background frame was passed through DimANSI.
func TestApp_HelpOverlayDimsBg(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.helpOpen {
		t.Fatal("setup: ? did not open help")
	}

	raw := app.View().Content
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
	without := strings.Count(app.View().Content, "\n")

	app, _ = app.Update(ErrorMsg{Op: "x", Err: errors.New("y")})
	with := strings.Count(app.View().Content, "\n")

	if with != without {
		t.Errorf("total view height changed: without=%d, with=%d", without, with)
	}
}

func TestApp_InitialConnStateIsOffline(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
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
			app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
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
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
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
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
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
	// ansix.Width (not lipgloss.Width) so each row is padded correctly
	// before the border is appended.
	//
	// Parameterized across three icon/width modes to lock in the invariant
	// that ansix.Width(row) == terminal width regardless of ansix.SetSPUACellWidth.
	for _, mode := range []struct {
		name    string
		width   int
		iconSet uicore.IconSet
	}{
		{"simple_w1", 1, uicore.SimpleIcons},
		{"fancy_w1", 1, uicore.FancyIcons},
		{"fancy_w2", 2, uicore.FancyIcons},
	} {
		t.Run(mode.name, func(t *testing.T) {
			ansix.SetSPUACellWidth(mode.width)
			defer ansix.SetSPUACellWidth(2) // restore the package init() default

			borderRune := '│'
			for _, w := range []int{80, 100, 120, 160} {
				app := newLoadedAppWithIcons(t, w, 30, mode.iconSet)
				view := app.View().Content
				lines := strings.Split(view, "\n")
				for lineIdx, line := range lines {
					if line == "" {
						continue
					}
					plain := stripANSI(line)
					if !strings.ContainsRune(plain, borderRune) {
						continue
					}
					dw := ansix.Width(line)
					if dw != w {
						t.Errorf("mode=%s w=%d line %d: ansix.Width=%d, want %d: %q",
							mode.name, w, lineIdx, dw, w, plain)
					}
				}
			}
		})
	}
}

func TestApp_ToastLifecycle(t *testing.T) {
	frozen := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	newApp := func() App {
		backend := mail.NewMockBackend()
		app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
		app.now = func() time.Time { return frozen }
		return app
	}

	t.Run("account.TriageStartedMsg sets toast and returns Tick Cmd", func(t *testing.T) {
		app := newApp()
		app, cmd := app.Update(account.TriageStartedMsg{
			Op:      "delete",
			N:       1,
			UIDs:    []mail.UID{"1"},
			Inverse: func() tea.Msg { return nil },
		})
		if app.toast.IsZero() {
			t.Fatal("toast should be set after account.TriageStartedMsg")
		}
		if app.toast.op != "delete" || app.toast.n != 1 {
			t.Errorf("toast = %+v, want op=delete n=1", app.toast)
		}
		want := frozen.Add(time.Duration(app.undoSeconds) * time.Second)
		if !app.toast.deadline.Equal(want) {
			t.Errorf("deadline = %v, want %v", app.toast.deadline, want)
		}
		if cmd == nil {
			t.Error("expected Tick Cmd, got nil")
		}
	})

	t.Run("toastExpireMsg with matching deadline clears toast", func(t *testing.T) {
		app := newApp()
		app, _ = app.Update(account.TriageStartedMsg{Op: "delete", N: 1})
		dl := app.toast.deadline
		app, _ = app.Update(toastExpireMsg{deadline: dl})
		if !app.toast.IsZero() {
			t.Error("toast should clear on matching expire")
		}
	})

	t.Run("toastExpireMsg with stale deadline ignored", func(t *testing.T) {
		app := newApp()
		app, _ = app.Update(account.TriageStartedMsg{Op: "delete", N: 1})
		stale := frozen.Add(-time.Hour)
		app, _ = app.Update(toastExpireMsg{deadline: stale})
		if app.toast.IsZero() {
			t.Error("toast should NOT clear on stale expire")
		}
	})

	t.Run("undoRequestedMsg returns inverse, clears toast", func(t *testing.T) {
		app := newApp()
		var ranInverse bool
		app, _ = app.Update(account.TriageStartedMsg{
			Op:      "delete",
			N:       1,
			Inverse: func() tea.Msg { ranInverse = true; return nil },
		})
		_, cmd := app.Update(undoRequestedMsg{})
		if cmd == nil {
			t.Fatal("expected inverse Cmd from undoRequestedMsg, got nil")
		}
		_ = cmd()
		if !ranInverse {
			t.Error("inverse Cmd did not fire")
		}
	})

	t.Run("undoRequestedMsg with no toast is no-op", func(t *testing.T) {
		app := newApp()
		_, cmd := app.Update(undoRequestedMsg{})
		if cmd != nil {
			t.Error("expected nil Cmd when no toast active")
		}
	})

	t.Run("ErrorMsg with active toast clears toast", func(t *testing.T) {
		app := newApp()
		app, _ = app.Update(account.TriageStartedMsg{
			Op: "delete",
			N:  1,
		})
		app, _ = app.Update(ErrorMsg{Op: "delete", Err: errors.New("boom")})
		if !app.toast.IsZero() {
			t.Error("toast should clear on ErrorMsg")
		}
		if app.lastErr.Err == nil {
			t.Error("lastErr should be set after ErrorMsg")
		}
	})

	t.Run("ErrorMsg with no toast just sets lastErr", func(t *testing.T) {
		app := newApp()
		app, _ = app.Update(ErrorMsg{Op: "x", Err: errors.New("y")})
		if app.lastErr.Err == nil {
			t.Error("lastErr should be set")
		}
	})
}

func TestApp_BannerToastPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		setupErr   bool
		setTost    bool
		wantSubs   []string
		bannedSubs []string
		empty      bool
	}{
		{name: "neither", empty: true},
		{name: "toast only", setTost: true, wantSubs: []string{"u undo"}, bannedSubs: []string{"⚠"}},
		{name: "error only", setupErr: true, wantSubs: []string{"⚠"}, bannedSubs: []string{"u undo"}},
		{name: "both → error wins", setupErr: true, setTost: true, wantSubs: []string{"⚠"}, bannedSubs: []string{"u undo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newLoadedApp(t, 100, 30)
			if tc.setTost {
				app, _ = app.Update(account.TriageStartedMsg{Op: "delete", N: 1})
			}
			if tc.setupErr {
				app, _ = app.Update(ErrorMsg{Op: "x", Err: errors.New("y")})
			}
			row := app.chromeBannerRow(100)
			if tc.empty {
				if row != "" {
					t.Errorf("want empty row, got %q", stripANSI(row))
				}
				return
			}
			plain := stripANSI(row)
			for _, s := range tc.wantSubs {
				if !strings.Contains(plain, s) {
					t.Errorf("row %q missing %q", plain, s)
				}
			}
			for _, s := range tc.bannedSubs {
				if strings.Contains(plain, s) {
					t.Errorf("row %q should NOT contain %q", plain, s)
				}
			}
		})
	}
}

func TestAppLinkPickerRoundTrip(t *testing.T) {
	captured := ""
	app := newLoadedApp(t, 120, 30).WithOpener(func(url string) error {
		captured = url
		return nil
	})

	// Open viewer on a message with one link.
	app, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)
	if !app.viewerOpen {
		t.Fatal("expected viewer open after Enter")
	}

	// Inject a body with one harvested link directly into the viewer
	// so the harvest path is exercised and v.links populates.
	app.acct = app.acct.WithViewer(app.acct.Viewer().SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{
			content.Link{Text: "click", URL: "https://example.com"},
		}},
	}, content.Unsubscribe{}, nil))

	// Tab → viewer sets pendingLinkPicker → AccountTab relays → App opens picker via deriveChromeFromAcct.
	app, cmd = app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	drainApp(t, &app, cmd)
	if !app.IsLinkPickerOpen() {
		t.Fatal("expected picker open after Tab")
	}

	// Enter → picker emits LaunchURLMsg + LinkPickerClosedMsg → App
	// fires launchURLCmd (which calls openURL hook) and closes picker.
	app, cmd = app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainApp(t, &app, cmd)

	if captured != "https://example.com" {
		t.Fatalf("expected openURL called with https://example.com, got %q", captured)
	}
	if app.IsLinkPickerOpen() {
		t.Fatal("expected picker closed after Enter launch")
	}
}

func TestApp_OpenMovePickerOpensOverlay(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	folders := []mail.FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: mail.GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal},
	}
	app, _ = app.Update(movepicker.OpenMsg{UIDs: []mail.UID{"1"}, Src: "INBOX", Folders: folders})
	if !app.movePicker.IsOpen() {
		t.Error("movePicker should be open after movepicker.OpenMsg")
	}
}

func TestApp_MovePickerClosedFlipsState(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	folders := []mail.FolderEntry{{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal}}
	app, _ = app.Update(movepicker.OpenMsg{UIDs: []mail.UID{"1"}, Src: "INBOX", Folders: folders})
	app, _ = app.Update(movepicker.ClosedMsg{})
	if app.movePicker.IsOpen() {
		t.Error("movePicker should be closed after movepicker.ClosedMsg")
	}
}

func TestApp_FolderJumpInertWhilePickerOpen(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	folders := []mail.FolderEntry{{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal}}
	app, _ = app.Update(movepicker.OpenMsg{UIDs: []mail.UID{"1"}, Src: "INBOX", Folders: folders})
	beforeFolder := app.acct.CurrentFolderName()
	app, _ = app.Update(tea.KeyPressMsg{Code: 'I', Text: "I"})
	if got := app.acct.CurrentFolderName(); got != beforeFolder {
		t.Errorf("folder changed to %q while picker open; want %q (key should be swallowed)", got, beforeFolder)
	}
}

func TestApp_MovePickerPickedRoutesToAccountTab(t *testing.T) {
	app := newLoadedApp(t, 120, 30)
	// Use action targets from the loaded message list so dispatchMoveFromPicker
	// doesn't short-circuit on empty UIDs.
	uids := app.acct.MsgList().ActionTargets()
	src := app.acct.CurrentFolderName()
	folders := []mail.FolderEntry{{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal}}
	app, _ = app.Update(movepicker.OpenMsg{UIDs: uids, Src: src, Folders: folders})
	_, cmd := app.Update(movepicker.PickedMsg{UIDs: uids, Src: src, Dest: "Archive"})
	if cmd == nil {
		t.Fatal("Picked produced nil cmd; expected account.TriageStartedMsg + forward")
	}
	msgs := drainBatch(cmd)
	var sawStart bool
	for _, m := range msgs {
		if ts, ok := m.(account.TriageStartedMsg); ok {
			sawStart = true
			if ts.Dest != "Archive" {
				t.Errorf("account.TriageStartedMsg.dest = %q, want %q", ts.Dest, "Archive")
			}
		}
	}
	if !sawStart {
		t.Errorf("no account.TriageStartedMsg in %v", msgs)
	}
}

func TestApp_FolderChangeCommitsToast(t *testing.T) {
	app := newLoadedApp(t, 100, 30)
	app, _ = app.Update(account.TriageStartedMsg{Op: "delete", N: 1})
	if app.toast.IsZero() {
		t.Fatal("setup: account.TriageStartedMsg should have set the toast")
	}

	// Simulate folder load completion. This is what selectionChangedCmds
	// resolves to when J/K or a folder-jump key fires.
	// Simulate a folder change: the msglist is empty (selectionChangedCmds
	// just reset it) and a account.FolderLoadedMsg arrives. That combination is
	// the App's signal to clear the toast.
	ml := app.acct.MsgList()
	ml.SetMessages(nil)
	app.acct = app.acct.WithMsgList(ml)
	app, _ = app.Update(account.FolderLoadedMsg{Name: "Drafts", Msgs: nil, Total: 0})
	if !app.toast.IsZero() {
		t.Errorf("toast should clear on folder change, got %+v", app.toast)
	}
}

func TestApp_OpensConfirmModalOnEmptyMsg(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	if app.IsConfirmOpen() {
		t.Fatal("confirm should be closed at start")
	}
	app, _ = app.Update(account.OpenConfirmEmptyMsg{Folder: "Trash", Total: 247, Source: "Trash"})
	if !app.IsConfirmOpen() {
		t.Fatal("confirm should be open after account.OpenConfirmEmptyMsg")
	}
	plain := stripANSI(app.View().Content)
	if !strings.Contains(plain, "Empty Trash") {
		t.Errorf("View should contain 'Empty Trash'; got %q", plain)
	}
	if !strings.Contains(plain, "247") {
		t.Errorf("View should contain '247'; got %q", plain)
	}
}

func TestApp_ConfirmYesEmitsConfirmedMsgAndCloses(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(account.OpenConfirmEmptyMsg{Folder: "Trash", Total: 5, Source: "Trash"})
	if !app.IsConfirmOpen() {
		t.Fatal("setup: confirm should be open")
	}

	app, cmd := app.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	yesMsgs := drainBatch(cmd)

	// Confirm.Update emits ConfirmModalYesMsg + ConfirmModalClosedMsg.
	// Feed both back through App.Update. The Yes handler then emits
	// account.EmptyFolderConfirmedMsg.
	found := false
	for _, m := range yesMsgs {
		var c tea.Cmd
		app, c = app.Update(m)
		for _, follow := range drainBatch(c) {
			if _, ok := follow.(account.EmptyFolderConfirmedMsg); ok {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected account.EmptyFolderConfirmedMsg after draining Yes batch; got %v", yesMsgs)
	}
}

func TestApp_ConfirmEscClosesWithoutEmit(t *testing.T) {
	app := newLoadedApp(t, 80, 24)
	app, _ = app.Update(account.OpenConfirmEmptyMsg{Folder: "Trash", Total: 5, Source: "Trash"})
	if !app.IsConfirmOpen() {
		t.Fatal("setup: confirm should be open")
	}

	app2, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msgs := drainBatch(cmd)

	// Must emit ConfirmModalClosedMsg.
	sawClosed := false
	for _, m := range msgs {
		if _, ok := m.(ConfirmModalClosedMsg); ok {
			sawClosed = true
		}
		if _, ok := m.(account.EmptyFolderConfirmedMsg); ok {
			t.Error("must NOT emit account.EmptyFolderConfirmedMsg on Esc")
		}
	}
	if !sawClosed {
		t.Errorf("expected ConfirmModalClosedMsg; got %v", msgs)
	}

	// Drain the ConfirmModalClosedMsg so the modal actually closes.
	for _, m := range msgs {
		app2, _ = app2.Update(m)
	}
	if app2.IsConfirmOpen() {
		t.Error("confirm should be closed after draining ConfirmModalClosedMsg")
	}
}

func TestApp_OfflineBannerWithQueuedOps(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.lastOutboxDepth = cache.OutboxDepth{Pending: 2}

	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnOffline,
	}})
	if app.lastErr.Op != "connection" {
		t.Errorf("expected offline banner, got Op=%q", app.lastErr.Op)
	}
	if !app.offlineHinted {
		t.Errorf("offlineHinted not set")
	}
}

func TestApp_OfflineNoBannerWhenOutboxEmpty(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)

	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnOffline,
	}})
	if app.lastErr.Op == "connection" {
		t.Errorf("offline banner emitted on empty outbox")
	}
	if app.offlineHinted {
		t.Errorf("offlineHinted set on empty outbox")
	}
}

func TestApp_ConnectedClearsOfflineHinted(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.offlineHinted = true

	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnConnected,
	}})
	if app.offlineHinted {
		t.Errorf("offlineHinted not cleared on Connected")
	}
}

func TestApp_QOpensOutboxOverlay(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, cmd := app.Update(tea.KeyPressMsg{Code: 'Q', Text: "Q"})
	if !app.outboxOpen {
		t.Errorf("Q did not set outboxOpen")
	}
	if cmd == nil {
		t.Errorf("Q did not issue a load Cmd")
	}
}

func TestApp_BangOpensConflictOverlay(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, cmd := app.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if !app.conflictOpen {
		t.Errorf("! did not set conflictOpen")
	}
	if cmd == nil {
		t.Errorf("! did not issue a load Cmd")
	}
}

func TestApp_QToBangTransition(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, _ = app.Update(tea.KeyPressMsg{Code: 'Q', Text: "Q"})
	if !app.outboxOpen {
		t.Fatalf("setup: Q did not open outbox")
	}
	app, cmd := app.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if app.outboxOpen {
		t.Errorf("outbox stayed open after !")
	}
	if cmd == nil {
		t.Fatal("! produced no Cmd")
	}
	msg := cmd()
	if _, ok := msg.(OpenConflictsFromOutboxMsg); !ok {
		t.Fatalf("expected OpenConflictsFromOutboxMsg, got %T", msg)
	}
	app, _ = app.Update(msg)
	if !app.conflictOpen {
		t.Errorf("OpenConflictsFromOutboxMsg did not open conflict overlay")
	}
}

func TestApp_OutboxDepthMsgUpdatesStatusBar(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app, _ = app.Update(outboxDepthMsg{depth: cache.OutboxDepth{Pending: 3, Conflict: 1}})
	if app.lastOutboxDepth.Pending != 3 || app.lastOutboxDepth.Conflict != 1 {
		t.Errorf("lastOutboxDepth not stored: %+v", app.lastOutboxDepth)
	}
	out := stripANSI(app.statusBar.View(120, 30))
	if !strings.Contains(out, "⚠4") {
		t.Errorf("status bar missing ⚠4: %q", out)
	}
}

func TestApp_ConflictResolvedRefreshes(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	app.conflictOpen = true
	_, cmd := app.Update(conflictResolvedMsg{opID: 42})
	if cmd == nil {
		t.Fatal("conflictResolvedMsg produced no Cmd")
	}
}

func TestApp_OfflineBannerLatchedOncePerOfflineEpisode(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.lastOutboxDepth = cache.OutboxDepth{Pending: 1}

	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnOffline,
	}})
	// User dismisses by writing a different lastErr.
	app.lastErr = ErrorMsg{}
	// Second offline transition without a Connected in between: should not re-emit.
	app, _ = app.Update(backendUpdateMsg{update: mail.Update{
		Type:      mail.UpdateConnState,
		ConnState: mail.ConnOffline,
	}})
	if app.lastErr.Op == "connection" {
		t.Errorf("offline banner re-emitted without intervening Connected")
	}
}

func TestApp_OpenAttachPickerMsg_OpensOverlay(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app2, _ := app.Update(reader.OpenAttachPickerMsg{
		UID:   "u1",
		Items: []mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 1}},
	})
	if !app2.attachPicker.IsOpen() {
		t.Error("OpenAttachPickerMsg did not open the picker")
	}
}

func TestApp_SaveAttachmentMsg_DispatchesCmd(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	_, cmd := app.Update(reader.SaveAttachmentMsg{
		UID: "u1",
		Att: mail.Attachment{PartID: "2", Filename: "x.pdf", Size: 1},
	})
	if cmd == nil {
		t.Error("SaveAttachmentMsg should dispatch a Cmd")
	}
}

func newTestApp(t *testing.T) App {
	t.Helper()
	backend := mail.NewMockBackend()
	return NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
}

// newJMAPTestApp is like newTestApp but with IsJMAP() = true.
func newJMAPTestApp(t *testing.T) (App, *mail.MockBackend) {
	t.Helper()
	backend := mail.NewMockBackend()
	backend.SetJMAP(true)
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	return app, backend
}

// blockingBackend is a minimal mail.Backend stub. Methods are no-ops
// returning zero values. FetchBody blocks on release.
type blockingBackend struct {
	release chan struct{}
}

func (b *blockingBackend) AccountName() string                 { return "test" }
func (b *blockingBackend) AccountEmail() string                { return "test@example.com" }
func (b *blockingBackend) Connect(_ context.Context) error     { return nil }
func (b *blockingBackend) Disconnect() error                   { return nil }
func (b *blockingBackend) ListFolders() ([]mail.Folder, error) { return nil, nil }
func (b *blockingBackend) OpenFolder(_ string) error           { return nil }
func (b *blockingBackend) QueryFolder(_ string, _, _ int) ([]mail.UID, int, error) {
	return nil, 0, nil
}
func (b *blockingBackend) FetchHeaders(_ []mail.UID) ([]mail.MessageInfo, error) { return nil, nil }
func (b *blockingBackend) FetchBody(_ mail.UID) ([]byte, error) {
	<-b.release
	return []byte("body"), nil
}
func (b *blockingBackend) Attachments(_ mail.UID) ([]mail.Attachment, error) { return nil, nil }
func (b *blockingBackend) FetchAttachment(_ mail.UID, _ string) ([]byte, error) {
	return nil, nil
}
func (b *blockingBackend) Move(_ []mail.UID, _ string) error            { return nil }
func (b *blockingBackend) Destroy(_ []mail.UID) error                   { return nil }
func (b *blockingBackend) Flag(_ []mail.UID, _ mail.Flag, _ bool) error { return nil }
func (b *blockingBackend) Send(_ mail.Envelope, _ []byte) error         { return nil }
func (b *blockingBackend) Append(_ string, _ []byte, _ mail.Flag) error { return nil }
func (b *blockingBackend) PushDraft(_ string, _ []byte, _ mail.UID) (mail.UID, error) {
	return "", mail.ErrUnsupported
}
func (b *blockingBackend) IsJMAP() bool { return false }
func (b *blockingBackend) Updates() <-chan mail.Update {
	ch := make(chan mail.Update)
	return ch
}

// noSentBackend is a mail.Backend stub whose folder list has no Sent
// role or Sent-named folder. Used to verify the missing-Sent path.
type noSentBackend struct {
	blockingBackend
}

func (b *noSentBackend) ListFolders() ([]mail.Folder, error) {
	return []mail.Folder{
		{Name: "Inbox", Exists: 1, Unseen: 0, Role: "inbox"},
	}, nil
}

func newTestAppWithoutSentFolder(t *testing.T) App {
	t.Helper()
	backend := &noSentBackend{blockingBackend: blockingBackend{release: make(chan struct{})}}
	acct := newTestCache(t, backend)
	return NewApp(theme.Nord, acct, config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
}

func TestApp_ComposeSend_QueuesOutboundAndClosesCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open")
	}
	app.compose.SetTo("alice@example.com")
	app.compose.SetSubject("hi")
	app.compose.SetBody("hello")

	d, err := app.compose.Draft()
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}

	out, cmd := app.Update(uicompose.SendMsg{Draft: d})
	app = out
	if app.compose != nil {
		t.Fatalf("compose should be false after uicompose.SendMsg")
	}
	if cmd == nil {
		t.Fatalf("uicompose.SendMsg should return a Cmd that runs QueueOutbound")
	}
	drainApp(t, &app, cmd)
	depth, err := app.acct.Cache().OutboxDepth(context.Background())
	if err != nil {
		t.Fatalf("OutboxDepth: %v", err)
	}
	if depth.Pending+depth.Conflict == 0 {
		t.Fatalf("outbox should have at least one queued op, got %+v", depth)
	}
}

func TestApp_ComposeSend_NoSentFolder_SurfacesError(t *testing.T) {
	app := newTestAppWithoutSentFolder(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open")
	}
	app.compose.SetTo("alice@example.com")
	app.compose.SetSubject("hi")
	app.compose.SetBody("hello")

	d, _ := app.compose.Draft()
	out, cmd := app.Update(uicompose.SendMsg{Draft: d})
	app = out
	if app.compose == nil {
		t.Fatalf("missing Sent folder should keep compose open")
	}
	if cmd != nil {
		_ = cmd()
	}
	if app.compose.Err() == "" {
		t.Fatalf("missing Sent folder should surface inline err")
	}
}

func TestApp_ComposeSentMsg_SetsToast(t *testing.T) {
	frozen := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	app := newTestApp(t)
	app.now = func() time.Time { return frozen }

	// Zero ScheduledFor means no hold window; drainer is immediately eligible.
	// No toast should appear.
	out, _ := app.Update(uicompose.SentMsg{})
	app = out
	if !app.toast.IsZero() {
		t.Fatalf("SentMsg with zero ScheduledFor should not arm a toast; got op=%q", app.toast.op)
	}

	// Non-zero ScheduledFor arms the send-undo banner.
	scheduled := frozen.Add(10 * time.Second)
	out, cmd := app.Update(uicompose.SentMsg{
		OpIDs:        []int64{1},
		ScheduledFor: scheduled,
	})
	app = out
	if app.toast.op != opSendUndo {
		t.Fatalf("toast.op = %q, want %q", app.toast.op, opSendUndo)
	}
	if !app.toast.deadline.Equal(scheduled) {
		t.Errorf("toast.deadline = %v, want %v", app.toast.deadline, scheduled)
	}
	if cmd == nil {
		t.Error("SentMsg with future ScheduledFor should return Cmd for tick/expiry")
	}
}

func TestApp_C_OpensCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("c should open compose")
	}
	if app.compose == nil {
		t.Fatal("c should construct compose.Model")
	}
}

func TestApp_View_RendersComposeWhenOpen(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	v := stripANSI(app.View().Content)
	if !strings.Contains(v, "From:") || !strings.Contains(v, "Subject:") {
		t.Fatalf("View should include compose headers when compose:\n%s", v)
	}
}

func TestApp_ComposeSendMsg_ClosesCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be true")
	}
	app, _ = app.Update(uicompose.SendMsg{})
	if app.compose != nil {
		t.Error("uicompose.SendMsg should clear compose")
	}
	if app.compose != nil {
		t.Error("uicompose.SendMsg should nil compose")
	}
}

func TestApp_ComposeCancelMsg_ClosesCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be true")
	}
	app, cmd := app.Update(uicompose.CancelMsg{})
	if app.compose != nil {
		t.Error("uicompose.CancelMsg should clear compose")
	}
	if app.compose != nil {
		t.Error("uicompose.CancelMsg should nil compose")
	}
	if cmd != nil {
		t.Error("stub should return nil Cmd")
	}
}

func TestApp_ComposeKeyStolenWhenOpen(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be true")
	}
	// While compose is open, keys route to compose.Model. j should not move
	// the message list cursor.
	beforeSelected := app.acct.MsgList().Selected()
	app, _ = app.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if app.acct.MsgList().Selected() != beforeSelected {
		t.Error("j should be consumed by compose.Model when compose is open, not msglist")
	}
}

func TestApp_WindowSizeSetsComposeSize(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should not be nil")
	}
	app, _ = app.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	if app.compose == nil {
		t.Fatal("compose should survive WindowSizeMsg")
	}
	// After resize compose should render non-empty content.
	if app.compose.View() == "" {
		t.Error("compose.View() is empty after resize, want non-empty")
	}
}

func TestApp_R_OpensReplySeededCompose(t *testing.T) {
	app := newLoadedApp(t, 120, 40)
	_, ok := app.selectedMessage()
	if !ok {
		t.Fatal("setup: no selected message in loaded app")
	}

	out, cmd := app.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	app = out
	if cmd == nil {
		t.Fatal("r should return a Cmd to fetch parent body and seed compose")
	}
	// Execute the seed Cmd and feed the result back.
	msg := cmd()
	app, _ = app.Update(msg)

	if app.compose == nil {
		t.Fatal("r should open compose after seeding")
	}
	if app.compose == nil {
		t.Fatal("compose should be non-nil after seeding")
	}
	if !strings.HasPrefix(app.compose.SubjectValue(), "Re:") {
		t.Fatalf("subject should be Re:-prefixed, got %q", app.compose.SubjectValue())
	}
}

func TestApp_CtrlC_DirtyOpensConfirm(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open after c")
	}
	app.compose.SetBody("dirty content")

	out, _ := app.Update(uicompose.CancelMsg{Dirty: true})
	app = out
	if !app.confirm.IsOpen() {
		t.Fatal("dirty cancel should open ConfirmModal")
	}
	if app.compose == nil {
		t.Fatal("compose should remain open while confirming")
	}
	if !app.pendingComposeSave {
		t.Fatal("pendingComposeSave should be set")
	}
}

func TestApp_CtrlC_EmptyClosesImmediately(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open after c")
	}

	out, _ := app.Update(uicompose.CancelMsg{Dirty: false})
	app = out
	if app.compose != nil {
		t.Fatal("empty cancel should close immediately")
	}
	if app.compose != nil {
		t.Fatal("compose should be nil after empty cancel")
	}
}

func TestApp_ConfirmModalYes_DiscardsCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open after c")
	}
	app.compose.SetBody("dirty content")

	app, _ = app.Update(uicompose.CancelMsg{Dirty: true})
	if !app.confirm.IsOpen() {
		t.Fatal("setup: confirm should be open after dirty cancel")
	}

	app, _ = app.Update(ConfirmModalYesMsg{})
	if app.compose != nil {
		t.Fatal("Yes on discard should close compose")
	}
	if app.compose != nil {
		t.Fatal("compose should be nil after Yes discard")
	}
	if app.pendingComposeSave {
		t.Fatal("pendingComposeSave should be cleared after Yes")
	}
}

// Pressing n on the Save? modal discards the draft and closes compose.
func TestApp_ConfirmModalNo_DiscardsCompose(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open after c")
	}
	app.compose.SetBody("dirty content")

	app, _ = app.Update(uicompose.CancelMsg{Dirty: true})
	if !app.confirm.IsOpen() {
		t.Fatal("setup: confirm should be open")
	}

	app, _ = app.Update(ConfirmModalNoMsg{})
	if app.compose != nil {
		t.Fatal("compose should be nil after No (discard) on Save? modal")
	}
	if app.pendingComposeSave {
		t.Fatal("pendingComposeSave should be cleared after No")
	}
}

// TestApp_FreshComposeAllocatesDraftID verifies that pressing c on a JMAP
// account opens compose with a non-empty draftID and wires the cache store.
func TestApp_FreshComposeAllocatesDraftID(t *testing.T) {
	app, _ := newJMAPTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("compose should be open after c")
	}
	if app.compose.DraftID() == "" {
		t.Error("compose DraftID should be non-empty")
	}
}

// TestApp_DraftsEnterLooksUpLocalRow verifies that pressing Enter in the
// Drafts folder on a JMAP account emits openDraftFromServerUIDCmd which
// resolves to an openDraftMsg carrying the seeded DraftRow.
func TestApp_DraftsEnterLooksUpLocalRow(t *testing.T) {
	app, _ := newJMAPTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drainApp(t, &app, app.Init())

	ctx := context.Background()
	acct := app.acct.Cache()

	// Seed a local draft tied to server UID "1" (first UID MockBackend returns).
	d := compose.Draft{Subject: "draft subject", Body: "draft body"}
	payload, err := compose.EncodeDraft(d)
	if err != nil {
		t.Fatalf("EncodeDraft: %v", err)
	}
	const draftID = "d-test-known"
	if err := acct.CreateDraft(ctx, draftID, payload); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := acct.MarkDraftPushed(ctx, draftID, "1", "Drafts"); err != nil {
		t.Fatalf("MarkDraftPushed: %v", err)
	}

	// Navigate to Drafts via D key, then inject a synthetic FolderLoadedMsg
	// so the message list is populated without relying on async cache sync.
	app, jumpCmd := app.Update(tea.KeyPressMsg{Code: 'D', Text: "D"})
	drainApp(t, &app, jumpCmd)

	if app.acct.CurrentFolderName() != "Drafts" {
		t.Fatalf("current folder = %q, want Drafts", app.acct.CurrentFolderName())
	}

	// Populate the message list with a single message whose UID matches the
	// seeded server UID "1", bypassing the async cache sync.
	draftMsg := mail.MessageInfo{UID: "1", ThreadID: "1", Subject: "draft subject"}
	app, _ = app.Update(account.FolderLoadedMsg{Name: "Drafts", Msgs: []mail.MessageInfo{draftMsg}, Total: 1})

	msg, ok := app.acct.SelectedMessage()
	if !ok {
		t.Fatal("no message selected after FolderLoadedMsg injection")
	}
	if msg.UID != "1" {
		t.Fatalf("selected UID = %q, want 1", msg.UID)
	}

	// Press Enter: should return openDraftFromServerUIDCmd.
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in Drafts should return a cmd")
	}
	result := cmd()
	odm, ok := result.(openDraftMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want openDraftMsg", result)
	}
	if odm.row.DraftID != draftID {
		t.Errorf("DraftID = %q, want %q", odm.row.DraftID, draftID)
	}
}

// TestApp_EscOnDirtyJMAPComposeOpensSaveDraftModal verifies that on a JMAP
// account, a CancelMsg with Dirty=true opens the Save? ConfirmModal and sets
// pendingComposeSave.
func TestApp_EscOnDirtyJMAPComposeOpensSaveDraftModal(t *testing.T) {
	app, _ := newJMAPTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app, _ = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if app.compose == nil {
		t.Fatal("setup: compose should be open after c")
	}
	app.compose.SetBody("draft content")

	app, _ = app.Update(uicompose.CancelMsg{Dirty: true})
	if !app.confirm.IsOpen() {
		t.Fatal("dirty cancel on JMAP should open ConfirmModal")
	}
	if !app.pendingComposeSave {
		t.Fatal("pendingComposeSave should be set")
	}
	if app.compose == nil {
		t.Fatal("compose should remain open while confirm is pending")
	}
}

func TestApp_OpenContactDeleteConfirmMsg_SetsState(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	app, _ = app.Update(contacts.OpenContactDeleteConfirmMsg{
		UID:         "uid-123",
		DisplayName: "Alice Example",
	})

	if app.pendingContactDelete != "uid-123" {
		t.Errorf("pendingContactDelete = %q, want uid-123", app.pendingContactDelete)
	}
	if !app.confirm.IsOpen() {
		t.Fatal("confirm modal should be open after OpenContactDeleteConfirmMsg")
	}
}

func TestApp_ConfirmYes_ContactDelete_ClearsState(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	app, _ = app.Update(contacts.OpenContactDeleteConfirmMsg{
		UID:         "uid-456",
		DisplayName: "Bob Example",
	})
	if app.pendingContactDelete != "uid-456" {
		t.Fatal("setup: pendingContactDelete not set")
	}

	app, cmd := app.Update(ConfirmModalYesMsg{})

	if app.pendingContactDelete != "" {
		t.Error("pendingContactDelete should be cleared after Yes")
	}
	if app.form != nil {
		t.Error("form should be nil after Yes on delete confirm")
	}
	if cmd == nil {
		t.Error("Yes on delete confirm should emit a Cmd")
	}
}

func TestAppArmsSendUndoOnSentMsg(t *testing.T) {
	frozen := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.now = func() time.Time { return frozen }
	app.undoSendWindow = 10 * time.Second

	scheduled := frozen.Add(10 * time.Second)
	app, _ = app.Update(uicompose.SentMsg{
		OpIDs:        []int64{42},
		ScheduledFor: scheduled,
	})

	if app.toast.op != opSendUndo {
		t.Errorf("op = %q, want %q", app.toast.op, opSendUndo)
	}
	if got := app.toast.sendOpIDs; len(got) != 1 || got[0] != 42 {
		t.Errorf("sendOpIDs = %v, want [42]", got)
	}
	if !app.toast.deadline.Equal(scheduled) {
		t.Errorf("deadline = %v, want %v", app.toast.deadline, scheduled)
	}
}

func TestAppDoesNotArmWhenWindowZero(t *testing.T) {
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.undoSendWindow = 0

	app, _ = app.Update(uicompose.SentMsg{
		OpIDs:        []int64{42},
		ScheduledFor: time.Time{},
	})

	if !app.toast.IsZero() {
		t.Errorf("pending should remain zero with window=0; got %+v", app.toast)
	}
}

func TestAppUndoSendCancelsAndRestores(t *testing.T) {
	frozen := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	backend := mail.NewMockBackend()
	app := NewApp(theme.Nord, newTestCache(t, backend), config.DefaultUIConfig(), uicore.FancyIcons, nil, nil)
	app.now = func() time.Time { return frozen }
	app.undoSendWindow = 10 * time.Second

	d := compose.Draft{Subject: "test"}
	app, _ = app.Update(uicompose.SentMsg{
		OpIDs:        []int64{42},
		ScheduledFor: frozen.Add(10 * time.Second),
		Draft:        d,
	})
	if app.toast.op != opSendUndo {
		t.Fatalf("setup: op = %q, want send-undo", app.toast.op)
	}

	app, cmd := app.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd == nil {
		t.Fatal("expected a Cmd from u")
	}
	if !app.toast.IsZero() {
		t.Errorf("pending not cleared: %+v", app.toast)
	}
}
