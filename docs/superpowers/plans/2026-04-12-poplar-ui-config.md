# Pass 2.5b-3.5: UI Config + Sidebar Polish + Elm Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce the first `[ui]` config section, polish the sidebar
with rank/label/hide/nested-indent, add a `poplar config init`
subcommand for folder discovery, and convert synchronous backend I/O to
`tea.Cmd` flow — establishing the patterns Pass 3 will depend on.

**Architecture:** Seven sequential phases (A–G). Each phase leaves the
tree compilable with passing tests so work can resume from any
checkpoint. Phases A–C add new primitives (config package move, classifier,
UIConfig). Phase D reworks the sidebar to consume them. Phase E is the
Elm refactor (Cmd-based I/O, FolderChangedMsg, dead stub removal).
Phase F adds the `config init` subcommand. Phase G updates the shipped
example TOML and the doc set.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, BurntSushi/toml, cobra.

**Spec:** `docs/superpowers/specs/2026-04-12-poplar-ui-config-design.md`

---

## File Structure

New files:

- `internal/config/account.go` — `AccountConfig` struct (moved from `internal/poplar/config.go`)
- `internal/config/accounts.go` — `ParseAccounts`, credential injection (moved from `internal/poplar/accounts.go`)
- `internal/config/accounts_test.go` — moved test
- `internal/config/ui.go` — `UIConfig`, `FolderConfig`, `LoadUI` loader
- `internal/config/ui_test.go` — table-driven UIConfig tests
- `internal/config/writer.go` — hand-rolled commented TOML writer for `config init`
- `internal/config/writer_test.go` — golden-file tests for the writer
- `internal/mail/classify.go` — `Classify`, `ClassifiedFolder`, alias table, `Group` enum
- `internal/mail/classify_test.go` — table-driven classifier tests
- `internal/ui/cmds.go` — `foldersLoadedMsg`, `folderLoadedMsg`, `backendErrMsg`, `FolderChangedMsg`, Cmd constructors
- `internal/ui/account_tab_test.go` — Init/Update Cmd dispatch tests
- `cmd/poplar/config.go` — `poplar config` parent cobra command
- `cmd/poplar/config_init.go` — `poplar config init` subcommand
- `cmd/poplar/config_init_test.go` — dry-run + merge + idempotence tests

Modified files:

- `internal/ui/sidebar.go` — consume `[]mail.ClassifiedFolder` + `config.UIConfig`; rank, label, hide, nested indent
- `internal/ui/sidebar_test.go` — add cases for new features (create if absent)
- `internal/ui/account_tab.go` — new signature `NewAccountTab(styles, backend, uiCfg)`; Cmd-based `Init`/`Update`; `handleKey` returns `(AccountTab, tea.Cmd)`
- `internal/ui/app.go` — drop `ListFolders` call, drop `syncStatusBar` grandchild peek, drop `:` stub, consume `FolderChangedMsg`
- `internal/mail/mock.go` — rename `Spam` → `Junk` to exercise alias classification
- `internal/mail/jmap.go` — import path rename `internal/poplar` → `internal/config`
- `internal/aercfork/worker/jmap/worker.go` — same import rename
- `internal/aercfork/worker/types/messages.go` — same import rename
- `cmd/poplar/root.go` — call `config.ParseAccounts` + `config.LoadUI`; pass `uiCfg` to `ui.NewApp`; register `config` subcommand
- `.config/aerc/` — N/A (poplar config lives at `~/.config/poplar/`, and the shipped example lives in the repo — see Phase G)

Deleted files (end of Phase A):

- `internal/poplar/config.go`
- `internal/poplar/accounts.go`
- `internal/poplar/accounts_test.go`
- `internal/poplar/` directory itself

Doc updates (Phase G):

- `docs/poplar/architecture.md` — 3 new decision entries, 2 note-backs on existing entries
- `docs/poplar/keybindings.md` — Select section reword, Threads (reserved) section
- `docs/poplar/wireframes.md` — replace 2 TBD references with Pass 2.5b-3.6 pointer
- `docs/poplar/STATUS.md` — mark 2.5b-3.5 done, bump next pass pointer
- Repo example config at `accounts.toml.example` (new) — shipped with canonical subsections pre-filled

---

## Phase A — Package Move (`internal/poplar/` → `internal/config/`)

Mechanical move. No behavior changes. Tree stays green after commit.

### Task A1: Create `internal/config/account.go`

**Files:**
- Create: `internal/config/account.go`

- [ ] **Step 1: Create the new file with `AccountConfig` copied verbatim from `internal/poplar/config.go`, changing only the package line**

```go
// Package config holds poplar's configuration types and loaders.
package config

import (
	"time"

	"github.com/emersion/go-message/mail"
)

// AccountConfig holds the configuration for a single email account.
// This replaces aerc's config.AccountConfig with only the fields
// that the forked workers actually use.
type AccountConfig struct {
	Name           string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string
	Headers        []string
	HeadersExclude []string
	CheckMail      time.Duration

	// Identity
	From    *mail.Address
	Aliases []*mail.Address
	CopyTo  []string

	// Outgoing
	Outgoing        string
	OutgoingCredCmd string
}
```

- [ ] **Step 2: Verify it compiles standalone**

Run: `go build ./internal/config/...`
Expected: success (the old package still exists and nothing imports the new one yet).

### Task A2: Create `internal/config/accounts.go`

**Files:**
- Create: `internal/config/accounts.go`

- [ ] **Step 1: Copy `internal/poplar/accounts.go` verbatim, changing only the package line**

Replace `package poplar` with `package config`. Leave everything else
(imports, struct names, function bodies) untouched. The file references
`AccountConfig` unqualified, which now resolves to the one in `account.go`.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/config/...`
Expected: success.

### Task A3: Create `internal/config/accounts_test.go`

**Files:**
- Create: `internal/config/accounts_test.go`

- [ ] **Step 1: Copy `internal/poplar/accounts_test.go` verbatim, changing only the package line**

Replace `package poplar` with `package config`.

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/config/...`
Expected: PASS (3 tests: `TestParseAccounts`, `TestParseAccountsCredentialInjection`, `TestParseAccountsFields`).

### Task A4: Update import sites

**Files:**
- Modify: `cmd/poplar/root.go`
- Modify: `internal/mail/jmap.go`
- Modify: `internal/aercfork/worker/jmap/worker.go`
- Modify: `internal/aercfork/worker/types/messages.go`

- [ ] **Step 1: In each file, rename the import from `internal/poplar` to `internal/config` and rename `poplar.AccountConfig` → `config.AccountConfig`**

For `cmd/poplar/root.go`:

Check whether it imports `internal/poplar` at all. It does not currently (it
calls `mail.NewMockBackend` and `ui.NewApp` with no explicit AccountConfig).
Skip this file for now — Phase E (Elm refactor) will add the import when it
wires `config.LoadUI`.

For `internal/mail/jmap.go`:

Replace:
```go
	"github.com/glw907/beautiful-aerc/internal/poplar"
```
with:
```go
	"github.com/glw907/beautiful-aerc/internal/config"
```

Replace `*poplar.AccountConfig` with `*config.AccountConfig` at every
occurrence in the file (two sites: the struct field `config *poplar.AccountConfig`
will collide with the new package alias `config`, so rename the field to
`acctCfg` — grep the file to catch every reference).

```go
type JMAPAdapter struct {
	acctCfg *config.AccountConfig
	w       *types.Worker
	updates chan Update
	done    chan struct{}
}

func NewJMAPAdapter(cfg *config.AccountConfig) (*JMAPAdapter, error) {
	w, err := worker.NewWorker(cfg.Source, cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("creating worker: %w", err)
	}
	return &JMAPAdapter{
		acctCfg: cfg,
		w:       w,
		updates: make(chan Update, 50),
		done:    make(chan struct{}),
	}, nil
}
```

Also update the Connect method that references `a.config`:
```go
	if err := a.doAction(&types.Configure{Config: a.acctCfg}); err != nil {
```

For `internal/aercfork/worker/jmap/worker.go`:

Replace the import and change `*poplar.AccountConfig` → `*config.AccountConfig`.
Check for a field named `config` that collides with the package alias. If
present, rename it to `acctCfg` throughout this file.

For `internal/aercfork/worker/types/messages.go`:

Replace the import and change `*poplar.AccountConfig` → `*config.AccountConfig`
on the `Configure.Config` field.

- [ ] **Step 2: Verify the build**

Run: `go build ./...`
Expected: success.

Run: `go vet ./...`
Expected: no output.

Run: `go test ./...`
Expected: all tests pass (including the duplicated ones in both
`internal/poplar/` and `internal/config/` — we haven't deleted the old
package yet).

### Task A5: Delete `internal/poplar/`

**Files:**
- Delete: `internal/poplar/config.go`
- Delete: `internal/poplar/accounts.go`
- Delete: `internal/poplar/accounts_test.go`
- Delete: `internal/poplar/` (empty directory)

- [ ] **Step 1: Delete the files and directory**

Run: `rm -r internal/poplar/`

- [ ] **Step 2: Verify no stragglers reference `internal/poplar`**

Run: `grep -r "internal/poplar" --include='*.go' .`
Expected: no matches.

- [ ] **Step 3: Final build check**

Run: `make check`
Expected: PASS (vet clean, all tests pass).

### Task A6: Commit Phase A

- [ ] **Step 1: Stage and commit**

```bash
git add internal/config/ internal/mail/jmap.go internal/aercfork/
git add -u  # picks up deletions under internal/poplar/
git commit -m "$(cat <<'EOF'
Move internal/poplar to internal/config

Mechanical rename in preparation for UIConfig alongside AccountConfig.
No behavior change.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase B — UIConfig Type and Loader

Add the new config structures and a loader that reads the `[ui]` table
from the same `accounts.toml` file.

### Task B1: Define `UIConfig` and `FolderConfig` types

**Files:**
- Create: `internal/config/ui.go`

- [ ] **Step 1: Write the type declarations**

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// UIConfig holds poplar's UI tuning. Currently scoped to folder/sidebar
// behavior. Populated from the [ui] table in accounts.toml.
type UIConfig struct {
	// Threading is the default threading state for folders that do not
	// specify a per-folder override. Default true. Consumer ships in
	// Pass 2.5b-3.6; this pass parses and stores only.
	Threading bool

	// Folders holds per-folder overrides keyed by canonical name for
	// canonical folders (Inbox, Drafts, Sent, Archive, Spam, Trash) or
	// literal provider name for custom folders.
	Folders map[string]FolderConfig
}

// FolderConfig holds per-folder overrides from [ui.folders.<name>]
// subsections. Any field left at its zero value is treated as "unset"
// and falls back to the group default.
type FolderConfig struct {
	// Rank is the within-group sort key. Zero means "use group default".
	// Lower values sort first. Ties break on display name.
	Rank int

	// RankSet distinguishes "unset" from "explicit 0".
	RankSet bool

	// Label overrides the display name. Empty = use default
	// (canonical name for canonicals, provider name for custom).
	Label string

	// Threading overrides the global threading default when Set.
	// Consumer ships in Pass 2.5b-3.6.
	Threading    bool
	ThreadingSet bool

	// Sort is the per-folder sort order. Empty = "date-desc". Consumer
	// ships in Pass 2.5b-3.6.
	Sort string

	// Hide drops the folder from the sidebar entirely.
	Hide bool
}

// DefaultUIConfig returns an empty UIConfig with sensible defaults.
// Use as a fallback when accounts.toml has no [ui] section.
func DefaultUIConfig() UIConfig {
	return UIConfig{
		Threading: true,
		Folders:   map[string]FolderConfig{},
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/config/...`
Expected: success.

