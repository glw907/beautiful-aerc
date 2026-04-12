# Poplar UI Config + Sidebar Polish + Elm Foundation Design

Pass 2.5b-3.5. Three tightly-related threads: the first non-account
config section (`[ui]`), sidebar rendering polish (classification,
ranks, nested indent, label override, hide), and an Elm/bubbletea
refactor that converts the existing synchronous I/O to `tea.Cmd` flow
before Pass 3 wires real JMAP.

## Scope

1. **`internal/config/` package.** New top-level config package
   holding both `AccountConfig` (moved from `internal/poplar/`) and
   the new `UIConfig`. `internal/poplar/` is deleted.
2. **`[ui]` section parsing.** Global defaults + per-folder overrides
   via `[ui.folders.<name>]` subsections.
3. **Folder classifier.** New `internal/mail/classify.go` maps raw
   backend folders into Primary / Disposal / Custom groups via role
   detection and an alias table verified against the six major
   providers.
4. **Sidebar renderer polish.** Consumes `UIConfig` + classifier
   output to apply rank, nested indent, label override, and hide
   rules.
5. **`poplar config init` subcommand.** Discovers folders from the
   configured backend, merges into `accounts.toml` with commented
   defaults.
6. **Elm/bubbletea refactor.** Convert synchronous backend I/O to
   `tea.Cmd`-dispatched flow. Stop parent models reaching into
   grandchildren. Remove dead command-mode stub.
7. **Keybindings doc + wireframes cleanup.** Finish the doc debt
   from prior passes.

Out of scope (deferred to 2.5b-3.6): thread rendering, fold state,
`Space`/`F`/`U` bindings. Threading-related config fields are
parsed and stored this pass — the consumer ships next pass.

## Config Schema

### File location

Existing: `~/.config/poplar/accounts.toml`. Unchanged.

### Global `[ui]` table

```toml
[ui]
threading = true   # default for folders without an override
```

- `threading` (bool, default `true`) — global threading default.
  Parsed and stored this pass; consumer is Pass 2.5b-3.6.

### Per-folder `[ui.folders.<name>]` subsections

```toml
[ui.folders.Inbox]
rank = 1
threading = false
sort = "date-asc"

[ui.folders."[Gmail]/Starred"]
label = "Starred"
rank = 5

[ui.folders."[Gmail]/All Mail"]
hide = true
```

Fields:

- `rank` (int, unset = group-default position) — user-specified
  sort key within the folder's group. Lower sorts first. Unset
  folders use their group-default position as their implicit rank:
  Primary canonicals get `100`, `200`, `300`, `400` (Inbox, Drafts,
  Sent, Archive); Disposal canonicals get `100`, `200` (Spam,
  Trash); Custom folders get `1000` so any explicit rank pins
  above the alphabetical default. Ties break on display name.
  Out-of-group rank is ignored — cross-group movement is forbidden.
  Negative ranks are valid and sort numerically (useful for
  pinning one custom folder above everything).
- `label` (string, unset = default display) — display-name override.
  Primary intended for custom folders whose provider name is noisy
  (`[Gmail]/Starred` → `Starred`). Canonical folders already display
  normalized names from the classifier, so `label` is rarely needed
  for them.
- `threading` (bool, unset = global default) — per-folder override.
  Parsed this pass; consumer is Pass 2.5b-3.6.
- `sort` (enum, `"date-asc"` | `"date-desc"`, unset = `"date-desc"`)
  — per-folder sort. Parsed this pass; consumer is Pass 2.5b-3.6.
- `hide` (bool, unset = `false`) — skip the folder entirely in the
  sidebar. Backend still lists it; user can't navigate to it from
  the UI.

### Section key convention

- **Canonical folders**: key is the canonical name (`Inbox`,
  `Drafts`, `Sent`, `Archive`, `Spam`, `Trash`). The classifier
  maps the provider name (`[Gmail]/Sent Mail`, `Sent Items`, etc.)
  to the canonical name before lookup.
