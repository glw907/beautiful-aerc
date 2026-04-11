# Poplar Chrome Shell Implementation Plan (Pass 2.5b-1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the persistent chrome shell frame — tab bar, status bar, command footer, focus cycling, theme bridge — that every future poplar screen renders inside.

**Architecture:** Root bubbletea program with Elm architecture. Root model (`App`) owns tabs, styles, and status bar. `AccountTab` owns a two-panel placeholder with focus cycling. All colors come from `*theme.CompiledTheme` via a `Styles` bridge struct. Mock backend provides hardcoded folder/message data for the status bar.

**Tech Stack:** Go 1.26, bubbletea, bubbles (help, key), lipgloss, existing `internal/theme` and `internal/mail` packages.

**Mandatory conventions:** Read and follow `~/.claude/docs/go-conventions.md` and `~/.claude/docs/elm-conventions.md` before writing any code. Use `superpowers:go` skill for all Go development tasks.

**Design spec:** `docs/superpowers/specs/2026-04-10-poplar-chrome-shell-design.md`

---

## File Structure

```
cmd/poplar/root.go             — rewrite: tea.NewProgram startup (replaces diagnostic stub)
internal/ui/styles.go          — Styles struct + NewStyles(*theme.CompiledTheme)
internal/ui/keys.go            — key bindings (bubbles/key KeyMap types)
internal/ui/tab.go             — Tab interface definition
internal/ui/tab_bar.go         — renderTabBar() function
internal/ui/status_bar.go      — StatusBar model
internal/ui/footer.go          — footer rendering with bubbles/help
internal/ui/account_tab.go     — AccountTab model with focus cycling
internal/ui/app.go             — root App model
internal/mail/mock.go          — mock Backend implementation
```

Each file has one responsibility. Test files live alongside source.

---

### Task 1: Add bubbletea and bubbles dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add bubbletea dependency**

```bash
cd /home/glw907/Projects/beautiful-aerc && go get github.com/charmbracelet/bubbletea@latest
```

- [ ] **Step 2: Add bubbles dependency**

```bash
cd /home/glw907/Projects/beautiful-aerc && go get github.com/charmbracelet/bubbles@latest
```

- [ ] **Step 3: Tidy modules**

```bash
cd /home/glw907/Projects/beautiful-aerc && go mod tidy
```

- [ ] **Step 4: Verify build**

Run: `cd /home/glw907/Projects/beautiful-aerc && make build`
Expected: All four binaries build without errors.

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add go.mod go.sum
git commit -m "Add bubbletea and bubbles dependencies for poplar TUI

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Theme-to-lipgloss bridge (`internal/ui/styles.go`)

The `Styles` struct translates `*theme.CompiledTheme` palette colors into composed lipgloss styles for every UI element in the chrome shell. Created once at startup, passed read-only to all child components.

**Files:**
- Create: `internal/ui/styles.go`
- Create: `internal/ui/styles_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/styles_test.go`:

```go
package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestNewStyles(t *testing.T) {
	s := NewStyles(theme.Nord)

	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"TabActiveBorder", s.TabActiveBorder},
		{"TabActiveText", s.TabActiveText},
		{"TabInactiveText", s.TabInactiveText},
		{"TabConnectLine", s.TabConnectLine},
		{"FrameBorder", s.FrameBorder},
		{"PanelDivider", s.PanelDivider},
		{"StatusBar", s.StatusBar},
		{"StatusConnected", s.StatusConnected},
		{"StatusReconnect", s.StatusReconnect},
		{"StatusOffline", s.StatusOffline},
		{"FooterKey", s.FooterKey},
		{"FooterHint", s.FooterHint},
		{"Selection", s.Selection},
		{"Dim", s.Dim},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify each style renders without panic and produces non-empty output.
			out := tt.style.Render("test")
			if out == "" {
				t.Errorf("style %s rendered empty string", tt.name)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestNewStyles -v`
Expected: FAIL — package `internal/ui` does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/styles.go`:

```go
// Package ui implements poplar's bubbletea terminal UI.
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// Styles holds composed lipgloss styles derived from a CompiledTheme.
// Created once at startup and passed read-only to all child components.
type Styles struct {
	// Tab bar
	TabActiveBorder lipgloss.Style
	TabActiveText   lipgloss.Style
	TabInactiveText lipgloss.Style
	TabConnectLine  lipgloss.Style

	// Content frame
	FrameBorder  lipgloss.Style
	PanelDivider lipgloss.Style

	// Status bar
	StatusBar       lipgloss.Style
	StatusConnected lipgloss.Style
	StatusReconnect lipgloss.Style
	StatusOffline   lipgloss.Style

	// Footer
	FooterKey  lipgloss.Style
	FooterHint lipgloss.Style

	// Selection (used by focus cycling)
	Selection lipgloss.Style

	// Placeholder text
	Dim lipgloss.Style
}