### Task B2: Write the failing `LoadUI` tests

**Files:**
- Create: `internal/config/ui_test.go`

- [ ] **Step 1: Write the table-driven test file**

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUI(t *testing.T) {
	tests := []struct {
		name     string
		toml     string
		want     UIConfig
		wantErr  string
	}{
		{
			name: "missing [ui] section uses defaults",
			toml: `[[account]]
name = "X"
source = "jmap://x@y"
`,
			want: UIConfig{
				Threading: true,
				Folders:   map[string]FolderConfig{},
			},
		},
		{
			name: "empty [ui] section uses defaults",
			toml: `[ui]
`,
			want: UIConfig{
				Threading: true,
				Folders:   map[string]FolderConfig{},
			},
		},
		{
			name: "global threading override",
			toml: `[ui]
threading = false
`,
			want: UIConfig{
				Threading: false,
				Folders:   map[string]FolderConfig{},
			},
		},
		{
			name: "per-folder rank and threading",
			toml: `[ui]
threading = true

[ui.folders.Inbox]
rank = 1
threading = false
sort = "date-asc"
`,
			want: UIConfig{
				Threading: true,
				Folders: map[string]FolderConfig{
					"Inbox": {
						Rank:         1,
						RankSet:      true,
						Threading:    false,
						ThreadingSet: true,
						Sort:         "date-asc",
					},
				},
			},
		},
		{
			name: "per-folder label and hide",
			toml: `[ui.folders."[Gmail]/All Mail"]
hide = true

[ui.folders."[Gmail]/Starred"]
label = "Starred"
rank = 5
`,
			want: UIConfig{
				Threading: true,
				Folders: map[string]FolderConfig{
					"[Gmail]/All Mail": {Hide: true},
					"[Gmail]/Starred":  {Label: "Starred", Rank: 5, RankSet: true},
				},
			},
		},
		{
			name: "invalid sort value rejected",
			toml: `[ui.folders.Inbox]
sort = "alphabetical"
`,
			wantErr: `invalid sort "alphabetical"`,
		},
		{
			name: "negative rank is valid",
			toml: `[ui.folders.Pinned]
rank = -10
`,
			want: UIConfig{
				Threading: true,
				Folders: map[string]FolderConfig{
					"Pinned": {Rank: -10, RankSet: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "accounts.toml")
			if err := os.WriteFile(path, []byte(tt.toml), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := LoadUI(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Threading != tt.want.Threading {
				t.Errorf("Threading = %v, want %v", got.Threading, tt.want.Threading)
			}
			if len(got.Folders) != len(tt.want.Folders) {
				t.Fatalf("Folders len = %d, want %d (got %+v)", len(got.Folders), len(tt.want.Folders), got.Folders)
			}
			for k, wantFC := range tt.want.Folders {
				gotFC, ok := got.Folders[k]
				if !ok {
					t.Errorf("missing folder %q", k)
					continue
				}
				if gotFC != wantFC {
					t.Errorf("folder %q = %+v, want %+v", k, gotFC, wantFC)
				}
			}
		})
	}
}

func TestLoadUIMissingFile(t *testing.T) {
	_, err := LoadUI("/nonexistent/accounts.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "reading ui config") {
		t.Errorf("expected 'reading ui config' in error, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the tests — expect compile failure**

Run: `go test ./internal/config/... -run LoadUI`
Expected: compile error (`LoadUI` undefined).

### Task B3: Implement `LoadUI`

**Files:**
- Modify: `internal/config/ui.go`

- [ ] **Step 1: Add `LoadUI` plus the raw TOML-shape types**

Append to `internal/config/ui.go`:

```go
// rawUI is the on-disk shape of the [ui] table. It uses pointers for
// optional bool fields so we can distinguish "unset" from "explicit false".
type rawUI struct {
	Threading *bool                   `toml:"threading"`
	Folders   map[string]rawFolderCfg `toml:"folders"`
}

type rawFolderCfg struct {
	Rank      *int    `toml:"rank"`
	Label     string  `toml:"label"`
	Threading *bool   `toml:"threading"`
	Sort      string  `toml:"sort"`
	Hide      bool    `toml:"hide"`
}

type rawUIFile struct {
	UI rawUI `toml:"ui"`
}

// LoadUI reads the [ui] table from an accounts.toml file and returns
// a UIConfig. A missing file is an error; a missing [ui] section
// returns DefaultUIConfig().
func LoadUI(path string) (UIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UIConfig{}, fmt.Errorf("reading ui config: %w", err)
	}

	var raw rawUIFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return UIConfig{}, fmt.Errorf("parsing ui config: %w", err)
	}

	out := DefaultUIConfig()
	if raw.UI.Threading != nil {
		out.Threading = *raw.UI.Threading
	}

	for name, fc := range raw.UI.Folders {
		converted, err := convertFolderCfg(name, fc)
		if err != nil {
			return UIConfig{}, err
		}
		out.Folders[name] = converted
	}

	return out, nil
}

func convertFolderCfg(name string, raw rawFolderCfg) (FolderConfig, error) {
	out := FolderConfig{
		Label: raw.Label,
		Sort:  raw.Sort,
		Hide:  raw.Hide,
	}
	if raw.Rank != nil {
		out.Rank = *raw.Rank
		out.RankSet = true
	}
	if raw.Threading != nil {
		out.Threading = *raw.Threading
		out.ThreadingSet = true
	}
	if raw.Sort != "" && raw.Sort != "date-asc" && raw.Sort != "date-desc" {
		return FolderConfig{}, fmt.Errorf(
			"ui.folders.%q: invalid sort %q (want \"date-asc\" or \"date-desc\")",
			name, raw.Sort)
	}
	return out, nil
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/config/... -run LoadUI -v`
Expected: all subtests PASS.

- [ ] **Step 3: Run the full config package test**

Run: `go test ./internal/config/...`
Expected: PASS.

### Task B4: Commit Phase B

- [ ] **Step 1: Stage and commit**

```bash
git add internal/config/ui.go internal/config/ui_test.go
git commit -m "$(cat <<'EOF'
Add UIConfig and LoadUI to internal/config

Parses the [ui] table from accounts.toml into UIConfig with global
threading default and per-folder overrides (rank, label, threading,
sort, hide). Threading and sort fields are parsed but unused until
Pass 2.5b-3.6 wires the consumer.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase C — Folder Classifier

New `internal/mail/classify.go` maps backend folders into
Primary/Disposal/Custom groups via role detection and an alias table.

### Task C1: Write the failing classifier tests

**Files:**
- Create: `internal/mail/classify_test.go`

- [ ] **Step 1: Write the tests**

```go
package mail

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		input   []Folder
		want    []ClassifiedFolder
	}{
		{
			name:  "empty input",
			input: nil,
			want:  nil,
		},
		{
			name: "role attribute wins",
			input: []Folder{
				{Name: "Bandeja de entrada", Role: "inbox"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Bandeja de entrada", Role: "inbox"},
					Canonical: "Inbox", DisplayName: "Inbox", Group: GroupPrimary},
			},
		},
		{
			name: "inbox by alias (no role)",
			input: []Folder{
				{Name: "INBOX"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "INBOX"},
					Canonical: "Inbox", DisplayName: "Inbox", Group: GroupPrimary},
			},
		},
		{
			name: "gmail sent mail via alias",
			input: []Folder{
				{Name: "[Gmail]/Sent Mail"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "[Gmail]/Sent Mail"},
					Canonical: "Sent", DisplayName: "Sent", Group: GroupPrimary},
			},
		},
		{
			name: "outlook sent items",
			input: []Folder{
				{Name: "Sent Items"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Sent Items"},
					Canonical: "Sent", DisplayName: "Sent", Group: GroupPrimary},
			},
		},
		{
			name: "icloud deleted messages",
			input: []Folder{
				{Name: "Deleted Messages"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Deleted Messages"},
					Canonical: "Trash", DisplayName: "Trash", Group: GroupDisposal},
			},
		},
		{
			name: "junk as spam",
			input: []Folder{
				{Name: "Junk", Unseen: 12},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Junk", Unseen: 12},
					Canonical: "Spam", DisplayName: "Spam", Group: GroupDisposal},
			},
		},
		{
			name: "gmail all mail → archive",
			input: []Folder{
				{Name: "All Mail"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "All Mail"},
					Canonical: "Archive", DisplayName: "Archive", Group: GroupPrimary},
			},
		},
		{
			name: "outlook junk email",
			input: []Folder{
				{Name: "Junk Email"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Junk Email"},
					Canonical: "Spam", DisplayName: "Spam", Group: GroupDisposal},
			},
		},
		{
			name: "gmail starred is custom",
			input: []Folder{
				{Name: "[Gmail]/Starred"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "[Gmail]/Starred"},
					Canonical: "", DisplayName: "[Gmail]/Starred", Group: GroupCustom},
			},
		},
		{
			name: "nested custom folder",
			input: []Folder{
				{Name: "Lists/golang"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Lists/golang"},
					Canonical: "", DisplayName: "Lists/golang", Group: GroupCustom},
			},
		},
		{
			name: "all six canonicals + one custom",
			input: []Folder{
				{Name: "Inbox", Role: "inbox"},
				{Name: "Drafts", Role: "drafts"},
				{Name: "Sent", Role: "sent"},
				{Name: "Archive", Role: "archive"},
				{Name: "Junk", Role: "junk"},
				{Name: "Trash", Role: "trash"},
				{Name: "Lists/golang"},
			},
			want: []ClassifiedFolder{
				{Folder: Folder{Name: "Inbox", Role: "inbox"}, Canonical: "Inbox", DisplayName: "Inbox", Group: GroupPrimary},
				{Folder: Folder{Name: "Drafts", Role: "drafts"}, Canonical: "Drafts", DisplayName: "Drafts", Group: GroupPrimary},
				{Folder: Folder{Name: "Sent", Role: "sent"}, Canonical: "Sent", DisplayName: "Sent", Group: GroupPrimary},
				{Folder: Folder{Name: "Archive", Role: "archive"}, Canonical: "Archive", DisplayName: "Archive", Group: GroupPrimary},
				{Folder: Folder{Name: "Junk", Role: "junk"}, Canonical: "Spam", DisplayName: "Spam", Group: GroupDisposal},
				{Folder: Folder{Name: "Trash", Role: "trash"}, Canonical: "Trash", DisplayName: "Trash", Group: GroupDisposal},
				{Folder: Folder{Name: "Lists/golang"}, Canonical: "", DisplayName: "Lists/golang", Group: GroupCustom},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Classify() mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/mail/... -run Classify`
Expected: compile error (`Classify`, `ClassifiedFolder`, `Group*` undefined).

### Task C2: Implement the classifier

**Files:**
- Create: `internal/mail/classify.go`

- [ ] **Step 1: Write the implementation**

```go
package mail

import "strings"

// Group is the visual grouping a classified folder falls into.
type Group int

const (
	GroupPrimary  Group = iota // Inbox, Drafts, Sent, Archive
	GroupDisposal              // Spam, Trash
	GroupCustom                // everything else
)

// ClassifiedFolder wraps a backend Folder with its canonical identity
// (when recognized), display name, and visual group.
type ClassifiedFolder struct {
	Folder      Folder
	Canonical   string // "Inbox", "Drafts", ... or "" for custom
	DisplayName string // canonical name, or provider name for custom
	Group       Group
}

// Classify maps raw backend folders into ClassifiedFolders.
// Priority: role attribute, then alias table, then Custom fallback.
// Matching is case-insensitive exact match on the provider name.
// Order of the input is preserved in the output — callers that want
// group ordering (sidebar) do that themselves.
func Classify(folders []Folder) []ClassifiedFolder {
	if len(folders) == 0 {
		return nil
	}
	out := make([]ClassifiedFolder, 0, len(folders))
	for _, f := range folders {
		out = append(out, classifyOne(f))
	}
	return out
}

func classifyOne(f Folder) ClassifiedFolder {
	if canonical := canonicalFromRole(f.Role); canonical != "" {
		return ClassifiedFolder{
			Folder:      f,
			Canonical:   canonical,
			DisplayName: canonical,
			Group:       groupOf(canonical),
		}
	}
	if canonical := canonicalFromAlias(f.Name); canonical != "" {
		return ClassifiedFolder{
			Folder:      f,
			Canonical:   canonical,
			DisplayName: canonical,
			Group:       groupOf(canonical),
		}
	}
	return ClassifiedFolder{
		Folder:      f,
		Canonical:   "",
		DisplayName: f.Name,
		Group:       GroupCustom,
	}
}

func canonicalFromRole(role string) string {
	switch strings.ToLower(role) {
	case "inbox":
		return "Inbox"
	case "drafts":
		return "Drafts"
	case "sent":
		return "Sent"
	case "archive":
		return "Archive"
	case "junk":
		return "Spam"
	case "trash":
		return "Trash"
	}
	return ""
}

// aliasTable maps lowercased provider names to canonical names.
// Verified against Gmail, Fastmail, Outlook/M365, iCloud, Yahoo/AOL,
// Proton Mail Bridge.
var aliasTable = map[string]string{
	"inbox": "Inbox",

	"drafts":         "Drafts",
	"draft":          "Drafts",
	"[gmail]/drafts": "Drafts",

	"sent":             "Sent",
	"sent mail":        "Sent",
	"sent items":       "Sent",
	"sent messages":    "Sent",
	"[gmail]/sent mail": "Sent",

	"archive":          "Archive",
	"all mail":         "Archive",
	"[gmail]/all mail": "Archive",

	"spam":          "Spam",
	"junk":          "Spam",
	"junk email":    "Spam",
	"junk e-mail":   "Spam",
	"bulk mail":     "Spam",
	"[gmail]/spam":  "Spam",

	"trash":             "Trash",
	"deleted":           "Trash",
	"deleted items":     "Trash",
	"deleted messages":  "Trash",
	"bin":               "Trash",
	"[gmail]/trash":     "Trash",
}

func canonicalFromAlias(name string) string {
	return aliasTable[strings.ToLower(name)]
}

func groupOf(canonical string) Group {
	switch canonical {
	case "Inbox", "Drafts", "Sent", "Archive":
		return GroupPrimary
	case "Spam", "Trash":
		return GroupDisposal
	}
	return GroupCustom
}
```

- [ ] **Step 2: Run the classifier tests**

Run: `go test ./internal/mail/... -run Classify -v`
Expected: all subtests PASS.

- [ ] **Step 3: Run the full mail package tests**

Run: `go test ./internal/mail/...`
Expected: PASS.

### Task C3: Update the mock backend to exercise alias path

**Files:**
- Modify: `internal/mail/mock.go`

- [ ] **Step 1: Rename `Spam` folder → `Junk`**

In `internal/mail/mock.go`, change:
```go
{Name: "Spam", Exists: 12, Unseen: 12, Role: "junk"},
```
to:
```go
{Name: "Junk", Exists: 12, Unseen: 12, Role: ""},
```

Dropping the role forces classification through the alias path.

- [ ] **Step 2: Verify the mock still builds and tests pass**

Run: `go test ./internal/mail/...`
Expected: PASS.

### Task C4: Commit Phase C

- [ ] **Step 1: Stage and commit**

```bash
git add internal/mail/classify.go internal/mail/classify_test.go internal/mail/mock.go
git commit -m "$(cat <<'EOF'
Add folder classifier with alias table

Classify maps raw backend folders into Primary/Disposal/Custom groups
via role attribute (JMAP role / IMAP \Special-Use) or an alias table
verified against Gmail, Fastmail, Outlook, iCloud, Yahoo, Proton.

Mock backend Spam→Junk (no role) to exercise the alias path in tests.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase D — Sidebar Refactor

Rework the sidebar to consume `[]mail.ClassifiedFolder` + `config.UIConfig`.
Apply rank, label, hide, nested indent. This phase *does not* change how
the sidebar is constructed by `AccountTab` — Phase E handles that. We keep
the old `NewSidebar(styles, folders, width, height)` signature working until
Phase E by routing it through a trivial adapter (classify + default UIConfig).

### Task D1: Add new sidebar constructor and test helpers

**Files:**
- Modify: `internal/ui/sidebar.go`

- [ ] **Step 1: Introduce the new entry style**

Replace the `folderEntry` struct and `NewSidebar` constructor section
with the following. Keep the rest of the file (movement helpers, `View`,
`renderRow`, `renderBlankLine`, icon helpers) intact for now — we'll edit
specific parts in later steps.

```go
// folderEntry holds a classified folder plus its rendered metadata.
// depth is the nested-folder indent level, capped at 3.
type folderEntry struct {
	cf     mail.ClassifiedFolder
	icon   string
	depth  int
	hidden bool
}

// Sidebar renders the folder list with groups, selection, and unread badges.
type Sidebar struct {
	entries  []folderEntry // visible entries only; hidden folders pre-filtered
	selected int
	styles   Styles
	width    int
	height   int
}

// NewSidebar creates a Sidebar from a pre-classified folder list and
// a UIConfig. Ordering, hiding, labelling, and indent calculation
// happen here. Hidden folders are dropped before indexing.
func NewSidebar(styles Styles, classified []mail.ClassifiedFolder, uiCfg config.UIConfig, width, height int) Sidebar {
	entries := buildEntries(classified, uiCfg)
	return Sidebar{
		entries:  entries,
		selected: 0,
		styles:   styles,
		width:    width,
		height:   height,
	}
}
```

Add an import for the config package near the top of the file:

```go
import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
)
```

(The existing `classifyGroup` and `sidebarIcon` helpers will be removed
in the next task — leaving them in place temporarily will break the build
because they use the old `folder mail.Folder` field name. Keep compile
errors flowing into the next step.)

- [ ] **Step 2: Don't build yet — proceed to D2**

### Task D2: Replace `classifyGroup`, `sidebarIcon`, `View`, `renderRow`, and ordering logic

**Files:**
- Modify: `internal/ui/sidebar.go`

- [ ] **Step 1: Replace the bottom half of the file**

Delete the existing `sidebarIcon(f mail.Folder)` and `classifyGroup(f mail.Folder)`
functions. Add the new helpers:

```go
// buildEntries applies UIConfig to the classified folders: drops hidden
// folders, computes depth, resolves display labels, sorts each group
// by rank then display name, and concatenates Primary + Disposal +
// Custom in that order.
func buildEntries(classified []mail.ClassifiedFolder, uiCfg config.UIConfig) []folderEntry {
	var primary, disposal, custom []folderEntry
	for _, cf := range classified {
		fc := uiCfg.Folders[folderConfigKey(cf)]
		if fc.Hide {
			continue
		}
		entry := folderEntry{
			cf:    cf,
			icon:  sidebarIcon(cf),
			depth: folderDepth(cf.Folder.Name),
		}
		if fc.Label != "" {
			entry.cf.DisplayName = fc.Label
		}
		switch cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, entry)
		case mail.GroupDisposal:
			disposal = append(disposal, entry)
		default:
			custom = append(custom, entry)
		}
	}
	sortEntries(primary, uiCfg, primaryDefaultRank)
	sortEntries(disposal, uiCfg, disposalDefaultRank)
	sortEntries(custom, uiCfg, customDefaultRank)

	out := make([]folderEntry, 0, len(primary)+len(disposal)+len(custom))
	out = append(out, primary...)
	out = append(out, disposal...)
	out = append(out, custom...)
	return out
}

// folderConfigKey returns the UIConfig.Folders lookup key for a
// classified folder. Canonicals key on canonical name; custom folders
// key on provider name.
func folderConfigKey(cf mail.ClassifiedFolder) string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	return cf.Folder.Name
}

// folderDepth returns the nested-folder indent depth for a folder name.
// Counts the number of '/' characters in the name, capped at 3.
func folderDepth(name string) int {
	d := strings.Count(name, "/")
	if d > 3 {
		d = 3
	}
	return d
}

// primaryDefaultRank returns the implicit rank for a Primary-group
// canonical folder when the user has not set one explicitly.
func primaryDefaultRank(cf mail.ClassifiedFolder) int {
	switch cf.Canonical {
	case "Inbox":
		return 100
	case "Drafts":
		return 200
	case "Sent":
		return 300
	case "Archive":
		return 400
	}
	return 500 // shouldn't happen for Primary
}

// disposalDefaultRank returns the implicit rank for a Disposal canonical.
func disposalDefaultRank(cf mail.ClassifiedFolder) int {
	switch cf.Canonical {
	case "Spam":
		return 100
	case "Trash":
		return 200
	}
	return 300
}

// customDefaultRank returns the implicit rank for a Custom folder.
// Unset ranks all land here so explicit ranks can pin above.
func customDefaultRank(_ mail.ClassifiedFolder) int {
	return 1000
}

// sortEntries orders a group by (rank, display name). Rank comes from
// user config if set, otherwise from the group's default-rank function.
func sortEntries(entries []folderEntry, uiCfg config.UIConfig, defaultRank func(mail.ClassifiedFolder) int) {
	sort.SliceStable(entries, func(i, j int) bool {
		ri := rankOf(entries[i], uiCfg, defaultRank)
		rj := rankOf(entries[j], uiCfg, defaultRank)
		if ri != rj {
			return ri < rj
		}
		return entries[i].cf.DisplayName < entries[j].cf.DisplayName
	})
}

func rankOf(e folderEntry, uiCfg config.UIConfig, defaultRank func(mail.ClassifiedFolder) int) int {
	fc := uiCfg.Folders[folderConfigKey(e.cf)]
	if fc.RankSet {
		return fc.Rank
	}
	return defaultRank(e.cf)
}

// sidebarIcon returns the Nerd Font icon for a classified folder.
// Canonicals use their canonical icon; custom folders fall back to the
// heuristic name matcher.
func sidebarIcon(cf mail.ClassifiedFolder) string {
	switch cf.Canonical {
	case "Inbox":
		return "󰇰"
	case "Drafts":
		return "󰏫"
	case "Sent":
		return "󰑚"
	case "Archive":
		return "󰀼"
	case "Spam":
		return "󰍷"
	case "Trash":
		return "󰩺"
	}
	lower := strings.ToLower(cf.Folder.Name)
	switch {
	case strings.Contains(lower, "notification"):
		return "󰂚"
	case strings.Contains(lower, "remind"):
		return "󰑴"
	default:
		return "󰡡"
	}
}
```

- [ ] **Step 2: Update `SelectedFolder`, `SelectedFolderInfo`, `SelectedIcon`**

Replace the three accessors with versions that read from the new
`folderEntry` shape:

```go
// SelectedFolder returns the provider name of the currently selected folder.
// Backends look up folders by provider name, not display name.
func (s Sidebar) SelectedFolder() string {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].cf.Folder.Name
	}
	return ""
}

// SelectedFolderInfo returns the raw backend Folder at the current selection.
func (s Sidebar) SelectedFolderInfo() (mail.Folder, bool) {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].cf.Folder, true
	}
	return mail.Folder{}, false
}

// SelectedIcon returns the icon of the currently selected folder.
func (s Sidebar) SelectedIcon() string {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].icon
	}
	return ""
}
```

- [ ] **Step 3: Update `View` and `renderRow`**

Update `View()` to insert a blank separator between groups using the
`cf.Group` field instead of the old `entry.group`. Update `renderRow()`
to use `entry.cf` and to apply the `depth` indent:

```go
// View renders the sidebar as a vertical list of folder rows.
func (s Sidebar) View() string {
	if len(s.entries) == 0 || s.width == 0 || s.height == 0 {
		return ""
	}

	plainBg := s.styles.SidebarBg
	selectedBg := s.styles.SidebarSelected

	var lines []string
	prevGroup := s.entries[0].cf.Group

	for i, entry := range s.entries {
		if i > 0 && entry.cf.Group != prevGroup {
			lines = append(lines, s.renderBlankLine())
		}
		prevGroup = entry.cf.Group
		bg := plainBg
		if i == s.selected {
			bg = selectedBg
		}
		lines = append(lines, s.renderRow(i, entry, bg))
	}

	for len(lines) < s.height {
		lines = append(lines, s.renderBlankLine())
	}
	if len(lines) > s.height {
		lines = lines[:s.height]
	}
	return strings.Join(lines, "\n")
}

// renderRow renders a single folder row with proper background layering.
// Nested folders (depth > 0) get one extra space per depth level before
// the icon. The selection indicator ┃ always sits in column 0.
func (s Sidebar) renderRow(idx int, entry folderEntry, bgStyle lipgloss.Style) string {
	isSelected := idx == s.selected
	hasUnread := entry.cf.Folder.Unseen > 0

	var indicator string
	if isSelected {
		indicator = applyBg(s.styles.SidebarIndicator, bgStyle).Render("┃")
	} else {
		indicator = bgStyle.Render(" ")
	}

	textStyle := s.styles.SidebarFolder
	if hasUnread {
		textStyle = s.styles.SidebarUnread
	}

	indent := bgStyle.Render(strings.Repeat(" ", entry.depth))
	icon := applyBg(textStyle, bgStyle).Render(entry.icon)
	name := applyBg(textStyle, bgStyle).Render(entry.cf.DisplayName)

	var countStr string
	var countWidth int
	if hasUnread {
		countStr = applyBg(textStyle, bgStyle).Render(fmt.Sprintf("%d", entry.cf.Folder.Unseen))
		countWidth = lipgloss.Width(countStr)
	}

	// Layout: indicator(1) + sp(1) + indent(depth) + icon + sp×2 + name + gap + count + margin(1)
	leftContent := indicator + bgStyle.Render(" ") + indent + icon + bgStyle.Render("  ") + name
	leftWidth := lipgloss.Width(leftContent)

	rightMargin := 1
	gap := max(1, s.width-leftWidth-countWidth-rightMargin)

	row := leftContent +
		bgStyle.Render(strings.Repeat(" ", gap)) +
		countStr +
		bgStyle.Render(strings.Repeat(" ", rightMargin))

	return fillRowToWidth(row, s.width, bgStyle)
}
```

- [ ] **Step 4: Temporarily fix the two call sites that still pass `[]mail.Folder`**

Phase D leaves `account_tab.go` and `app.go` broken until Phase E. To
keep the tree compilable per our phase contract, add a shim constructor
that adapts the old call signature. Append to `sidebar.go`:

```go
// newSidebarFromFolders adapts the legacy NewSidebar(folders) signature
// for call sites not yet migrated. Used only by account_tab.go until
// Phase E refactors it.
//
// Deprecated: Phase E removes this shim.
func newSidebarFromFolders(styles Styles, folders []mail.Folder, width, height int) Sidebar {
	return NewSidebar(styles, mail.Classify(folders), config.DefaultUIConfig(), width, height)
}
```

- [ ] **Step 5: Update `account_tab.go` to call the shim**

In `internal/ui/account_tab.go`, replace:
```go
	sb := NewSidebar(styles, folders, sidebarWidth, 1)
```
with:
```go
	sb := newSidebarFromFolders(styles, folders, sidebarWidth, 1)
```

- [ ] **Step 6: Build and test**

Run: `go build ./...`
Expected: success.

Run: `go test ./internal/ui/...`
Expected: PASS (existing sidebar tests, if any, still pass through the shim).

### Task D3: Write failing sidebar tests for new behavior

**Files:**
- Create or modify: `internal/ui/sidebar_test.go`

- [ ] **Step 1: Check whether the test file exists**

Run: `ls internal/ui/sidebar_test.go`
If it does not exist, create it. If it does, append the new tests.

- [ ] **Step 2: Write the tests**

```go
package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func testSidebarStyles() Styles {
	return NewStyles(theme.Themes["one-dark"])
}

func classifyAll(in []mail.Folder) []mail.ClassifiedFolder {
	return mail.Classify(in)
}

func TestSidebarOrdering_DefaultGroups(t *testing.T) {
	input := []mail.Folder{
		{Name: "Trash", Role: "trash"},
		{Name: "Inbox", Role: "inbox"},
		{Name: "Lists/rust"},
		{Name: "Archive", Role: "archive"},
		{Name: "Lists/golang"},
		{Name: "Drafts", Role: "drafts"},
		{Name: "Sent", Role: "sent"},
		{Name: "Spam", Role: "junk"},
	}
	sb := NewSidebar(testSidebarStyles(), classifyAll(input), config.DefaultUIConfig(), 30, 20)

	got := displayNames(sb)
	want := []string{"Inbox", "Drafts", "Sent", "Archive", "Spam", "Trash", "Lists/golang", "Lists/rust"}
	assertNames(t, got, want)
}

func TestSidebarOrdering_ExplicitRank(t *testing.T) {
	input := []mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "Lists/golang"},
		{Name: "Lists/rust"},
		{Name: "Notifications"},
	}
	uiCfg := config.DefaultUIConfig()
	uiCfg.Folders["Lists/rust"] = config.FolderConfig{Rank: 1, RankSet: true}
	uiCfg.Folders["Notifications"] = config.FolderConfig{Rank: 2, RankSet: true}

	sb := NewSidebar(testSidebarStyles(), classifyAll(input), uiCfg, 30, 20)
	got := displayNames(sb)
	want := []string{"Inbox", "Lists/rust", "Notifications", "Lists/golang"}
	assertNames(t, got, want)
}

func TestSidebarHide(t *testing.T) {
	input := []mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "All Mail"},
		{Name: "Lists/golang"},
	}
	uiCfg := config.DefaultUIConfig()
	uiCfg.Folders["Archive"] = config.FolderConfig{Hide: true}

	sb := NewSidebar(testSidebarStyles(), classifyAll(input), uiCfg, 30, 20)
	got := displayNames(sb)
	want := []string{"Inbox", "Lists/golang"}
	assertNames(t, got, want)
}

func TestSidebarLabelOverride(t *testing.T) {
	input := []mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "[Gmail]/Starred"},
	}
	uiCfg := config.DefaultUIConfig()
	uiCfg.Folders["[Gmail]/Starred"] = config.FolderConfig{Label: "Starred"}

	sb := NewSidebar(testSidebarStyles(), classifyAll(input), uiCfg, 30, 20)
	got := displayNames(sb)
	want := []string{"Inbox", "Starred"}
	assertNames(t, got, want)
}

func TestSidebarNestedIndent(t *testing.T) {
	cases := []struct {
		name  string
		depth int
	}{
		{"Lists/golang", 1},
		{"Projects/Acme/Planning", 2},
		{"Projects/Acme/Planning/Q2", 3},
		{"Projects/Acme/Planning/Q2/Week1", 3},
		{"Inbox", 0},
	}
	for _, tc := range cases {
		if got := folderDepth(tc.name); got != tc.depth {
			t.Errorf("folderDepth(%q) = %d, want %d", tc.name, got, tc.depth)
		}
	}
}

func TestSidebarDisplayNormalizesCanonicals(t *testing.T) {
	input := []mail.Folder{
		{Name: "[Gmail]/Sent Mail"},
		{Name: "Deleted Items"},
	}
	sb := NewSidebar(testSidebarStyles(), classifyAll(input), config.DefaultUIConfig(), 30, 20)
	got := displayNames(sb)
	want := []string{"Sent", "Trash"}
	assertNames(t, got, want)
}

// displayNames extracts DisplayName from each sidebar entry in order.
func displayNames(sb Sidebar) []string {
	out := make([]string, 0, len(sb.entries))
	for _, e := range sb.entries {
		out = append(out, e.cf.DisplayName)
	}
	return out
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("order mismatch\n got: %v\nwant: %v", got, want)
	}
}
```

- [ ] **Step 3: Run the tests**

Run: `go test ./internal/ui/... -run TestSidebar -v`
Expected: PASS — all functionality already exists from D2.

### Task D4: Commit Phase D

- [ ] **Step 1: Stage and commit**

```bash
git add internal/ui/sidebar.go internal/ui/sidebar_test.go internal/ui/account_tab.go
git commit -m "$(cat <<'EOF'
Sidebar: consume ClassifiedFolder + UIConfig

Rank-aware in-group ordering, label override, hide, one-space indent
per depth level capped at 3, display name normalization for canonicals.

Adds a newSidebarFromFolders shim that will be removed in the Elm
refactor pass so the account_tab call site stays compilable.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase E — Elm Refactor (Cmd-Based I/O)

Convert synchronous backend I/O to `tea.Cmd` flow. Remove parent-grandchild
peeks. Drop the dead `:` stub.

### Task E1: Add message types and Cmd constructors

**Files:**
- Create: `internal/ui/cmds.go`

- [ ] **Step 1: Write the file**

```go
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// foldersLoadedMsg carries the result of an initial ListFolders call.
type foldersLoadedMsg struct {
	folders []mail.Folder
}

// folderLoadedMsg carries the result of opening a folder and fetching
// its header list.
type folderLoadedMsg struct {
	name string
	msgs []mail.MessageInfo
}

// backendErrMsg wraps a backend error. Pass 2.5b-6 (status/toast) will
// surface this to the user; for now Update logs and moves on.
type backendErrMsg struct {
	err error
}

// FolderChangedMsg is emitted by AccountTab whenever the selected folder
// changes (after initial load, J/K navigation, or folder-jump keys).
// App consumes it to update the status bar without reaching into child
// state.
type FolderChangedMsg struct {
	Name   string
	Exists int
	Unseen int
}

// loadFoldersCmd returns a Cmd that fetches the folder list from the
// backend. The result is delivered as a foldersLoadedMsg, or a
// backendErrMsg on failure.
func loadFoldersCmd(b mail.Backend) tea.Cmd {
	return func() tea.Msg {
		folders, err := b.ListFolders()
		if err != nil {
			return backendErrMsg{err: err}
		}
		return foldersLoadedMsg{folders: folders}
	}
}

// loadFolderCmd returns a Cmd that opens a folder and fetches its
// header list. The result is a folderLoadedMsg, or a backendErrMsg.
func loadFolderCmd(b mail.Backend, name string) tea.Cmd {
	if name == "" {
		return func() tea.Msg { return folderLoadedMsg{name: "", msgs: nil} }
	}
	return func() tea.Msg {
		if err := b.OpenFolder(name); err != nil {
			return backendErrMsg{err: err}
		}
		msgs, err := b.FetchHeaders(nil)
		if err != nil {
			return backendErrMsg{err: err}
		}
		return folderLoadedMsg{name: name, msgs: msgs}
	}
}

// folderChangedCmd returns a zero-latency Cmd that emits a
// FolderChangedMsg. Using a Cmd (rather than a direct mutation) keeps
// message flow inside bubbletea's Update loop.
func folderChangedCmd(f mail.Folder) tea.Cmd {
	return func() tea.Msg {
		return FolderChangedMsg{
			Name:   f.Name,
			Exists: f.Exists,
			Unseen: f.Unseen,
		}
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/ui/...`
Expected: success.

### Task E2: Add `SetFolders` / `SetSelectedByName` to `Sidebar`

The sidebar needs a mutator so `AccountTab.Update` can seed it from a
`foldersLoadedMsg`. The existing `NewSidebar` builds a fresh Sidebar;
we want to keep the selection across reloads (Pass 3 will trigger reloads).

**Files:**
- Modify: `internal/ui/sidebar.go`

- [ ] **Step 1: Add the mutator**

Append to `sidebar.go`:

```go
// SetFolders replaces the sidebar's folder set with a newly classified
// list under a given UIConfig. Selection is preserved by provider name
// where possible; otherwise it clamps to 0.
func (s *Sidebar) SetFolders(classified []mail.ClassifiedFolder, uiCfg config.UIConfig) {
	var prevName string
	if s.selected < len(s.entries) {
		prevName = s.entries[s.selected].cf.Folder.Name
	}
	s.entries = buildEntries(classified, uiCfg)
	s.selected = 0
	if prevName != "" {
		for i, e := range s.entries {
			if e.cf.Folder.Name == prevName {
				s.selected = i
				break
			}
		}
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/ui/...`
Expected: success.

### Task E3: Write failing tests for `AccountTab` Cmd dispatch

**Files:**
- Create: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write the test file**

```go
package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func newTestTab(t *testing.T) AccountTab {
	t.Helper()
	backend := mail.NewMockBackend()
	styles := NewStyles(theme.Themes["one-dark"])
	tab := NewAccountTab(styles, backend, config.DefaultUIConfig())
	// Size it so View() is exercisable if we need it.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return tab
}

// runCmd executes a tea.Cmd synchronously and returns its message.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestAccountTabInit_ReturnsFoldersCmd(t *testing.T) {
	tab := newTestTab(t)
	msg := runCmd(tab.Init())
	if _, ok := msg.(foldersLoadedMsg); !ok {
		t.Fatalf("expected foldersLoadedMsg from Init, got %T", msg)
	}
}

func TestAccountTab_foldersLoadedSeedsSidebar(t *testing.T) {
	tab := newTestTab(t)
	folders, _ := mail.NewMockBackend().ListFolders()
	tab, cmd := tab.updateTab(foldersLoadedMsg{folders: folders})
	if len(tab.sidebar.entries) == 0 {
		t.Fatalf("expected sidebar to be seeded")
	}
	if cmd == nil {
		t.Fatalf("expected a follow-up cmd to load the initial folder")
	}
	msg := runCmd(cmd)
	// Either a folderLoadedMsg (from loadFolderCmd) or a batch. Accept
	// the BatchMsg too for robustness but prefer the direct case.
	switch msg.(type) {
	case folderLoadedMsg:
	case tea.BatchMsg:
	default:
		t.Fatalf("expected folderLoadedMsg or BatchMsg, got %T", msg)
	}
}

func TestAccountTab_folderLoadedSeedsMsglist(t *testing.T) {
	tab := newTestTab(t)
	msgs := []mail.MessageInfo{
		{UID: "1", Subject: "hello", From: "a", Date: "now"},
	}
	tab, _ = tab.updateTab(folderLoadedMsg{name: "Inbox", msgs: msgs})
	if tab.msglist.Count() != 1 {
		t.Fatalf("expected msglist count 1, got %d", tab.msglist.Count())
	}
}

func TestAccountTab_JDispatchesFolderLoad(t *testing.T) {
	tab := newTestTab(t)
	folders, _ := mail.NewMockBackend().ListFolders()
	tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})

	tab, cmd := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if cmd == nil {
		t.Fatal("expected J to dispatch a Cmd")
	}
	// The cmd should be a batch (folderChanged + loadFolder) OR a bare
	// loadFolderCmd. Running it should yield one of the two messages.
	msg := runCmd(cmd)
	switch m := msg.(type) {
	case folderLoadedMsg:
	case FolderChangedMsg:
	case tea.BatchMsg:
		if len(m) == 0 {
			t.Fatal("empty batch")
		}
	default:
		t.Fatalf("unexpected cmd result: %T", msg)
	}
}
```

- [ ] **Step 2: Run — expect compile failures**

Run: `go test ./internal/ui/... -run TestAccountTab`
Expected: compile errors — `NewAccountTab` signature mismatch, `updateTab`
signature doesn't return `(AccountTab, tea.Cmd)` on KeyMsg, etc.

### Task E4: Rewrite `AccountTab` for Cmd-based flow

**Files:**
- Modify: `internal/ui/account_tab.go`

- [ ] **Step 1: Replace the struct, constructor, Init, Update, and handleKey**

```go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// sidebarWidth is the fixed width of the sidebar panel.
const sidebarWidth = 30

// AccountTab is the main account view. One pane (like pine): every
// key is always live. J/K/G navigate folders, j/k navigate messages.
type AccountTab struct {
	styles  Styles
	backend mail.Backend
	uiCfg   config.UIConfig
	sidebar Sidebar
	msglist MessageList
	width   int
	height  int
}

// NewAccountTab builds an empty AccountTab. The initial folder list is
// fetched via Init's returned Cmd, not synchronously.
func NewAccountTab(styles Styles, backend mail.Backend, uiCfg config.UIConfig) AccountTab {
	return AccountTab{
		styles:  styles,
		backend: backend,
		uiCfg:   uiCfg,
		sidebar: NewSidebar(styles, nil, uiCfg, sidebarWidth, 1),
		msglist: NewMessageList(styles, nil, 1, 1),
	}
}

// Title returns the current folder name.
func (m AccountTab) Title() string { return m.sidebar.SelectedFolder() }

// Icon returns the folder's Nerd Font icon.
func (m AccountTab) Icon() string { return m.sidebar.SelectedIcon() }

// Closeable returns false — the account tab cannot be closed.
func (m AccountTab) Closeable() bool { return false }

// Init fires the initial folder-list fetch.
func (m AccountTab) Init() tea.Cmd {
	return loadFoldersCmd(m.backend)
}

// Update satisfies tea.Model. Delegates to updateTab for typed access.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
	return m.updateTab(msg)
}

// updateTab handles the message cases and returns a typed AccountTab.
func (m AccountTab) updateTab(msg tea.Msg) (AccountTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sw := min(sidebarWidth, m.width/2)
		m.sidebar.SetSize(sw, m.height-2) // -2 for account name + blank line
		mw := max(1, m.width-sw-1)        // -1 for divider
		m.msglist.SetSize(mw, m.height)
		return m, nil

	case foldersLoadedMsg:
		m.sidebar.SetFolders(mail.Classify(msg.folders), m.uiCfg)
		return m, m.selectionChangedCmds()

	case folderLoadedMsg:
		m.msglist.SetMessages(msg.msgs)
		return m, nil

	case backendErrMsg:
		// TODO(pass-2.5b-6): surface via status/toast.
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches navigation keys by identity. J/K/G move the
// sidebar (and dispatch a folder-load Cmd); j/k/Ctrl-d/Ctrl-u move the
// message list cursor.
func (m AccountTab) handleKey(msg tea.KeyMsg) (AccountTab, tea.Cmd) {
	switch msg.String() {
	case "J":
		m.sidebar.MoveDown()
		return m, m.selectionChangedCmds()
	case "K":
		m.sidebar.MoveUp()
		return m, m.selectionChangedCmds()
	case "G":
		m.msglist.MoveToBottom()
	case "g":
		m.msglist.MoveToTop()
	case "j", "down":
		m.msglist.MoveDown()
	case "k", "up":
		m.msglist.MoveUp()
	case "ctrl+d":
		m.msglist.HalfPageDown()
	case "ctrl+u":
		m.msglist.HalfPageUp()
	case "ctrl+f", "pgdown":
		m.msglist.PageDown()
	case "ctrl+b", "pgup":
		m.msglist.PageUp()
	}
	return m, nil
}

// selectionChangedCmds returns the batch of Cmds that run every time
// the selected folder changes: a FolderChangedMsg emission so App's
// status bar updates, plus a load Cmd that will populate the message
// list when it resolves.
func (m AccountTab) selectionChangedCmds() tea.Cmd {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return nil
	}
	return tea.Batch(
		folderChangedCmd(folder),
		loadFolderCmd(m.backend, folder.Name),
	)
}

// View renders the sidebar + divider + message list.
func (m AccountTab) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sw := min(sidebarWidth, m.width/2)

	acctLine := m.styles.SidebarAccount.Width(sw).Render(" " + m.backend.AccountName())
	blank := m.styles.SidebarBg.Width(sw).Render("")

	sidebarFolders := m.sidebar.View()

	var sidebarLines []string
	sidebarLines = append(sidebarLines, acctLine, blank)
	if sidebarFolders != "" {
		sidebarLines = append(sidebarLines, strings.Split(sidebarFolders, "\n")...)
	}
	for len(sidebarLines) < m.height {
		sidebarLines = append(sidebarLines, blank)
	}
	if len(sidebarLines) > m.height {
		sidebarLines = sidebarLines[:m.height]
	}

	sidebarView := strings.Join(sidebarLines, "\n")
	divider := renderDivider(m.height, m.styles)
	msglistView := m.msglist.View()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, divider, msglistView)
}

// renderDivider renders a vertical line of │ characters.
func renderDivider(height int, s Styles) string {
	div := s.PanelDivider.Render("│")
	lines := make([]string, height)
	for i := range lines {
		lines[i] = div
	}
	return strings.Join(lines, "\n")
}
```

The old `loadSelectedFolder` method is gone. The `backend` field is
still read, only through Cmds.

- [ ] **Step 2: Remove the shim `newSidebarFromFolders`**

In `internal/ui/sidebar.go`, delete the `newSidebarFromFolders` helper
added in Task D2 step 4 — nothing calls it anymore.

- [ ] **Step 3: Build**

Run: `go build ./internal/ui/...`
Expected: build error in `app.go` — `NewAccountTab` signature changed.
Leave it; Task E5 fixes it.

### Task E5: Rewrite `App` to consume `FolderChangedMsg` and drop `:` stub

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Replace the file**

```go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// App is the root bubbletea model for poplar.
type App struct {
	acct      AccountTab
	styles    Styles
	topLine   TopLine
	statusBar StatusBar
	footer    Footer
	keys      GlobalKeys
	width     int
	height    int
}

// NewApp creates the root model with a single AccountTab. Folder loading
// happens in Init's Cmd chain, not in the constructor.
func NewApp(t *theme.CompiledTheme, backend mail.Backend, uiCfg config.UIConfig) App {
	styles := NewStyles(t)
	sb := NewStatusBar(styles)
	sb.SetConnectionState(Connected)

	return App{
		acct:      NewAccountTab(styles, backend, uiCfg),
		styles:    styles,
		topLine:   NewTopLine(styles),
		statusBar: sb,
		footer:    NewFooter(styles),
		keys:      NewGlobalKeys(),
	}
}

// Init delegates to the account tab so the initial folder fetch fires.
func (m App) Init() tea.Cmd {
	return m.acct.Init()
}

// Update handles global keys and delegates everything else to the
// account tab. FolderChangedMsg bubbles up from the child and updates
// the status bar without reaching into child state.
func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(contentMsg)
		return m, cmd

	case FolderChangedMsg:
		m.statusBar.SetCounts(msg.Exists, msg.Unseen)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			// Stubbed for 2.5b-5 (help popover)
			return m, nil
		}
	}

	// Delegate everything else to the account tab.
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	return m, cmd
}