- **Custom folders**: key is the literal provider name, quoted when
  it contains special characters (`[ui.folders."Lists/golang"]`,
  `[ui.folders."[Gmail]/Starred"]`).

### Shipped example

The example `accounts.toml` in the repo ships with every canonical
subsection pre-filled as commented hints:

```toml
[ui]
threading = true

[ui.folders.Inbox]
# rank = 1
# threading = true
# sort = "date-desc"

[ui.folders.Drafts]
# rank = 2

[ui.folders.Sent]
# rank = 3

[ui.folders.Archive]
# rank = 4

[ui.folders.Spam]
# rank = 1

[ui.folders.Trash]
# rank = 2
```

Users discover the tunable fields without reading docs.

## Folder Classification

### Priority

1. Backend `Role` field (JMAP role, IMAP `\Special-Use` flag per
   RFC 6154) — authoritative when present.
2. Alias table — case-insensitive exact match on provider name.
3. Otherwise → Custom.

### Alias table

| Canonical | Aliases (case-insensitive exact match) |
|-----------|---------------------------------------|
| Inbox | `inbox` |
| Drafts | `drafts`, `draft`, `[Gmail]/Drafts` |
| Sent | `sent`, `sent mail`, `sent items`, `sent messages`, `[Gmail]/Sent Mail` |
| Archive | `archive`, `all mail`, `[Gmail]/All Mail` |
| Spam | `spam`, `junk`, `junk email`, `junk e-mail`, `bulk mail`, `[Gmail]/Spam` |
| Trash | `trash`, `deleted`, `deleted items`, `deleted messages`, `bin`, `[Gmail]/Trash` |

Verified against: Gmail IMAP, Fastmail JMAP, Outlook/Microsoft 365
IMAP, iCloud IMAP, Yahoo/AOL IMAP, Proton Mail Bridge.

Deliberate omissions from the alias table:

- `[Gmail]/Starred`, `[Gmail]/Important`, `[Gmail]/Category/*` —
  labels, not folder roles. Go to Custom.
- `Outbox`, `Conversation History`, `Clutter`, `Templates` — client
  concepts or provider-specific extras. Go to Custom.

### Group assignment

| Group | Canonicals |
|-------|------------|
| Primary | Inbox, Drafts, Sent, Archive |
| Disposal | Spam, Trash |
| Custom | Everything else |

Within-group default order:

- **Primary**: canonical order (Inbox, Drafts, Sent, Archive).
- **Disposal**: canonical order (Spam, Trash).
- **Custom**: alphabetical on display name.

### Display name normalization

- Classified canonicals display their canonical name (`Sent`, not
  `Sent Items`), unless `label` overrides.
- Custom folders display the literal provider name, unless `label`
  overrides.

### Package location

`internal/mail/classify.go` — pure function
`Classify(folders []Folder) []ClassifiedFolder`, no backend
dependency, testable in isolation. The mock backend includes at
least one aliased folder (e.g. `Junk` instead of `Spam`) to exercise
the alias path in tests.

## Sidebar Rendering

### Order within the sidebar

1. Primary group (Inbox, Drafts, Sent, Archive) — canonical order,
   `rank` overrides respected.
2. Blank line.
3. Disposal group (Spam, Trash) — canonical order, `rank` overrides
   respected.
4. Blank line.
5. Custom group — alphabetical by display name, `rank` overrides
   respected.

Folders with `hide = true` are dropped before ordering.

### Nested folder indent

Folders whose names contain `/` get one leading space per depth
level, capped at depth 3.

- `Lists/golang` → indent depth 1 (one leading space)
- `Projects/Acme/Planning` → indent depth 2 (two leading spaces)
- `Projects/Acme/Planning/Q2` → indent depth 3 (three leading spaces)
- `Projects/Acme/Planning/Q2/Week1` → still indent depth 3 (cap)

Indent applies to the `label` if one is set, or to the raw name
otherwise. The selection indicator `┃` always sits in column 0 —
it is not indented.

### Rendering contract

The sidebar receives `[]ClassifiedFolder` + `UIConfig` at
construction and re-runs its internal ordering on every
`SetFolders` or config change. Rank/label/hide are applied during
ordering; indent is applied during row rendering.