// NewStyles creates a Styles from a CompiledTheme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		TabActiveBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		TabActiveText: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).
			Background(t.BgBase),
		TabInactiveText: lipgloss.NewStyle().
			Foreground(t.FgDim),
		TabConnectLine: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		FrameBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		PanelDivider: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.FgBright).
			Background(t.BgBorder),
		StatusConnected: lipgloss.NewStyle().
			Foreground(t.ColorSuccess),
		StatusReconnect: lipgloss.NewStyle().
			Foreground(t.ColorWarning),
		StatusOffline: lipgloss.NewStyle().
			Foreground(t.FgDim),

		FooterKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		FooterHint: lipgloss.NewStyle().
			Foreground(t.FgDim),

		Selection: lipgloss.NewStyle().
			Background(t.BgSelection),

		Dim: lipgloss.NewStyle().
			Foreground(t.FgDim),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestNewStyles -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/styles.go internal/ui/styles_test.go
git commit -m "Add theme-to-lipgloss bridge (Styles struct)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Key bindings (`internal/ui/keys.go`)

Defines key bindings using `bubbles/key`. Three KeyMap types: global keys (handled by root App), message list context, and sidebar context.

**Files:**
- Create: `internal/ui/keys.go`

- [ ] **Step 1: Write the implementation**

Create `internal/ui/keys.go`:

```go
package ui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys are handled by the root App model.
type GlobalKeys struct {
	Tab1  key.Binding
	Tab2  key.Binding
	Tab3  key.Binding
	Tab4  key.Binding
	Tab5  key.Binding
	Tab6  key.Binding
	Tab7  key.Binding
	Tab8  key.Binding
	Tab9  key.Binding
	Help  key.Binding
	Cmd   key.Binding
	Quit  key.Binding
}

// NewGlobalKeys returns the default global key bindings.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Tab1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "tab 1")),
		Tab2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "tab 2")),
		Tab3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "tab 3")),
		Tab4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "tab 4")),
		Tab5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "tab 5")),
		Tab6: key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "tab 6")),
		Tab7: key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "tab 7")),
		Tab8: key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "tab 8")),
		Tab9: key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "tab 9")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Cmd:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// MsgListKeys are shown in the footer when the message list is focused.
type MsgListKeys struct {
	Delete  key.Binding
	Archive key.Binding
	Star    key.Binding
	Reply   key.Binding
	ReplyAll key.Binding
	Forward key.Binding
	Compose key.Binding
	Search  key.Binding
	Help    key.Binding
	Cmd     key.Binding
}

// NewMsgListKeys returns the default message list key bindings.
func NewMsgListKeys() MsgListKeys {
	return MsgListKeys{
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "del")),
		Archive:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
		Star:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
		Reply:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
		ReplyAll: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "all")),
		Forward:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fwd")),
		Compose:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Cmd:      key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
	}
}

// ShortHelp implements help.KeyMap for the message list context.
func (k MsgListKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Delete, k.Archive, k.Star, k.Reply, k.ReplyAll,
		k.Forward, k.Compose, k.Search, k.Help, k.Cmd,
	}
}

// FullHelp implements help.KeyMap for the message list context.
func (k MsgListKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// SidebarKeys are shown in the footer when the sidebar is focused.
type SidebarKeys struct {
	Open    key.Binding
	Compose key.Binding
	Cmd     key.Binding
}

// NewSidebarKeys returns the default sidebar key bindings.
func NewSidebarKeys() SidebarKeys {
	return SidebarKeys{
		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "open")),
		Compose: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		Cmd:     key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
	}
}

// ShortHelp implements help.KeyMap for the sidebar context.
func (k SidebarKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Compose, k.Cmd}
}

// FullHelp implements help.KeyMap for the sidebar context.
func (k SidebarKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/glw907/Projects/beautiful-aerc && go build ./internal/ui/`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/keys.go
git commit -m "Add key bindings for global, message list, and sidebar contexts

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Tab interface (`internal/ui/tab.go`)

Defines the `Tab` interface that all tab types implement. The root model stores a slice of these.

**Files:**
- Create: `internal/ui/tab.go`

- [ ] **Step 1: Write the implementation**

Create `internal/ui/tab.go`:

```go
package ui

import tea "github.com/charmbracelet/bubbletea"

// Tab is a bubbletea model that renders inside the chrome shell.
// Each tab occupies the content area between the tab bar and status bar.
type Tab interface {
	tea.Model

	// Title returns the tab's display title (e.g., folder name).
	Title() string

	// Icon returns a Nerd Font icon for the tab bar.
	Icon() string

	// Closeable returns whether the user can close this tab.
	Closeable() bool
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/glw907/Projects/beautiful-aerc && go build ./internal/ui/`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/tab.go
git commit -m "Add Tab interface for chrome shell content area

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Mock backend (`internal/mail/mock.go`)

Implements `mail.Backend` with hardcoded data. Returns immediately, no goroutines. The AccountTab uses it to populate status bar counts. Stays permanently for development, testing, and demos.

**Files:**
- Create: `internal/mail/mock.go`
- Create: `internal/mail/mock_test.go`

- [ ] **Step 1: Write the test**

Create `internal/mail/mock_test.go`:

```go
package mail

import (
	"context"
	"testing"
)

func TestMockBackend(t *testing.T) {
	b := NewMockBackend()

	t.Run("connect succeeds", func(t *testing.T) {
		if err := b.Connect(context.Background()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
	})

	t.Run("list folders returns expected data", func(t *testing.T) {
		folders, err := b.ListFolders()
		if err != nil {
			t.Fatalf("ListFolders: %v", err)
		}
		if len(folders) == 0 {
			t.Fatal("expected at least one folder")
		}
		// Inbox should be first
		if folders[0].Name != "Inbox" {
			t.Errorf("first folder = %q, want Inbox", folders[0].Name)
		}
		if folders[0].Role != "inbox" {
			t.Errorf("Inbox role = %q, want inbox", folders[0].Role)
		}
	})

	t.Run("inbox has unread messages", func(t *testing.T) {
		folders, _ := b.ListFolders()
		inbox := folders[0]
		if inbox.Unseen == 0 {
			t.Error("expected Inbox to have unread messages")
		}
		if inbox.Exists == 0 {
			t.Error("expected Inbox to have messages")
		}
	})

	t.Run("fetch headers returns messages", func(t *testing.T) {
		msgs, err := b.FetchHeaders(nil)
		if err != nil {
			t.Fatalf("FetchHeaders: %v", err)
		}
		if len(msgs) == 0 {
			t.Fatal("expected at least one message")
		}
		for i, m := range msgs {
			if m.Subject == "" {
				t.Errorf("message %d has empty subject", i)
			}
			if m.From == "" {
				t.Errorf("message %d has empty from", i)
			}
		}
	})

	t.Run("updates channel is non-nil", func(t *testing.T) {
		ch := b.Updates()
		if ch == nil {
			t.Fatal("Updates() returned nil channel")
		}
	})

	t.Run("disconnect succeeds", func(t *testing.T) {
		if err := b.Disconnect(); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/mail/ -run TestMockBackend -v`
Expected: FAIL — `NewMockBackend` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/mail/mock.go`:

```go
package mail

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// MockBackend implements Backend with hardcoded data.
// Used for prototype development, testing, and demos.
type MockBackend struct {
	folders []Folder
	msgs    []MessageInfo
	updates chan Update
}

// NewMockBackend creates a MockBackend with realistic sample data.
func NewMockBackend() *MockBackend {
	return &MockBackend{
		folders: []Folder{
			{Name: "Inbox", Exists: 10, Unseen: 3, Role: "inbox"},
			{Name: "Drafts", Exists: 2, Unseen: 0, Role: "drafts"},
			{Name: "Sent", Exists: 145, Unseen: 0, Role: "sent"},
			{Name: "Archive", Exists: 1893, Unseen: 0, Role: "archive"},
			{Name: "Spam", Exists: 12, Unseen: 12, Role: "junk"},
			{Name: "Trash", Exists: 5, Unseen: 0, Role: "trash"},
			{Name: "Notifications", Exists: 47, Unseen: 0, Role: ""},
			{Name: "Remind", Exists: 8, Unseen: 0, Role: ""},
			{Name: "Lists/golang", Exists: 234, Unseen: 0, Role: ""},
			{Name: "Lists/rust", Exists: 89, Unseen: 0, Role: ""},
		},
		msgs: []MessageInfo{
			{UID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "10:23 AM", Flags: 0},
			{UID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "9:45 AM", Flags: 0},
			{UID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "9:12 AM", Flags: 0},
			{UID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "Yesterday", Flags: FlagSeen},
			{UID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "Yesterday", Flags: FlagSeen | FlagFlagged},
			{UID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "Yesterday", Flags: FlagSeen | FlagAnswered},
			{UID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "Apr 8", Flags: FlagSeen},
			{UID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "Apr 8", Flags: FlagSeen},
			{UID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "Apr 7", Flags: FlagSeen},
			{UID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "Apr 7", Flags: FlagSeen | FlagFlagged},
		},
		updates: make(chan Update),
	}
}

func (m *MockBackend) Connect(_ context.Context) error { return nil }
func (m *MockBackend) Disconnect() error               { return nil }

// ListFolders returns the hardcoded folder list.
func (m *MockBackend) ListFolders() ([]Folder, error) {
	return m.folders, nil
}

// OpenFolder is a no-op for the mock backend.
func (m *MockBackend) OpenFolder(_ string) error { return nil }

// FetchHeaders returns the hardcoded message list. The uids parameter is
// ignored — the mock always returns all messages.
func (m *MockBackend) FetchHeaders(_ []UID) ([]MessageInfo, error) {
	return m.msgs, nil
}

// FetchBody returns a placeholder body.
func (m *MockBackend) FetchBody(uid UID) (io.Reader, error) {
	return strings.NewReader(fmt.Sprintf("Mock body for message %s", uid)), nil
}

func (m *MockBackend) Search(_ SearchCriteria) ([]UID, error) { return nil, nil }
func (m *MockBackend) Move(_ []UID, _ string) error           { return nil }
func (m *MockBackend) Copy(_ []UID, _ string) error           { return nil }
func (m *MockBackend) Delete(_ []UID) error                   { return nil }
func (m *MockBackend) Flag(_ []UID, _ Flag, _ bool) error     { return nil }
func (m *MockBackend) MarkRead(_ []UID) error                 { return nil }
func (m *MockBackend) MarkAnswered(_ []UID) error             { return nil }

func (m *MockBackend) Send(_ string, _ []string, _ io.Reader) error {
	return nil
}