// View composes the full-screen layout.
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	rawContent := m.acct.View()
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	for i, line := range contentLines {
		pad := max(0, m.width-1-lipgloss.Width(line))
		contentLines[i] = line + strings.Repeat(" ", pad) + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := sidebarWidth
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, sidebarWidth)
	foot := m.footer.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		content,
		status,
		foot,
	)
}

// contentHeight returns the height available for the content area.
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}
```

Changes from the old `app.go`:

- `NewApp` signature gains `uiCfg config.UIConfig`.
- Drops the synchronous `ListFolders` call and `folders[0]` seeding.
- `Init` delegates to `acct.Init()` instead of returning `nil`.
- `Update` handles `FolderChangedMsg` to update the status bar.
- Drops `syncStatusBar` (grandchild peek) entirely.
- Drops the `case ":":` stub for command mode.

- [ ] **Step 2: Update `cmd/poplar/root.go` to pass `uiCfg`**

In `runRoot`, after the theme lookup, add config loading and pass it
through:

```go
func runRoot(f rootFlags) error {
	t, ok := theme.Themes[strings.ToLower(f.theme)]
	if !ok {
		return fmt.Errorf("unknown theme %q (available: %s)",
			f.theme, strings.Join(theme.ThemeNames(), ", "))
	}

	backend := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig()
	// When poplar gains a --config flag (Pass 3), LoadUI(configPath) replaces this line.
	app := ui.NewApp(t, backend, uiCfg)

	p := tea.NewProgram(appModel{app: app}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running poplar: %w", err)
	}
	return nil
}
```

Add the import:
```go
	"github.com/glw907/beautiful-aerc/internal/config"
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Run the account_tab tests**

