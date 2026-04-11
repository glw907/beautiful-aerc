# Elm Architecture Conventions for Poplar UI

Disciplined architecture guide for the poplar bubbletea UI layer
(`internal/ui/`). Defines five rules enforced through documentation,
CLAUDE.md directives, and Claude Code hooks.

## Scope

These conventions apply exclusively to `internal/ui/` — the poplar
bubbletea UI layer. They do not apply to:

- `internal/mail/` (backend adapter — goroutines, mutexes, blocking
  calls are normal)
- `internal/aercfork/` (forked worker code — channel-based async)
- `internal/filter/` (stdin/stdout filters — no tea loop)
- `cmd/poplar/` (bootstrap wiring — sets up dependencies before the
  tea loop starts)

## Rules

### Rule 1: All state in models

Every component is a `tea.Model` struct. No package-level mutable
variables, no singletons, no shared mutable state outside the model
tree.

**Right:**
```go
type Sidebar struct {
    folders []mail.Folder
    cursor  int
    width   int
}
```

**Wrong:**
```go
var currentFolders []mail.Folder // package-level mutable state

type Sidebar struct {
    cursor int
}
```

### Rule 2: Update is the only mutation point

State changes happen only by returning a new model from `Update`.
No mutating model fields in `View`, `Init`, or `Cmd` closures.

**Right:**
```go
func (m Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
    switch msg := msg.(type) {
    case FoldersLoadedMsg:
        m.folders = msg.Folders
        return m, nil
    }
    return m, nil
}
```

**Wrong:**
```go
func (m Sidebar) View() string {
    m.lastRendered = time.Now() // mutation in View
    return m.render()
}
```

**Wrong:**
```go
func fetchFolders(m *Sidebar, backend mail.Backend) tea.Cmd {
    return func() tea.Msg {
        folders, _ := backend.ListFolders()
        m.folders = folders // mutation in Cmd closure
        return nil
    }
}
```

### Rule 3: All I/O in Cmds

Blocking calls (backend methods, file I/O, network) run inside
`tea.Cmd` functions, never in `Update` or `View`. The `Update`
function must return instantly.

**Right:**
```go
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FolderSelectedMsg:
        return m, fetchHeaders(m.backend, msg.Name)
    }
    return m, nil
}

func fetchHeaders(b mail.Backend, folder string) tea.Cmd {
    return func() tea.Msg {
        headers, err := b.FetchHeaders(folder, nil)
        if err != nil {
            return ErrMsg{Err: err}
        }
        return HeadersLoadedMsg{Headers: headers}
    }
}
```

**Wrong:**
```go
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FolderSelectedMsg:
        headers, _ := m.backend.FetchHeaders(msg.Name, nil) // blocks UI
        m.headers = headers
        return m, nil
    }
    return m, nil
}
```

### Rule 4: Message-driven communication

Children signal parents by returning sentinel `Msg` types from
`Update`. No parent method calls, no upward pointers, no callbacks
stored in child models.

**Right:**
```go
// Child defines its own message type
type FolderSelectedMsg struct {
    Name string
}

func (m Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            folder := m.folders[m.cursor]
            return m, func() tea.Msg {
                return FolderSelectedMsg{Name: folder.Name}
            }
        }
    }
    return m, nil
}

// Parent handles it in its own Update
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FolderSelectedMsg:
        return m, fetchHeaders(m.backend, msg.Name)
    }
    // delegate to children...
}
```

**Wrong:**
```go
type Sidebar struct {
    onSelect func(string) // callback — couples child to parent
}
```

### Rule 5: State ownership, not duplication

Shared state lives in the root model and is passed to children as
read-only values during `Update` delegation or as constructor
parameters. Children never own copies of data they don't exclusively
control.

**Right:**
```go
type App struct {
    backend  mail.Backend  // owned by root
    styles   Styles        // computed once, shared read-only
    tabs     []Tab
    active   int
}

// Styles passed to child View
func (m App) View() string {
    return m.tabs[m.active].View(m.styles)
}
```

**Wrong:**
```go
type Sidebar struct {
    backend mail.Backend // child owns a copy of shared dependency
}

type MsgList struct {
    backend mail.Backend // another copy — who's authoritative?
}
```

**Exception:** A child may hold a reference to a read-only dependency
(like `mail.Backend`) if it needs to create `tea.Cmd` closures that
call backend methods. The key constraint is that the child never
mutates the dependency or caches its results as owned state. Results
come back as `Msg` types through the normal Update flow.

## Deliverables

### 1. Reference doc: `~/.claude/docs/elm-conventions.md`

Standalone conventions doc containing the five rules above with code
examples. Referenced from CLAUDE.md via `@` directive so it
auto-loads for any conversation touching `internal/ui/`.

### 2. CLAUDE.md addition

New mandatory section in the project CLAUDE.md:

```
## MANDATORY: Elm Architecture (Poplar UI)

**Read and follow `~/.claude/docs/elm-conventions.md` before writing
ANY code in `internal/ui/`.** Key rules:

- All state in tea.Model structs, no package-level mutable vars
- State changes only in Update, never in View/Init/Cmd closures
- All I/O in tea.Cmd, never blocking in Update or View
- Children signal parents via Msg types, never method calls
- Shared state hoisted to root, passed down read-only
```

### 3. Claude Code hook: `.claude/hooks/elm-architecture-lint.sh`

PostToolUse hook on Edit/Write. Only fires for files matching
`internal/ui/**/*.go` (excludes test files).

**Checks:**

| Pattern | Rule violated | Warning |
|---------|--------------|---------|
| `go func` or `go ` inside `func.*Update` | Rule 2, 3 | Goroutine in Update — use tea.Cmd |
| `backend.`, `adapter.`, `os.Open`, `http.` inside Update or View | Rule 3 | Blocking I/O in Update/View — move to tea.Cmd |
| Package-level `var` with mutable types (slice, map, struct literal) | Rule 1 | Package-level mutable state — move to model struct |
| `sync.Mutex` or `sync.RWMutex` | Rule 2 | Mutex in UI code — trust the tea event loop |

Warnings emit to stderr (non-blocking). The edit succeeds regardless.

**Hook registration:** Added to the existing PostToolUse array in
`.claude/settings.json` for both Edit and Write matchers.

## Non-goals

- No AST-based analysis — grep heuristics are sufficient for the
  common violations and have zero dependencies
- No enforcement outside `internal/ui/` — other packages have
  legitimate reasons for goroutines, mutexes, and blocking calls
- No framework layer or helper types — bubbletea's raw API is
  sufficient; abstractions should emerge from need, not speculation