// Updates returns the update channel. The mock backend never sends updates.
func (m *MockBackend) Updates() <-chan Update {
	return m.updates
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/mail/ -run TestMockBackend -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/mail/mock.go internal/mail/mock_test.go
git commit -m "Add mock backend with hardcoded sample data

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: Tab bar renderer (`internal/ui/tab_bar.go`)

Renders the 3-row bubble tab bar. The active tab is a rounded bubble that opens into the content area below. Pure rendering function — no model, no state.

**Files:**
- Create: `internal/ui/tab_bar.go`
- Create: `internal/ui/tab_bar_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/tab_bar_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestRenderTabBar(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("single tab", func(t *testing.T) {
		tabs := []tabInfo{{title: "Inbox", icon: "\U000F01C6"}}
		result := renderTabBar(tabs, 0, 80, styles)
		lines := strings.Split(result, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
		if !strings.Contains(lines[0], "\u256D") {
			t.Error("row 1 missing top-left corner")
		}
		if !strings.Contains(lines[1], "Inbox") {
			t.Error("row 2 missing tab title")
		}
		if !strings.Contains(lines[2], "\u256F") {
			t.Error("row 3 missing bottom-left corner")
		}
	})

	t.Run("two tabs active first", func(t *testing.T) {
		tabs := []tabInfo{
			{title: "Inbox", icon: "\U000F01C6"},
			{title: "Re: Project update", icon: "\U000F01C6"},
		}
		result := renderTabBar(tabs, 0, 80, styles)
		if !strings.Contains(result, "Inbox") {
			t.Error("missing active tab title")
		}
		if !strings.Contains(result, "Re: Project update") {
			t.Error("missing inactive tab title")
		}
	})

	t.Run("two tabs active second", func(t *testing.T) {
		tabs := []tabInfo{
			{title: "Inbox", icon: "\U000F01C6"},
			{title: "Re: Project update", icon: "\U000F01C6"},
		}
		result := renderTabBar(tabs, 1, 80, styles)
		lines := strings.Split(result, "\n")
		// Row 2 should have Inbox as inactive before the active bubble
		if !strings.Contains(lines[1], "Inbox") {
			t.Error("row 2 missing inactive tab")
		}
	})

	t.Run("title truncation", func(t *testing.T) {
		tabs := []tabInfo{{
			title: "This is a very long subject line that exceeds thirty characters",
			icon:  "\U000F01C6",
		}}
		result := renderTabBar(tabs, 0, 80, styles)
		// The truncated title should end with ellipsis
		if !strings.Contains(result, "\u2026") {
			t.Error("long title not truncated with ellipsis")
		}
	})
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"short", "Inbox", 30, "Inbox"},
		{"exact", "123456789012345678901234567890", 30, "123456789012345678901234567890"},
		{"long", "1234567890123456789012345678901", 30, "12345678901234567890123456789\u2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestRenderTabBar -v`
Expected: FAIL — `renderTabBar` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/tab_bar.go`:

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// tabInfo holds the data needed to render a single tab in the tab bar.
type tabInfo struct {
	title string
	icon  string
}

// maxTabTitle is the maximum display width for a tab title.
const maxTabTitle = 30

// truncateTitle caps a title at max runes, appending "…" if truncated.
func truncateTitle(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "\u2026"
}

// renderTabBar renders the 3-row bubble tab bar.
//
// The active tab is a rounded bubble that opens into the content area:
//
//	Row 1:  ╭───────────╮
//	Row 2:  │ 󰇰  Inbox  │  Re: Project update
//	Row 3: ─╯           ╰──────────────────────────╮
func renderTabBar(tabs []tabInfo, active, width int, s Styles) string {
	if len(tabs) == 0 || width < 20 {
		return ""
	}

	// Build the active tab content: " icon  title "
	activeTab := tabs[active]
	activeTitle := truncateTitle(activeTab.title, maxTabTitle)
	activeContent := " " + activeTab.icon + "  " + activeTitle + " "
	activeInner := lipgloss.Width(activeContent)

	// Compute the left offset: sum of widths of inactive tabs before active
	var beforeParts []string
	for i := 0; i < active; i++ {
		t := truncateTitle(tabs[i].title, maxTabTitle)
		beforeParts = append(beforeParts, " "+tabs[i].icon+"  "+t+" ")
	}
	// Inactive tabs before active are rendered as plain text with " · " separators
	leftOffset := 0
	for _, p := range beforeParts {
		leftOffset += lipgloss.Width(p)
		leftOffset += 3 // " · " separator
	}

	// Build inactive tabs after active
	var afterParts []string
	for i := active + 1; i < len(tabs); i++ {
		t := truncateTitle(tabs[i].title, maxTabTitle)
		afterParts = append(afterParts, tabs[i].icon+"  "+t)
	}
	afterStr := ""
	if len(afterParts) > 0 {
		afterStr = "  " + strings.Join(afterParts, "  \u00B7  ")
	}

	border := s.TabActiveBorder
	activeText := s.TabActiveText
	inactiveText := s.TabInactiveText
	connectLine := s.TabConnectLine

	// Row 1: padding + ╭ + ─ fill + ╮
	row1Pad := strings.Repeat(" ", leftOffset)
	row1Inner := strings.Repeat("\u2500", activeInner)
	row1 := row1Pad + border.Render("\u256D"+row1Inner+"\u256E")
	row1 += strings.Repeat(" ", max(0, width-lipgloss.Width(row1)))

	// Row 2: inactive before + │ content │ + inactive after
	row2 := ""
	for i, p := range beforeParts {
		row2 += inactiveText.Render(p)
		if i < len(beforeParts)-1 {
			row2 += inactiveText.Render(" \u00B7 ")
		} else {
			row2 += inactiveText.Render(" \u00B7 ")
		}
	}
	row2 += border.Render("\u2502") + activeText.Render(activeContent) + border.Render("\u2502")
	if afterStr != "" {
		row2 += inactiveText.Render(afterStr)
	}
	row2 += strings.Repeat(" ", max(0, width-lipgloss.Width(row2)))

	// Row 3: ─╯ + spaces + ╰ + ─ fill + ╮
	row3Left := connectLine.Render(strings.Repeat("\u2500", max(1, leftOffset)) + "\u256F")
	row3Mid := strings.Repeat(" ", activeInner)
	rightFill := max(0, width-lipgloss.Width(row3Left)-activeInner-2)
	row3Right := connectLine.Render("\u2570" + strings.Repeat("\u2500", rightFill) + "\u256E")
	row3 := row3Left + row3Mid + row3Right

	return row1 + "\n" + row2 + "\n" + row3
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run "TestRenderTabBar|TestTruncateTitle" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/tab_bar.go internal/ui/tab_bar_test.go
git commit -m "Add 3-row bubble tab bar renderer

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 7: Status bar (`internal/ui/status_bar.go`)

One-row status bar between content and footer. Shows folder stats on the left and connection indicator on the right.

**Files:**
- Create: `internal/ui/status_bar.go`
- Create: `internal/ui/status_bar_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/status_bar_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestStatusBarView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("renders folder info and connection", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb.SetFolder("\U000F01C6", "Inbox", 10, 3)
		sb.SetConnected(true)
		result := sb.View(80)
		if !strings.Contains(result, "Inbox") {
			t.Error("missing folder name")
		}
		if !strings.Contains(result, "10") {
			t.Error("missing message count")
		}
		if !strings.Contains(result, "connected") {
			t.Error("missing connection indicator")
		}
	})

	t.Run("disconnected state", func(t *testing.T) {
		sb := NewStatusBar(styles)
		sb.SetFolder("\U000F01C6", "Inbox", 10, 3)
		sb.SetConnected(false)
		result := sb.View(80)
		if !strings.Contains(result, "offline") {
			t.Error("missing offline indicator")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestStatusBarView -v`
Expected: FAIL — `NewStatusBar` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/status_bar.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders a one-row status line with folder info and connection state.
type StatusBar struct {
	styles    Styles
	icon      string
	folder    string
	total     int
	unread    int
	connected bool
}

// NewStatusBar creates a StatusBar with the given styles.
func NewStatusBar(styles Styles) StatusBar {
	return StatusBar{
		styles:    styles,
		connected: true,
	}
}

// SetFolder updates the displayed folder information.
func (sb *StatusBar) SetFolder(icon, name string, total, unread int) {
	sb.icon = icon
	sb.folder = name
	sb.total = total
	sb.unread = unread
}

// SetConnected updates the connection state.
func (sb *StatusBar) SetConnected(connected bool) {
	sb.connected = connected
}

// View renders the status bar at the given width.
func (sb StatusBar) View(width int) string {
	// Left side: folder icon + name · count · unread
	left := fmt.Sprintf(" %s  %s \u00B7 %d messages", sb.icon, sb.folder, sb.total)
	if sb.unread > 0 {
		left += fmt.Sprintf(" \u00B7 %d unread", sb.unread)
	}

	// Right side: connection indicator
	var right string
	if sb.connected {
		dot := sb.styles.StatusConnected.Render("\u25CF")
		right = dot + " connected "
	} else {
		dot := sb.styles.StatusOffline.Render("\u25CF")
		right = dot + " offline "
	}

	// Fill the gap between left and right
	gap := max(0, width-lipgloss.Width(left)-lipgloss.Width(right))
	middle := strings.Repeat(" ", gap)

	return sb.styles.StatusBar.Render(left + middle + right)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestStatusBarView -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/status_bar.go internal/ui/status_bar_test.go
git commit -m "Add status bar with folder stats and connection indicator

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 8: Command footer (`internal/ui/footer.go`)

Renders context-appropriate keybinding hints using `bubbles/help`. The KeyMap swaps based on active tab type and focused panel.

**Files:**
- Create: `internal/ui/footer.go`
- Create: `internal/ui/footer_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/footer_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestFooterView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("message list context", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := f.View(120)
		if !strings.Contains(result, "del") {
			t.Error("missing delete hint")
		}
		if !strings.Contains(result, "compose") {
			t.Error("missing compose hint")
		}
	})

	t.Run("sidebar context", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(SidebarContext)
		result := f.View(120)
		if !strings.Contains(result, "open") {
			t.Error("missing open hint")
		}
		if !strings.Contains(result, "compose") {
			t.Error("missing compose hint")
		}
		// Sidebar should not show message list keys
		if strings.Contains(result, "del") {
			t.Error("sidebar should not show delete hint")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestFooterView -v`
Expected: FAIL — `NewFooter` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/footer.go`:

```go
package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// FooterContext identifies which keybinding set to display.
type FooterContext int

const (
	MsgListContext FooterContext = iota
	SidebarContext
)

// Footer renders context-appropriate keybinding hints.
type Footer struct {
	styles     Styles
	help       help.Model
	context    FooterContext
	msgKeys    MsgListKeys
	sidebarKeys SidebarKeys
}

// NewFooter creates a Footer with the given styles.
func NewFooter(styles Styles) Footer {
	h := help.New()
	h.ShortSeparator = "  "
	h.Styles.ShortKey = styles.FooterKey
	h.Styles.ShortDesc = styles.FooterHint
	h.Styles.ShortSeparator = styles.FooterHint

	return Footer{
		styles:      styles,
		help:        h,
		context:     MsgListContext,
		msgKeys:     NewMsgListKeys(),
		sidebarKeys: NewSidebarKeys(),
	}
}

// SetContext switches the displayed keybinding set.
func (f *Footer) SetContext(ctx FooterContext) {
	f.context = ctx
}

// View renders the footer at the given width.
func (f Footer) View(width int) string {
	f.help.Width = width

	var line string
	switch f.context {
	case SidebarContext:
		line = f.help.ShortHelpView(f.sidebarKeys.ShortHelp())
	default:
		line = f.help.ShortHelpView(f.msgKeys.ShortHelp())
	}

	return lipgloss.NewStyle().Width(width).Render(line)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestFooterView -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/footer.go internal/ui/footer_test.go
git commit -m "Add command footer with context-switching keybinding hints

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 9: AccountTab with focus cycling (`internal/ui/account_tab.go`)

The `AccountTab` implements the `Tab` interface. It owns a `focusedPanel` enum and renders a two-panel placeholder with `Tab` key focus cycling. The focused panel gets a `bg_selection` highlight. Both panels are centered `fg_dim` placeholders in 2.5b-1.

**Files:**
- Create: `internal/ui/account_tab.go`
- Create: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/account_tab_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestAccountTab(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()

	t.Run("title returns folder name", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.Title() != "Inbox" {
			t.Errorf("Title() = %q, want Inbox", tab.Title())
		}
	})

	t.Run("not closeable", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.Closeable() {
			t.Error("AccountTab should not be closeable")
		}
	})

	t.Run("tab key toggles focus", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.focused != SidebarPanel {
			t.Errorf("initial focus = %d, want SidebarPanel", tab.focused)
		}

		tab, _ = tab.Update(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != MsgListPanel {
			t.Errorf("after Tab, focus = %d, want MsgListPanel", tab.focused)
		}

		tab, _ = tab.Update(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != SidebarPanel {
			t.Errorf("after second Tab, focus = %d, want SidebarPanel", tab.focused)
		}
	})

	t.Run("view renders two panels with divider", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		result := tab.View()
		if !strings.Contains(result, "\u2502") {
			t.Error("missing panel divider")
		}
	})

	t.Run("resize propagates", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if tab.width != 120 {
			t.Errorf("width = %d, want 120", tab.width)
		}
		if tab.height != 40 {
			t.Errorf("height = %d, want 40", tab.height)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestAccountTab -v`
Expected: FAIL — `NewAccountTab` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/account_tab.go`:

```go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// Panel identifies which panel of the AccountTab is focused.
type Panel int

const (
	SidebarPanel Panel = iota
	MsgListPanel
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// AccountTab implements Tab for the main account view with sidebar and message list.
type AccountTab struct {
	styles  Styles
	backend mail.Backend
	focused Panel
	folder  string
	icon    string
	width   int
	height  int
}

// NewAccountTab creates an AccountTab using the given styles and backend.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
	return AccountTab{
		styles:  styles,
		backend: backend,
		focused: SidebarPanel,
		folder:  "Inbox",
		icon:    "\U000F01C6",
	}
}

// Title returns the current folder name.
func (m AccountTab) Title() string { return m.folder }

// Icon returns the folder's Nerd Font icon.
func (m AccountTab) Icon() string { return m.icon }

// Closeable returns false — the account tab cannot be closed.
func (m AccountTab) Closeable() bool { return false }

// Init returns no initial command.
func (m AccountTab) Init() tea.Cmd { return nil }

// Update handles key events and window size changes.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			if m.focused == SidebarPanel {
				m.focused = MsgListPanel
			} else {
				m.focused = SidebarPanel
			}
		}
	}
	return m, nil
}