Run: `go test ./internal/ui/... -run TestAccountTab -v`
Expected: PASS.

- [ ] **Step 5: Run the full test suite**

Run: `make check`
Expected: PASS.

### Task E6: Smoke-test the live binary

**Files:** N/A

- [ ] **Step 1: Build and install**

Run: `make install`
Expected: success.

- [ ] **Step 2: Launch poplar and verify no regression**

Run: `poplar` (user launches manually).

Expected behavior:
- Sidebar loads with the mock folders in their new order: Inbox, Drafts, Sent, Archive, blank, Spam (from `Junk` alias), Trash, blank, Lists/golang, Lists/rust, Notifications, Remind.
- Nested folders `Lists/golang` and `Lists/rust` show a one-space indent before the icon.
- `Junk` displays as `Spam` (the canonical name).
- J/K navigates folders; message list loads on folder change.
- j/k navigates messages.
- The status bar updates folder counts when you press J/K.
- `q` quits.

If the live UI matches, continue. If not, fix the regression before committing.

### Task E7: Commit Phase E

- [ ] **Step 1: Stage and commit**

```bash
git add internal/ui/cmds.go internal/ui/account_tab.go internal/ui/app.go internal/ui/sidebar.go internal/ui/account_tab_test.go cmd/poplar/root.go
git commit -m "$(cat <<'EOF'
Convert poplar UI to tea.Cmd-based backend I/O

NewAccountTab no longer calls ListFolders synchronously; Init fires
loadFoldersCmd. J/K dispatch loadFolderCmd instead of blocking the
Update loop. App consumes FolderChangedMsg from AccountTab to update
the status bar without reaching through m.acct.sidebar. Drops the
dead : command-mode stub.

Establishes the Cmd flow Pass 3 needs before real JMAP latency lands.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase F — `poplar config init` Subcommand

Discovers folders from the configured backend and writes commented
subsection defaults into `accounts.toml`, merge-only, idempotent.

### Task F1: Write the failing writer tests

**Files:**
- Create: `internal/config/writer_test.go`

- [ ] **Step 1: Write the tests**

```go
package config

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/mail"
)