## `poplar config init` Subcommand

New cobra subcommand under `cmd/poplar/`.

### Behavior

1. Loads `accounts.toml` from its default path (or `--config`
   override).
2. For each configured account, connects to the backend and calls
   `ListFolders`.
3. Classifies each folder via the same classifier used by the
   sidebar.
4. Builds a set of existing `[ui.folders.<name>]` keys from the
   current file.
5. For every discovered folder not yet in the file, generates a
   subsection with commented defaults:
   ```toml
   [ui.folders."Lists/golang"]
   # label = "golang"
   # rank = 0
   # threading = true
   # sort = "date-desc"
   # hide = false
   ```
6. Existing subsections are untouched — no overwrites, no deletions.
7. Output is grouped: Primary subsections first, Disposal, then
   Custom, with a blank line between groups.

### Dry-run default

Without `--write`, prints the merged file to stdout as a diff
against the original. With `--write`, replaces `accounts.toml`
atomically (write to temp + rename).

### Idempotence

Running `poplar config init --write` twice in a row is a no-op.
Running after creating a new provider folder only adds subsections
for the new folder.

### Error handling

- Missing `accounts.toml` → clear error pointing at the default path.
- Backend connection failure → error with the account name and
  underlying cause. v1 is single-account; multi-account retry
  policy is a future concern.

## Elm / Bubbletea Refactor

The goal is to establish the patterns Pass 3 will depend on — real
JMAP backends have 200-500ms latency, and the current synchronous
flow will freeze the UI on every keypress. Fixing now is cheaper
than fixing later on top of the JMAP wiring.

### Item 1: Cmd-based backend I/O

**Current shape:**

```go
// NewAccountTab calls ListFolders synchronously in the constructor.
func NewAccountTab(styles Styles, backend mail.Backend) AccountTab {
    folders, _ := backend.ListFolders()
    sb := NewSidebar(styles, folders, sidebarWidth, 1)
    tab := AccountTab{...}
    tab.loadSelectedFolder() // also synchronous
    return tab
}

// loadSelectedFolder runs on every J/K keypress, blocking.
func (m *AccountTab) loadSelectedFolder() {
    m.backend.OpenFolder(name)
    msgs, _ := m.backend.FetchHeaders(nil)
    m.msglist.SetMessages(msgs)
}
```

**Target shape:**

```go
// NewAccountTab builds an empty model. The initial fetch is a Cmd.
func NewAccountTab(styles Styles, backend mail.Backend, uiCfg config.UIConfig) AccountTab {
    return AccountTab{
        styles: styles, backend: backend, uiCfg: uiCfg,
        sidebar: NewSidebar(styles, nil, sidebarWidth, 1),
        msglist: NewMessageList(styles, nil, 1, 1),
    }
}

// Init fires the initial folder-list fetch.
func (m AccountTab) Init() tea.Cmd {
    return loadFoldersCmd(m.backend)
}

// Update handles the fetch results via typed messages.
func (m AccountTab) Update(msg tea.Msg) (AccountTab, tea.Cmd) {
    switch msg := msg.(type) {
    case foldersLoadedMsg:
        m.sidebar.SetFolders(Classify(msg.folders), m.uiCfg)
        return m, loadFolderCmd(m.backend, m.sidebar.SelectedFolder())
    case folderLoadedMsg:
        m.msglist.SetMessages(msg.msgs)
        return m, nil
    case tea.KeyMsg:
        return m.handleKey(msg)
    }
    return m, nil
}

// handleKey returns (model, cmd). Navigation keys dispatch a
// folder-load Cmd; no synchronous I/O.
func (m AccountTab) handleKey(msg tea.KeyMsg) (AccountTab, tea.Cmd) {
    switch msg.String() {
    case "J":
        m.sidebar.MoveDown()
        return m, loadFolderCmd(m.backend, m.sidebar.SelectedFolder())
    // ...
    }
    return m, nil
}
```

**Command helpers** live in `internal/ui/cmds.go`:

```go
type foldersLoadedMsg struct { folders []mail.Folder }
type folderLoadedMsg  struct { name string; msgs []mail.MessageInfo }
type backendErrMsg    struct { err error }

func loadFoldersCmd(b mail.Backend) tea.Cmd {
    return func() tea.Msg {
        folders, err := b.ListFolders()
        if err != nil { return backendErrMsg{err} }
        return foldersLoadedMsg{folders}
    }
}

func loadFolderCmd(b mail.Backend, name string) tea.Cmd {
    return func() tea.Msg {
        if err := b.OpenFolder(name); err != nil {
            return backendErrMsg{err}
        }
        msgs, err := b.FetchHeaders(nil)
        if err != nil { return backendErrMsg{err} }
        return folderLoadedMsg{name: name, msgs: msgs}
    }
}
```

`backendErrMsg` has no handler yet — the pass leaves it as a TODO
for Pass 2.5b-6 (status/toast). It's not dropped; the type exists
and the Update arm logs and moves on.

### Item 2: Parent → grandchild peek removal

**Current:** `App.syncStatusBar()` reaches through
`m.acct.sidebar.SelectedFolderInfo()`.

**Target:** `AccountTab` emits a `FolderChangedMsg{name, exists, unseen}`
every time the selected folder changes (after `foldersLoadedMsg`
loads or after J/K/folder-jump keys move the selection). `App.Update`
consumes the message and updates `statusBar`. `App` no longer knows
the sidebar exists.

```go
type FolderChangedMsg struct {
    Name   string
    Exists int
    Unseen int
}
```

Emission is a `tea.Cmd` that returns the message synchronously — a
zero-latency Cmd that just wraps a value. This keeps message flow
going through the normal Update path instead of creating an
ad-hoc mutation channel.

### Item 3: `NewApp` stops calling `ListFolders`

Falls out of items 1 and 2. `App` no longer fetches folders — it
receives `FolderChangedMsg` from `AccountTab` and updates
`statusBar` from there. Status bar starts with zero counts until
the first message arrives.

### Item 4: Remove `folders[0]` assumption

Same fix as item 3 — status bar initial state is zero, first
`FolderChangedMsg` populates it. Rank-aware ordering means "index
0 is Inbox" was already wrong for any user with custom ranking.

### Item 5: Remove dead `:` command-mode stub

Delete `case ":":` from `App.Update`. Command mode was dropped
(architecture.md, 2026-04-12).

### Deferred / logged to BACKLOG.md

- **Pointer-receiver `handleKey` inside value-returning Update.**
  Gray area, not a bug, cosmetic-only. Not worth the churn.
- **`NewApp(t *theme.CompiledTheme, ...)` pointer type.** Nothing
  mutates it; could be a value. Cosmetic. Defer to a future
  simplify pass.

## Package Moves

`internal/poplar/` → `internal/config/`.

- `internal/poplar/accounts.go` → `internal/config/accounts.go`
  (functions rename: `ParseAccounts` stays; type references
  `AccountConfig` stay same name).
- `internal/poplar/accounts_test.go` → `internal/config/accounts_test.go`.
- `internal/poplar/config.go` (struct definition) →
  `internal/config/account.go`.
- `internal/poplar/` directory deleted.

Imports updated:

- `cmd/poplar/root.go`
- `internal/mail/jmap.go`
- `internal/aercfork/worker/jmap/worker.go`
- `internal/aercfork/worker/types/messages.go`

New file `internal/config/ui.go` holds `UIConfig`, `FolderConfig`,
and `LoadUI(path) (UIConfig, error)`. `LoadUI` reads the same
`accounts.toml` but only decodes the `[ui]` table. `ParseAccounts`
continues to ignore the `[ui]` table (BurntSushi/toml silently
drops unknown top-level keys).

`cmd/poplar/root.go` calls both:

```go
accounts, err := config.ParseAccounts(configPath)
if err != nil { return err }
uiCfg, err := config.LoadUI(configPath)
if err != nil { return err }
```

## Documentation Updates