// View renders the two-panel placeholder layout.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := min(sidebarWidth, m.width/2)
	mw := m.width - sw - 1 // -1 for divider

	sidebarContent := renderPlaceholder("Sidebar", sw, m.height, m.focused == SidebarPanel, m.styles)
	divider := renderDivider(m.height, m.styles)
	msglistContent := renderPlaceholder("Message List", mw, m.height, m.focused == MsgListPanel, m.styles)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, divider, msglistContent)
}

// renderPlaceholder renders a centered label in a panel of the given size.
func renderPlaceholder(label string, width, height int, focused bool, s Styles) string {
	style := s.Dim
	if focused {
		style = s.Dim.Background(s.Selection.GetBackground())
	}

	text := style.Render(label)
	// Center vertically
	topPad := max(0, (height-1)/2)
	botPad := max(0, height-1-topPad)
	// Center horizontally
	leftPad := max(0, (width-lipgloss.Width(text))/2)

	var lines []string
	emptyLine := strings.Repeat(" ", width)
	bgLine := emptyLine
	if focused {
		bgLine = lipgloss.NewStyle().
			Width(width).
			Background(s.Selection.GetBackground()).
			Render("")
	}

	for i := 0; i < topPad; i++ {
		if focused {
			lines = append(lines, bgLine)
		} else {
			lines = append(lines, emptyLine)
		}
	}
	centeredText := strings.Repeat(" ", leftPad) + text
	if focused {
		centeredText = lipgloss.NewStyle().
			Width(width).
			Background(s.Selection.GetBackground()).
			Render(strings.Repeat(" ", leftPad) + label)
	}
	lines = append(lines, centeredText)
	for i := 0; i < botPad; i++ {
		if focused {
			lines = append(lines, bgLine)
		} else {
			lines = append(lines, emptyLine)
		}
	}

	return strings.Join(lines, "\n")
}