func TestRenderFolderSubsections_Empty(t *testing.T) {
	got := RenderFolderSubsections(nil, nil)
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestRenderFolderSubsections_CanonicalsAndCustom(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "Drafts", Role: "drafts"},
		{Name: "Sent", Role: "sent"},
		{Name: "Archive", Role: "archive"},
		{Name: "Junk"},
		{Name: "Trash", Role: "trash"},
		{Name: "Lists/golang"},
		{Name: "Lists/rust"},
	})
	got := RenderFolderSubsections(classified, nil)

	// Expect Primary group first, then Disposal, then Custom, with a
	// blank line between groups.
	expectOrder := []string{
		`[ui.folders.Inbox]`,
		`[ui.folders.Drafts]`,
		`[ui.folders.Sent]`,
		`[ui.folders.Archive]`,
		``,
		`[ui.folders.Spam]`,
		`[ui.folders.Trash]`,
		``,
		`[ui.folders."Lists/golang"]`,
		`[ui.folders."Lists/rust"]`,
	}
	lines := strings.Split(got, "\n")
	// Extract the header/blank lines only (ignore commented fields).
	var headers []string
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "[ui.folders") {
			headers = append(headers, line)
		}
	}
	if !sliceContainsSubseq(headers, expectOrder) {
		t.Fatalf("expected header order %v in output, got %v\nfull output:\n%s", expectOrder, headers, got)
	}

	// Every subsection must include all five commented field hints.
	wantFields := []string{"# label =", "# rank =", "# threading =", "# sort =", "# hide ="}
	for _, f := range wantFields {
		if !strings.Contains(got, f) {
			t.Errorf("expected %q in output", f)
		}
	}
}