### `docs/poplar/architecture.md`

- Mark "Minimal AccountConfig" (2026-04-09) with a note: package
  moved from `internal/poplar/` to `internal/config/` in Pass
  2.5b-3.5 to consolidate config concerns.
- Mark "Config in ~/.config/poplar/" (2026-04-09) with the same
  note.
- New decision entry: "UIConfig and AccountConfig in internal/config/".
- New decision entry: "Folder classifier in internal/mail/".
- New decision entry: "Tea.Cmd-based I/O established before Pass 3
  wires real backends".

### `docs/poplar/keybindings.md`

- Reword § Select intro: `v` enters visual-select mode; `Space`
  toggles row selection inside that mode. Outside visual mode,
  `Space` is the thread fold-toggle (Pass 2.5b-3.6, deferred).
- Add § Threads (reserved — Pass 2.5b-3.6) with `Space` (fold
  toggle), `F` (fold all), `U` (unfold all).

### `docs/poplar/wireframes.md`

- § 5 Help Popover, line 259: replace `… fold (TBD)` with
  `Space fold  F/U all`.
- § 7 Screen States #14, lines 470, 521-527: replace "fold key
  TBD" text with the Pass 2.5b-3.6 reference.

### `docs/poplar/STATUS.md`

- Mark Pass 2.5b-3.5 done with a one-line summary.
- Update "Next steps" to point at 2.5b-3.6.
- Next starter prompt is already drafted in STATUS.md; leave it.

## Testing

- **`internal/config/` unit tests.** Table-driven tests for
  `LoadUI` covering: missing file, empty `[ui]`, global-only,
  per-folder overrides, unknown fields (ignored), invalid enum
  values (rejected with clear error).
- **`internal/mail/classify_test.go`** — table-driven tests of
  `Classify` covering every row of the alias table, mixed
  role/alias inputs, all-custom-folder input, empty input.
- **`internal/ui/sidebar_test.go` additions** — rank ordering,
  nested indent at every depth, label override, hide, display
  normalization for canonicals.
- **`internal/ui/account_tab_test.go` additions** — Init returns
  the initial load Cmd; foldersLoadedMsg seeds the sidebar;
  folderLoadedMsg seeds the message list; J/K dispatch a Cmd,
  not a sync call.
- **`cmd/poplar/config_init_test.go`** — dry-run output format,
  merge correctness, idempotence (run twice, second run no-op),
  ordering in output.
- **Mock backend** gains a `Junk` folder (instead of `Spam`) to
  exercise the alias-based classification path.

## Implementation Notes

### BurntSushi/toml behavior

`toml.Unmarshal` silently drops keys not present in the target
struct. This means `ParseAccounts` parsing into a struct that
doesn't mention `[ui]` will ignore it, and `LoadUI` parsing into
a struct that only has `[ui]` will ignore `[[account]]`. Both
decodings can read the same file without collision.

### Config file I/O atomicity for `config init --write`

Write new content to `accounts.toml.tmp` in the same directory,
`fsync`, `os.Rename` to target. Preserves the file on crash mid-write.

### Commented defaults: TOML encoder won't emit comments

The standard BurntSushi encoder writes key=value only, no comments.
`config init` uses a hand-rolled writer (`internal/config/writer.go`)
that emits subsection headers, commented field lines, and blank
lines between groups. Writer is straightforward — formatting is
fixed, no dynamic indentation. Tests compare output to golden
files.

## Risks

1. **Scope is meatier than the original 2.5b-3.5 envelope.** Mitigated
   by breaking the plan into phases that each leave the tree in a
   compilable, testable state, so a fresh Claude Code context can
   resume cleanly if one session runs long.
2. **Elm refactor touches all existing UI code.** Mitigated by
   keeping the refactor mechanical: every sync call site becomes a
   Cmd dispatch; message types are narrow; no behavior change for
   the mock backend (user sees identical UI before and after).
3. **`poplar config init` writer hand-rolled instead of library-based.**
   Mitigated by the formatting being fixed (no user templates),
   golden-file tests, and the writer being isolated to one file.