// renderDivider renders a vertical line of │ characters.
func renderDivider(height int, s Styles) string {
	div := s.PanelDivider.Render("\u2502")
	lines := make([]string, height)
	for i := range lines {
		lines[i] = div
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestAccountTab -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "Add AccountTab with two-panel placeholder and focus cycling

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 10: Root App model (`internal/ui/app.go`)

The root bubbletea model. Owns tabs, styles, status bar, footer, and terminal dimensions. Handles global keys (tab switching, quit), delegates to active tab, composes the full-screen layout.

**Files:**
- Create: `internal/ui/app.go`
- Create: `internal/ui/app_test.go`

- [ ] **Step 1: Write the test**

Create `internal/ui/app_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestApp(t *testing.T) {
	backend := mail.NewMockBackend()

	t.Run("initial state", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		if len(app.tabs) != 1 {
			t.Fatalf("expected 1 tab, got %d", len(app.tabs))
		}
		if app.activeTab != 0 {
			t.Errorf("activeTab = %d, want 0", app.activeTab)
		}
	})

	t.Run("quit on q", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
		// Execute the cmd and check it produces a quit msg
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
	})

	t.Run("window size stored", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if app.width != 120 || app.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", app.width, app.height)
		}
	})

	t.Run("tab delegates to account tab", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		// Tab should toggle focus in the account tab
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
		acct := app.tabs[0].(AccountTab)
		if acct.focused != MsgListPanel {
			t.Errorf("after Tab, focused = %d, want MsgListPanel", acct.focused)
		}
	})

	t.Run("view renders all sections", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		view := app.View()
		// Should contain tab bar elements
		if !strings.Contains(view, "Inbox") {
			t.Error("view missing Inbox in tab bar")
		}
		// Should contain status bar elements
		if !strings.Contains(view, "connected") {
			t.Error("view missing connection indicator")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestApp -v`
Expected: FAIL — `NewApp` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/app.go`:

```go
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
)

// tabBarHeight is the number of rows the tab bar occupies.
const tabBarHeight = 3

// App is the root bubbletea model for poplar.
type App struct {
	tabs      []Tab
	activeTab int
	styles    Styles
	statusBar StatusBar
	footer    Footer
	keys      GlobalKeys
	width     int
	height    int
}

// NewApp creates the root model with a single AccountTab.
func NewApp(t *theme.CompiledTheme, backend mail.Backend) App {
	styles := NewStyles(t)
	acct := NewAccountTab(styles, backend)

	sb := NewStatusBar(styles)
	folders, _ := backend.ListFolders()
	if len(folders) > 0 {
		inbox := folders[0]
		sb.SetFolder(acct.Icon(), inbox.Name, inbox.Exists, inbox.Unseen)
	}
	sb.SetConnected(true)

	return App{
		tabs:      []Tab{acct},
		activeTab: 0,
		styles:    styles,
		statusBar: sb,
		footer:    NewFooter(styles),
		keys:      NewGlobalKeys(),
	}
}

// Init returns no initial command.
func (m App) Init() tea.Cmd { return nil }

// Update handles global keys and delegates to the active tab.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to active tab with content area dimensions
		contentHeight := m.contentHeight()
		tabMsg := tea.WindowSizeMsg{Width: m.width, Height: contentHeight}
		tab, cmd := m.tabs[m.activeTab].Update(tabMsg)
		m.tabs[m.activeTab] = tab.(Tab)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.Runes[0]-'0') - 1
			if idx < len(m.tabs) {
				m.activeTab = idx
				m.updateFooterContext()
			}
			return m, nil
		case "?":
			// Stubbed for 2.5b-5 (help popover)
			return m, nil
		case ":":
			// Stubbed for 2.5b-7 (command mode)
			return m, nil
		}
	}

	// Delegate to active tab
	if len(m.tabs) > 0 {
		tab, cmd := m.tabs[m.activeTab].Update(msg)
		m.tabs[m.activeTab] = tab.(Tab)
		cmds = append(cmds, cmd)

		// Update footer context based on AccountTab focus
		m.updateFooterContext()
	}

	return m, tea.Batch(cmds...)
}