func TestRenderFolderSubsections_SkipsExisting(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "Drafts", Role: "drafts"},
	})
	existing := map[string]bool{"Inbox": true}
	got := RenderFolderSubsections(classified, existing)

	if strings.Contains(got, "[ui.folders.Inbox]") {
		t.Errorf("Inbox subsection should have been skipped, got %q", got)
	}
	if !strings.Contains(got, "[ui.folders.Drafts]") {
		t.Errorf("Drafts subsection should be present")
	}
}

func TestRenderFolderSubsections_QuotesCustomNames(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Lists/golang"},
	})
	got := RenderFolderSubsections(classified, nil)
	if !strings.Contains(got, `[ui.folders."Lists/golang"]`) {
		t.Errorf("custom folder name should be quoted, got %q", got)
	}
}

// sliceContainsSubseq returns true if target appears as an ordered
// (not necessarily contiguous) subsequence of src.
func sliceContainsSubseq(src, target []string) bool {
	i := 0
	for _, s := range src {
		if i < len(target) && s == target[i] {
			i++
		}
	}
	return i == len(target)
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/config/... -run RenderFolder`
Expected: compile error (`RenderFolderSubsections` undefined).

### Task F2: Implement the writer

**Files:**
- Create: `internal/config/writer.go`

- [ ] **Step 1: Write the implementation**

```go
package config

import (
	"strings"

	"github.com/glw907/beautiful-aerc/internal/mail"
)

// RenderFolderSubsections renders `[ui.folders.<name>]` subsections
// with commented default hints for each classified folder not already
// present in the existing set. Output is grouped: Primary, then
// Disposal, then Custom, separated by blank lines. Returns "" when
// there is nothing to write.
//
// existing may be nil. Keys are:
//   - canonical name (Inbox, Drafts, ...) for classified canonicals
//   - provider name for classified custom folders
//
// — matching the same lookup keys Sidebar and UIConfig.Folders use.
func RenderFolderSubsections(classified []mail.ClassifiedFolder, existing map[string]bool) string {
	primary, disposal, custom := splitByGroup(classified, existing)

	var parts []string
	if block := renderGroup(primary); block != "" {
		parts = append(parts, block)
	}
	if block := renderGroup(disposal); block != "" {
		parts = append(parts, block)
	}
	if block := renderGroup(custom); block != "" {
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n")
}

func splitByGroup(classified []mail.ClassifiedFolder, existing map[string]bool) (primary, disposal, custom []mail.ClassifiedFolder) {
	for _, cf := range classified {
		key := subsectionKey(cf)
		if existing[key] {
			continue
		}
		switch cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, cf)
		case mail.GroupDisposal:
			disposal = append(disposal, cf)
		default:
			custom = append(custom, cf)
		}
	}
	return
}

func renderGroup(folders []mail.ClassifiedFolder) string {
	if len(folders) == 0 {
		return ""
	}
	var b strings.Builder
	for _, cf := range folders {
		b.WriteString(renderSubsection(cf))
	}
	b.WriteString("\n")
	return b.String()
}

func renderSubsection(cf mail.ClassifiedFolder) string {
	var b strings.Builder
	b.WriteString("[ui.folders.")
	b.WriteString(subsectionHeaderKey(cf))
	b.WriteString("]\n")
	b.WriteString("# label = \"\"\n")
	b.WriteString("# rank = 0\n")
	b.WriteString("# threading = true\n")
	b.WriteString("# sort = \"date-desc\"\n")
	b.WriteString("# hide = false\n")
	return b.String()
}

// subsectionKey returns the lookup key for UIConfig.Folders and for
// detecting existing subsections — canonical name for canonicals,
// provider name for custom.
func subsectionKey(cf mail.ClassifiedFolder) string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	return cf.Folder.Name
}

// subsectionHeaderKey returns the TOML header key — bare identifier
// when possible, otherwise a quoted string. Canonical names are always
// bare (they're alphanumeric); custom folder names get quoted when
// they contain anything other than ASCII letters, digits, hyphen, or
// underscore.
func subsectionHeaderKey(cf mail.ClassifiedFolder) string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	if isBareKey(cf.Folder.Name) {
		return cf.Folder.Name
	}
	return `"` + cf.Folder.Name + `"`
}

func isBareKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			continue
		default:
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/config/... -run RenderFolder -v`
Expected: PASS.

### Task F3: Add merge and idempotence support to the writer

The `poplar config init` subcommand needs two operations: parsing which
subsection keys already exist in `accounts.toml`, and appending the
rendered output to the existing file.

**Files:**
- Modify: `internal/config/writer.go`

- [ ] **Step 1: Add `ExistingFolderKeys` and `MergeFolderSubsections`**

Append to `writer.go`:

```go
// ExistingFolderKeys parses an accounts.toml file and returns the set
// of subsection keys already present under [ui.folders.<name>]. The
// keys use the same convention as subsectionKey: canonical or provider
// name, unquoted.
func ExistingFolderKeys(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var raw rawUIFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	out := make(map[string]bool, len(raw.UI.Folders))
	for k := range raw.UI.Folders {
		out[k] = true
	}
	return out, nil
}

// MergeFolderSubsections appends new rendered subsections to the end
// of the config file at path and returns the merged file contents.
// Existing content is preserved byte-for-byte. If newContent is empty,
// the original contents are returned unchanged. The caller decides
// whether to write the result back (dry-run vs --write).
func MergeFolderSubsections(path, newContent string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading config: %w", err)
	}
	if newContent == "" {
		return string(data), nil
	}
	existing := string(data)
	// Ensure exactly one blank line between old and new content.
	existing = strings.TrimRight(existing, "\n")
	return existing + "\n\n" + newContent, nil
}
```

Add the imports:
```go
import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/glw907/beautiful-aerc/internal/mail"
)
```

- [ ] **Step 2: Add a merge test**

Append to `internal/config/writer_test.go`:

```go
func TestMergeFolderSubsections_EmptyNewContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	orig := "[ui]\nthreading = true\n"
	if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := MergeFolderSubsections(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != orig {
		t.Errorf("expected unchanged file, got %q", got)
	}
}

func TestMergeFolderSubsections_Appends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	orig := "[ui]\nthreading = true\n"
	if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := MergeFolderSubsections(path, "[ui.folders.Inbox]\n# rank = 0\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "[ui]\nthreading = true\n\n[ui.folders.Inbox]\n# rank = 0\n"
	if got != want {
		t.Errorf("merge mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestExistingFolderKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	contents := `[ui]
threading = true

[ui.folders.Inbox]
rank = 1

[ui.folders."Lists/golang"]
rank = 5
`
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	keys, err := ExistingFolderKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Inbox": true, "Lists/golang": true}
	if len(keys) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(keys), len(want), keys)
	}
	for k := range want {
		if !keys[k] {
			t.Errorf("missing key %q", k)
		}
	}
}
```

Add the missing imports to the test file:
```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/mail"
)
```

- [ ] **Step 3: Run all writer tests**

Run: `go test ./internal/config/... -v`
Expected: all PASS.

### Task F4: Write the failing `config init` command tests

**Files:**
- Create: `cmd/poplar/config_init_test.go`

- [ ] **Step 1: Write the tests**

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runConfigInit invokes the subcommand with the given flags and
// returns its captured stdout.
func runConfigInit(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newConfigInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	return buf.String()
}

func writeStubConfig(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "accounts.toml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// minimalMockConfig creates a config file with one mock-backend account.
// The mock backend scheme is "mock://" — Phase F wires it into
// openBackendForInit, bypassing real JMAP/IMAP for tests.
const minimalMockConfig = `[[account]]
name = "Mock"
backend = "mock"
source = "mock://local"

[ui]
threading = true
`

func TestConfigInit_DryRunShowsDiscoveredFolders(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	out := runConfigInit(t, "--config", path)

	// Expect every mock folder subsection to appear in the output.
	wantKeys := []string{
		"[ui.folders.Inbox]",
		"[ui.folders.Drafts]",
		"[ui.folders.Sent]",
		"[ui.folders.Archive]",
		"[ui.folders.Spam]",
		"[ui.folders.Trash]",
		`[ui.folders.Notifications]`,
		`[ui.folders.Remind]`,
		`[ui.folders."Lists/golang"]`,
		`[ui.folders."Lists/rust"]`,
	}
	for _, k := range wantKeys {
		if !strings.Contains(out, k) {
			t.Errorf("expected %q in dry-run output", k)
		}
	}

	// File on disk must be unchanged.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != minimalMockConfig {
		t.Errorf("dry-run should not modify file; got:\n%s", got)
	}
}

func TestConfigInit_WriteAppends(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = runConfigInit(t, "--config", path, "--write")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "[ui.folders.Inbox]") {
		t.Errorf("expected Inbox subsection after --write\ngot:\n%s", got)
	}
	// Original content preserved.
	if !strings.Contains(string(got), `name = "Mock"`) {
		t.Errorf("original config lost\ngot:\n%s", got)
	}
}

func TestConfigInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = runConfigInit(t, "--config", path, "--write")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = runConfigInit(t, "--config", path, "--write")
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second run should be no-op\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./cmd/poplar/... -run ConfigInit`
Expected: compile error (`newConfigInitCmd`, `openBackendForInit` undefined).

### Task F5: Implement `poplar config` parent command

**Files:**
- Create: `cmd/poplar/config.go`

- [ ] **Step 1: Write the parent command**

```go
package main

import (
	"github.com/spf13/cobra"
)

// newConfigCmd creates the parent `poplar config` command. Subcommands
// hang off this one.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "config",
		Short:        "Manage poplar configuration",
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigInitCmd())
	return cmd
}
```

### Task F6: Implement `poplar config init`

**Files:**
- Create: `cmd/poplar/config_init.go`
- Modify: `cmd/poplar/main.go` (wire the parent cmd into root)

- [ ] **Step 1: Write the init subcommand**

```go
package main

import (
	"fmt"

	"github.com/glw907/beautiful-aerc/internal/config"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/spf13/cobra"
)

type configInitFlags struct {
	config string
	write  bool
}

func newConfigInitCmd() *cobra.Command {
	f := configInitFlags{}
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Discover folders and merge [ui.folders] defaults into accounts.toml",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.config, "config", "", "path to accounts.toml (default: $XDG_CONFIG_HOME/poplar/accounts.toml)")
	cmd.Flags().BoolVar(&f.write, "write", false, "write merged output to the config file (default: dry-run to stdout)")
	return cmd
}

func runConfigInit(cmd *cobra.Command, f configInitFlags) error {
	path := f.config
	if path == "" {
		p, err := defaultConfigPath()
		if err != nil {
			return err
		}
		path = p
	}

	accounts, err := config.ParseAccounts(path)
	if err != nil {
		return fmt.Errorf("loading accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts in %s", path)
	}

	// v1 is single-account. Connect to the first account's backend.
	backend, err := openBackendForInit(accounts[0])
	if err != nil {
		return fmt.Errorf("opening backend for account %q: %w", accounts[0].Name, err)
	}
	defer backend.Disconnect()

	folders, err := backend.ListFolders()
	if err != nil {
		return fmt.Errorf("listing folders: %w", err)
	}
	classified := mail.Classify(folders)

	existing, err := config.ExistingFolderKeys(path)
	if err != nil {
		return fmt.Errorf("reading existing folder keys: %w", err)
	}

	rendered := config.RenderFolderSubsections(classified, existing)
	merged, err := config.MergeFolderSubsections(path, rendered)
	if err != nil {
		return fmt.Errorf("merging: %w", err)
	}

	if !f.write {
		fmt.Fprint(cmd.OutOrStdout(), merged)
		return nil
	}
	return writeAtomically(path, merged)
}

// openBackendForInit returns a connected backend for the given account.
// Currently only the "mock" backend type is wired for init — real JMAP
// wiring will follow when Pass 3 lands the adapter. The mock backend
// URL scheme is "mock://" and requires no credentials.
func openBackendForInit(acct config.AccountConfig) (mail.Backend, error) {
	switch acct.Backend {
	case "mock":
		return mail.NewMockBackend(), nil
	default:
		return nil, fmt.Errorf("backend %q not yet supported by config init (Pass 3)", acct.Backend)
	}
}
```

- [ ] **Step 2: Add `defaultConfigPath` and `writeAtomically`**

Append to `cmd/poplar/config_init.go`:

```go
import (
	// ... existing imports ...
	"os"
	"path/filepath"
)

func defaultConfigPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "poplar", "accounts.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home dir: %w", err)
	}
	return filepath.Join(home, ".config", "poplar", "accounts.toml"), nil
}