// View composes the full-screen layout.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Build tab info for the tab bar
	tabs := make([]tabInfo, len(m.tabs))
	for i, t := range m.tabs {
		tabs[i] = tabInfo{title: t.Title(), icon: t.Icon()}
	}

	tabBar := renderTabBar(tabs, m.activeTab, m.width, m.styles)
	content := m.tabs[m.activeTab].View()
	status := m.statusBar.View(m.width)
	foot := m.footer.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		content,
		status,
		foot,
	)
}

// contentHeight returns the height available for the content area.
func (m App) contentHeight() int {
	// tab bar (3) + status bar (1) + footer (1)
	chrome := tabBarHeight + 2
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}

// updateFooterContext switches the footer KeyMap based on the active tab's focus.
func (m *App) updateFooterContext() {
	if acct, ok := m.tabs[m.activeTab].(AccountTab); ok {
		if acct.focused == SidebarPanel {
			m.footer.SetContext(SidebarContext)
		} else {
			m.footer.SetContext(MsgListContext)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/beautiful-aerc && go test ./internal/ui/ -run TestApp -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Add root App model with layout composition and global keys

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 11: Rewrite `cmd/poplar/root.go` for bubbletea

Replace the diagnostic stub with `tea.NewProgram` startup. The root command creates a `MockBackend`, builds the `App` model, and runs the fullscreen bubbletea program.

**Files:**
- Modify: `cmd/poplar/root.go`

- [ ] **Step 1: Rewrite root.go**

Replace the contents of `cmd/poplar/root.go` with:

```go
package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/glw907/beautiful-aerc/internal/ui"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "poplar",
		Short:        "A bubbletea-based terminal email client",
		SilenceUsage: true,
		RunE:         runRoot,
	}
	return cmd
}

// appModel wraps ui.App to satisfy tea.Model (returns tea.Model, not App).
type appModel struct {
	app ui.App
}

func (m appModel) Init() tea.Cmd { return m.app.Init() }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	app, cmd := m.app.Update(msg)
	m.app = app
	return m, cmd
}

func (m appModel) View() string { return m.app.View() }

func runRoot(_ *cobra.Command, _ []string) error {
	backend := mail.NewMockBackend()
	app := ui.NewApp(theme.Nord, backend)

	p := tea.NewProgram(appModel{app: app}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running poplar: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `cd /home/glw907/Projects/beautiful-aerc && make build`
Expected: All four binaries build without errors.

- [ ] **Step 3: Run make check**

Run: `cd /home/glw907/Projects/beautiful-aerc && make check`
Expected: `go vet` and `go test` both pass.

- [ ] **Step 4: Commit**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add cmd/poplar/root.go
git commit -m "Rewrite poplar entry point to launch bubbletea TUI

Replace the diagnostic folder-listing stub with a fullscreen
bubbletea program using MockBackend and the Nord theme.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 12: Manual testing and polish

Run the prototype, verify all gate conditions, and fix any visual issues.

**Files:**
- Possibly modify any `internal/ui/*.go` file for visual fixes

- [ ] **Step 1: Install and run**

```bash
cd /home/glw907/Projects/beautiful-aerc && make install && poplar
```

Verify interactively:
1. Fullscreen bubbletea program launches
2. Tab bar renders with 3-row bubble style (single tab showing "Inbox")
3. Content area shows two-panel placeholder with `│` divider
4. Status bar shows "Inbox · 10 messages · 3 unread" + "● connected"
5. Command footer shows message list keybindings
6. `Tab` key cycles focus between panels (visible `bg_selection` highlight)
7. `q` exits cleanly
8. All colors come from Nord theme

- [ ] **Step 2: Fix any visual issues found**

Adjust tab bar spacing, status bar alignment, panel sizing, or divider rendering as needed. Each fix should be small and targeted.

- [ ] **Step 3: Run make check one final time**

Run: `cd /home/glw907/Projects/beautiful-aerc && make check`
Expected: All tests pass, all vet checks clean.

- [ ] **Step 4: Commit any fixes**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add -p  # stage only changed ui files
git commit -m "Fix visual polish from manual testing

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 13: Update docs and status

Update STATUS.md, architecture.md, and prepare the next pass starter prompt.

**Files:**
- Modify: `docs/poplar/STATUS.md`
- Modify: `docs/poplar/architecture.md`

- [ ] **Step 1: Update STATUS.md**

Mark Pass 2.5b-1 as done. Update the next steps section to point to Pass 2.5b-2 (sidebar). Write the next starter prompt:

```markdown
| 2.5b-1 | Prototype: chrome shell | done |
```

Next starter prompt:

> Start Pass 2.5b-2: sidebar prototype. Read the wireframes at
> `docs/poplar/wireframes.md`, the chrome shell design spec at
> `docs/superpowers/specs/2026-04-10-poplar-chrome-shell-design.md`,
> and the architecture doc at `docs/poplar/architecture.md`. Write a
> plan first, then build the sidebar: folder groups, selection,
> unread badges, and g-prefix jumps.

- [ ] **Step 2: Update architecture.md**

Add any new architectural decisions made during implementation. At minimum:

- **appModel wrapper in cmd/poplar**: `ui.App.Update` returns `(App, tea.Cmd)` (typed, per Elm convention). The `appModel` wrapper in `cmd/poplar/root.go` satisfies `tea.Model`'s `(tea.Model, tea.Cmd)` return type. This keeps the UI layer free of interface-return overhead.

- [ ] **Step 3: Commit docs**

```bash
cd /home/glw907/Projects/beautiful-aerc
git add docs/poplar/STATUS.md docs/poplar/architecture.md
git commit -m "Mark Pass 2.5b-1 done, set up Pass 2.5b-2 starter prompt

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 4: Push**

```bash
cd /home/glw907/Projects/beautiful-aerc && git push
```