// writeAtomically writes content to path via a temp file + rename.
// Preserves the original on crash.
func writeAtomically(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".accounts.toml.tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op on success after Rename

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Wire the parent command into root**

In `cmd/poplar/main.go`, after the root command is constructed, add the
config subcommand. Find the spot where `newRootCmd()` is called and adapt:

```go
func main() {
	root := newRootCmd()
	root.AddCommand(newConfigCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

If `main.go` has a different shape, read it and integrate. The intent is
`newConfigCmd()` becomes a subcommand of the root.

- [ ] **Step 4: Extend the mock backend `Backend` field handling**

`accounts_test.go` already passes `backend = "mock"` through `ParseAccounts`
but the field was never populated. Confirm `config/accounts.go` sets
`Backend: e.Backend` in `toAccountConfig`. If not, add the assignment.

Run: `grep -n 'Backend:' internal/config/accounts.go`

If `Backend: e.Backend` is missing, add it:

```go
	acct := &AccountConfig{
		Name:            e.Name,
		Backend:         e.Backend,
		...
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./cmd/poplar/... -v -run ConfigInit`
Expected: all PASS.

- [ ] **Step 6: Build and smoke-test**

Run: `make build`
Expected: success.

Run: `./bin/poplar config init --help` (or wherever `make build` puts the
binary — if it installs to `~/.local/bin/poplar`, use that path)
Expected: help text showing `--config`, `--write`.

### Task F7: Commit Phase F

- [ ] **Step 1: Stage and commit**

```bash
git add internal/config/writer.go internal/config/writer_test.go cmd/poplar/config.go cmd/poplar/config_init.go cmd/poplar/config_init_test.go cmd/poplar/main.go internal/config/accounts.go
git commit -m "$(cat <<'EOF'
Add poplar config init subcommand

Discovers folders from the configured backend, classifies them, and
merges [ui.folders.<name>] subsections with commented default hints
into accounts.toml. Dry-run by default; --write replaces the file
atomically. Idempotent: existing subsections are skipped.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Phase G — Shipped Example + Docs

Ship the example `accounts.toml` with canonical subsections pre-filled,
and finish the doc debt (architecture, keybindings, wireframes, STATUS).

### Task G1: Ship the example `accounts.toml`

**Files:**
- Create: `accounts.toml.example` (at repo root)

- [ ] **Step 1: Write the example**

```toml
# poplar account configuration
# Copy to $XDG_CONFIG_HOME/poplar/accounts.toml (usually
# ~/.config/poplar/accounts.toml) and edit to taste.

[[account]]
name = "Example"
backend = "jmap"
source = "jmap+oauthbearer://you@example.com@api.example.com/.well-known/jmap"
credential-cmd = "fastmail-password"

# UI preferences. The [ui] table is optional — omit it to take the
# defaults. Run `poplar config init` after first launch to discover
# your provider's folder names and generate [ui.folders.<name>]
# subsections for your custom folders.
[ui]
threading = true

[ui.folders.Inbox]
# label = ""
# rank = 0
# threading = true
# sort = "date-desc"
# hide = false

[ui.folders.Drafts]
# rank = 0

[ui.folders.Sent]
# rank = 0

[ui.folders.Archive]
# rank = 0

[ui.folders.Spam]
# rank = 0

[ui.folders.Trash]
# rank = 0
```

- [ ] **Step 2: Verify the example parses cleanly**

Add a test at `internal/config/accounts_test.go` that loads the example
file (use a relative path from the test, resolved via `filepath.Abs`):

Actually — keep things simple. Write the example content inline in a new
test to guarantee parser acceptance:

```go
func TestExampleConfigParses(t *testing.T) {
	const example = `[[account]]
name = "Example"
backend = "jmap"
source = "jmap+oauthbearer://you@example.com@api.example.com/.well-known/jmap"
credential-cmd = "echo token"

[ui]
threading = true

[ui.folders.Inbox]
# rank = 0

[ui.folders.Drafts]
[ui.folders.Sent]
[ui.folders.Archive]
[ui.folders.Spam]
[ui.folders.Trash]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	if err := os.WriteFile(path, []byte(example), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccounts(path); err != nil {
		t.Fatalf("ParseAccounts: %v", err)
	}
	if _, err := LoadUI(path); err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/config/... -run TestExample`
Expected: PASS.

### Task G2: Update `docs/poplar/architecture.md`

**Files:**
- Modify: `docs/poplar/architecture.md`

- [ ] **Step 1: Add three new decision entries at the bottom of the decisions section**

Append these after the most recent decision entry (the "Runtime threading
toggle: dropped" one):

````markdown
### UIConfig and AccountConfig unified in `internal/config/`
**Decision:** The `internal/poplar/` package is deleted in Pass
2.5b-3.5. `AccountConfig` moves to `internal/config/account.go`
and a new `UIConfig` + `LoadUI` lives in `internal/config/ui.go`.
Both types read from the same `accounts.toml` file via
independent decodings (`BurntSushi/toml` silently drops unknown
keys, so the two parsers don't collide).
**Rationale:** `internal/poplar/` held exactly one type (and its
loader) and was colliding with the `poplar` package-alias name
inside `internal/mail/jmap.go`. Consolidating both config
concerns under `internal/config/` gives a single clear home for
"things read from the user's config file" and removes the
alias-shadow footgun. Follow-on config sections (keybindings,
compose, themes) will live here too.
**Date:** 2026-04-12 (Pass 2.5b-3.5)

### Folder classifier in `internal/mail/classify.go`
**Decision:** A pure `Classify(folders []Folder) []ClassifiedFolder`
function in `internal/mail/` maps raw backend folders to canonical
identity + group (Primary / Disposal / Custom). Priority is role
attribute → alias table → Custom fallback. The alias table is
verified against Gmail IMAP, Fastmail JMAP, Outlook/M365, iCloud,
Yahoo/AOL, and Proton Mail Bridge.
**Rationale:** Folder identity was previously scattered between
`sidebar.go:classifyGroup` (role-only) and `sidebar.go:sidebarIcon`
(role + name heuristic). Moving it to a pure function in the mail
package makes it testable in isolation, shareable between the
sidebar renderer and `poplar config init`, and backend-agnostic —
IMAP workers with `\Special-Use` flags set `Folder.Role` the same
way JMAP does. The alias table is the fallback for providers that
don't send role metadata (or send it inconsistently).
**Date:** 2026-04-12 (Pass 2.5b-3.5)

### Tea.Cmd-based backend I/O before Pass 3
**Decision:** `AccountTab.Init` returns a `loadFoldersCmd`. J/K
navigation dispatches `loadFolderCmd(name)`. Results come back as
`foldersLoadedMsg` and `folderLoadedMsg` handled in `Update`. The
synchronous `ListFolders` call in `NewAccountTab` and the
`loadSelectedFolder` helper in `handleKey` are both gone.
`AccountTab` emits `FolderChangedMsg` when selection moves; `App`
consumes it to update the status bar instead of reaching through
`m.acct.sidebar.SelectedFolderInfo()`. The dead `case ":":` stub
in `App.Update` is deleted.
**Rationale:** Pass 3 wires real JMAP/IMAP backends. Their
`ListFolders` and `FetchHeaders` calls take 200–500ms. Running
them in `Update` or constructors would freeze the UI on every
keypress and on startup. Fixing the pattern now — while the mock
backend is instant and the regression surface is small — is
cheaper than landing JMAP latency on top of a blocking Update
loop. This also matches the Elm architecture the `CLAUDE.md`
elm-conventions file mandates.
**Date:** 2026-04-12 (Pass 2.5b-3.5)
````

- [ ] **Step 2: Add the note to the two existing entries**

Find the "Minimal AccountConfig" entry (dated 2026-04-09 Pass 1). Append
at the end of its `**Rationale:**` paragraph:

> **Update 2026-04-12 (Pass 2.5b-3.5):** package moved to
> `internal/config/` alongside `UIConfig`.

Find the "Config in ~/.config/poplar/" entry (dated 2026-04-09 Pass 2).
Append the same update line at the end of its rationale.

### Task G3: Update `docs/poplar/keybindings.md`

**Files:**
- Modify: `docs/poplar/keybindings.md`

- [ ] **Step 1: Reword the Select section**

Replace the current `## Select (deferred — Pass 6)` section with:

```markdown
## Select (deferred — Pass 6)

Multi-select is not yet implemented. The bindings below are
reserved in the design so later passes don't collide with them.
`v` enters visual-select mode; inside that mode `Space` toggles
selection on the current row. Outside visual mode, `Space` is
the thread fold-toggle — see § Threads below. Both `v` and
`Space` are unbound until Pass 6 / Pass 2.5b-3.6 respectively.

| Key | Action | Context |
|-----|--------|---------|
| `v` | Enter/exit visual select *(reserved, Pass 6)* | A |
| `Space` | Toggle selection on current row *(inside visual mode, Pass 6)* | A |
```

- [ ] **Step 2: Add the Threads section**

Insert a new section after Select, before Viewer:

```markdown
## Threads (reserved — Pass 2.5b-3.6)

Threaded view and per-thread fold state ship in Pass 2.5b-3.6.
The bindings below are reserved now so no other pass can grab
them; they do nothing until that pass lands.

| Key | Action | Context |
|-----|--------|---------|
| `Space` | Toggle fold on thread under cursor *(reserved, 2.5b-3.6)* | A |
| `F` | Fold all threads *(reserved, 2.5b-3.6)* | A |
| `U` | Unfold all threads *(reserved, 2.5b-3.6)* | A |

`Space` is dual-purpose: inside visual-select mode (Pass 6) it
toggles row selection, outside visual mode it toggles thread
fold. See architecture.md "Thread fold key: Space, dual meaning
in visual-select mode".
```

### Task G4: Update `docs/poplar/wireframes.md`

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Fix line 257 (§ 5 Help Popover)**

Replace:
```
│  Search             Select          Threads             │
│  /    search        v  select       …  fold (TBD)      │
│  n    next          ␣  toggle                           │
│  N    prev                                              │
```

with:

```
│  Search             Select          Threads             │
│  /    search        v  select       ␣  fold             │
│  n    next          ␣  toggle       F  fold all         │
│  N    prev                          U  unfold all       │
```

- [ ] **Step 2: Fix line 470 (§ 7 Screen States #14)**

Replace:
```
Thread folded via the fold key (TBD — see Pass 2.5b-3.5
brainstorm). Shows message count badge.
```

with:

```
Thread folded via `Space` (Pass 2.5b-3.6). Shows message count
badge.
```

- [ ] **Step 3: Fix lines 521–527 (thread annotation)**

Replace:
```
- **Thread collapse (#14):** Fold key TBD — the original
  `zo`/`zc`/`za` proposal violates the no-multikey rule
  (architecture.md). Candidates under discussion are `Tab`
  and `Space`; final choice is pending the Pass 2.5b-3.5
  brainstorm. Fold-all / unfold-all may ship in this pass or
  be deferred. Collapsed thread shows `[N]` count in `fg_dim`
  before subject. Thread root always visible. Count includes
  root.
```

with:

```
- **Thread collapse (#14):** `Space` toggles the fold under the
  cursor, `F` folds all, `U` unfolds all (Pass 2.5b-3.6). Space
  is dual-purpose: inside visual-select mode (Pass 6) it
  toggles row selection, outside visual mode it toggles fold.
  Collapsed thread shows `[N]` count in `fg_dim` before subject.
  Thread root always visible. Count includes root.
```

### Task G5: Update `docs/poplar/STATUS.md`

**Files:**
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 1: Mark Pass 2.5b-3.5 done in the passes table**

Find the row `| 2.5b-3.5 | Prototype: UI config + sidebar polish | pending |`
and change its status to `done`.

- [ ] **Step 2: Update the "Current state" paragraph at the top of the file**

Rewrite the **Current state** paragraph to reflect:

- `internal/config/` package now holds `AccountConfig` + `UIConfig` +
  `LoadUI`.
- Folder classifier in `internal/mail/classify.go`.
- Sidebar consumes `[]mail.ClassifiedFolder` + `config.UIConfig` with
  rank, label, hide, one-space nested indent capped at depth 3.
- `poplar config init` subcommand discovers folders and merges
  `[ui.folders.<name>]` subsections.
- Backend I/O is Cmd-based via `loadFoldersCmd` / `loadFolderCmd`;
  `AccountTab` emits `FolderChangedMsg`; `App` status bar updates via
  that message instead of grandchild peek.

Keep this to one dense paragraph (same shape as the existing "Current
state" paragraph).

- [ ] **Step 3: Update "Next steps"**

Change the "Next steps" list so that "Execute Pass 2.5b-3.5" is removed
and Pass 2.5b-3.6 is promoted to the top. The 2.5b-3.6 starter prompt
already in STATUS.md stays as-is.

### Task G6: Run the full check and commit Phase G

- [ ] **Step 1: Run `/simplify`-equivalent review manually**

The project CLAUDE.md says `/simplify` should run before committing code
changes. Phase G is docs-only, but the prior phases weren't reviewed.
Run `make check` and `go vet ./...` one final time to confirm everything
still passes.

Run: `make check`
Expected: PASS.

- [ ] **Step 2: Smoke-test the installed binary one more time**

Run: `make install`
Run: `poplar` (user launches manually)

Expected: same as Task E6 — no regression, all features working.

Run: `poplar config init` (dry-run against the user's real config)
Expected: prints merged output to stdout; no file written.

- [ ] **Step 3: Stage and commit docs**

```bash
git add docs/poplar/architecture.md docs/poplar/keybindings.md docs/poplar/wireframes.md docs/poplar/STATUS.md accounts.toml.example internal/config/accounts_test.go
git commit -m "$(cat <<'EOF'
Docs and shipped example for Pass 2.5b-3.5

Architecture: three new decisions (config package move, folder
classifier, Cmd-based I/O) plus note-backs on the two existing
AccountConfig decisions.

Keybindings: Select section reworded for dual-meaning Space;
new Threads (reserved) section pointing at 2.5b-3.6.

Wireframes: fold-key TBDs replaced with Space/F/U.

STATUS: mark 2.5b-3.5 done, next-steps bump to 2.5b-3.6.

Ships accounts.toml.example at repo root with all canonical
[ui.folders.<name>] subsections pre-filled as commented hints.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: Push**

Run: `git push`
Expected: success.

---

## Pass-End Checklist

After all seven phases commit cleanly, run the pass-end checklist from
`docs/poplar/STATUS.md`:

- [ ] `/simplify` — code quality review (launches reuse + quality +
  efficiency agents; aggregate findings, apply genuine wins as a
  follow-up commit if any).
- [ ] Verify `docs/poplar/architecture.md` reflects all decisions made
  in this pass (three new entries + two note-backs — already done in G2).
- [ ] Verify `docs/poplar/STATUS.md` marks 2.5b-3.5 done and points at
  2.5b-3.6 (done in G5).
- [ ] Verify `docs/poplar/keybindings.md` and `docs/poplar/wireframes.md`
  cleanup landed (done in G3 + G4).
- [ ] Final `make check` passes.
- [ ] Final `git push`.

---

## Spec Coverage Summary

| Spec section | Phase | Task(s) |
|---|---|---|
| `internal/config/` package | A | A1–A6 |
| `[ui]` section parsing | B | B1–B4 |
| Folder classifier + alias table | C | C1–C4 |
| Sidebar renderer polish | D | D1–D4 |
| Cmd-based backend I/O (item 1) | E | E1, E3, E4, E7 |
| Parent→grandchild peek removal (item 2) | E | E1 (FolderChangedMsg), E5 |
| NewApp stops calling ListFolders (item 3) | E | E5 |
| Remove folders[0] assumption (item 4) | E | E5 |
| Remove dead `:` stub (item 5) | E | E5 |
| Package move (`internal/poplar/` → `internal/config/`) | A | A1–A6 |
| `poplar config init` subcommand | F | F1–F7 |
| Shipped example accounts.toml | G | G1 |
| architecture.md updates | G | G2 |
| keybindings.md updates | G | G3 |
| wireframes.md updates | G | G4 |
| STATUS.md updates | G | G5 |
| Testing (config, classifier, sidebar, account_tab, config_init) | B, C, D, E, F | B2, C1, D3, E3, F1, F4 |
| Mock backend alias-path exercise | C | C3 |
