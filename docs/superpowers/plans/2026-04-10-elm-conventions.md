# Elm Architecture Conventions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce Elm Architecture discipline in poplar's bubbletea UI layer through documentation, CLAUDE.md directives, and a Claude Code lint hook.

**Architecture:** Three enforcement layers — a reference doc (`~/.claude/docs/elm-conventions.md`) with rules and code examples, a mandatory section in the project `CLAUDE.md`, and a PostToolUse hook that greps `internal/ui/` files for anti-patterns on every Edit/Write.

**Tech Stack:** Bash (hook script), Markdown (docs), JSON (settings.json)

---

### Task 1: Write the reference doc

**Files:**
- Create: `~/.claude/docs/elm-conventions.md`

- [ ] **Step 1: Create the conventions doc**

Write `~/.claude/docs/elm-conventions.md` with the following content. This mirrors the style of `~/.claude/docs/go-conventions.md` — terse rules with right/wrong code examples.

```markdown
# Elm Architecture Conventions (Poplar UI)

Rules for writing bubbletea UI code in `internal/ui/`. These do NOT
apply to `internal/mail/`, `internal/aercfork/`, `internal/filter/`,
or `cmd/poplar/` — those packages have their own patterns.

## Rule 1: All State in Models

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

## Rule 2: Update Is the Only Mutation Point

State changes happen only by returning a new model from `Update`.
Never mutate model fields in `View`, `Init`, or `Cmd` closures.

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

**Wrong — mutation in View:**
```go
func (m Sidebar) View() string {
    m.lastRendered = time.Now()
    return m.render()
}
```

**Wrong — mutation in Cmd closure:**
```go
func fetchFolders(m *Sidebar, backend mail.Backend) tea.Cmd {
    return func() tea.Msg {
        folders, _ := backend.ListFolders()
        m.folders = folders
        return nil
    }
}
```

## Rule 3: All I/O in Cmds

Blocking calls (backend methods, file I/O, network) run inside
`tea.Cmd` functions, never in `Update` or `View`. `Update` must
return instantly.

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

**Wrong — blocking in Update:**
```go
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FolderSelectedMsg:
        headers, _ := m.backend.FetchHeaders(msg.Name, nil)
        m.headers = headers
        return m, nil
    }
    return m, nil
}
```

## Rule 4: Message-Driven Communication

Children signal parents by returning sentinel `Msg` types from
`Update`. No parent method calls, no upward pointers, no callbacks
stored in child models.

**Right — child emits message, parent handles it:**
```go
// In sidebar.go
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

// In app.go
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FolderSelectedMsg:
        return m, fetchHeaders(m.backend, msg.Name)
    }
    // delegate to children...
}
```

**Wrong — callback couples child to parent:**
```go
type Sidebar struct {
    onSelect func(string)
}
```

## Rule 5: State Ownership, Not Duplication

Shared state lives in the root model and is passed to children as
read-only values during `Update` delegation or as constructor
parameters. Children never own copies of data they don't exclusively
control.

**Right:**
```go
type App struct {
    backend  mail.Backend
    styles   Styles
    tabs     []Tab
    active   int
}

func (m App) View() string {
    return m.tabs[m.active].View(m.styles)
}
```

**Wrong — duplicated dependency:**
```go
type Sidebar struct {
    backend mail.Backend
}

type MsgList struct {
    backend mail.Backend
}
```

**Exception:** A child may hold a reference to a read-only dependency
(like `mail.Backend`) if it needs to create `tea.Cmd` closures that
call backend methods. The child never mutates the dependency or caches
its results as owned state. Results come back as `Msg` types through
the normal `Update` flow.

## Parent-Child Update Delegation Pattern

The canonical pattern for the root model's `Update`:

```go
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // 1. Parent handles its own messages first
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // tab switching, quit, etc.
    case FolderSelectedMsg:
        cmds = append(cmds, fetchHeaders(m.backend, msg.Name))
    }

    // 2. Delegate to active child, collect its cmds
    var cmd tea.Cmd
    m.tabs[m.active], cmd = m.tabs[m.active].Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

## Cmd Closures: Capture Values, Not Pointers

Cmd closures run in goroutines after `Update` returns. Capture
the values you need, never the model pointer.

**Right:**
```go
case FolderSelectedMsg:
    name := msg.Name // capture value
    return m, func() tea.Msg {
        headers, err := b.FetchHeaders(name, nil)
        // ...
    }
```

**Wrong:**
```go
case FolderSelectedMsg:
    return m, func() tea.Msg {
        headers, err := m.backend.FetchHeaders(msg.Name, nil) // m may be stale
        // ...
    }
```
```

- [ ] **Step 2: Verify the file was created**

Run: `head -5 ~/.claude/docs/elm-conventions.md`
Expected: The first 5 lines of the conventions doc.

- [ ] **Step 3: Commit**

```bash
git add ~/.claude/docs/elm-conventions.md
git commit -m "Add Elm architecture conventions doc for poplar UI

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Add CLAUDE.md directive

**Files:**
- Modify: `CLAUDE.md:8` (after the Poplar section, before Go Conventions)

- [ ] **Step 1: Add the Elm Architecture section to CLAUDE.md**

Insert the following section after the `## Poplar` section (after `@docs/poplar/architecture.md`) and before `## MANDATORY: Go Conventions`:

```markdown
## MANDATORY: Elm Architecture (Poplar UI)

**Read and follow `~/.claude/docs/elm-conventions.md` before writing
ANY code in `internal/ui/`.** Key rules:

- All state in tea.Model structs, no package-level mutable vars
- State changes only in Update, never in View/Init/Cmd closures
- All I/O in tea.Cmd, never blocking in Update or View
- Children signal parents via Msg types, never method calls
- Shared state hoisted to root, passed down read-only
```

- [ ] **Step 2: Verify the section appears correctly**

Run: `grep -A 8 "Elm Architecture" CLAUDE.md`
Expected: The full section with all five bullet points.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "Add Elm architecture mandate to CLAUDE.md for poplar UI

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Write the lint hook

**Files:**
- Create: `.claude/hooks/elm-architecture-lint.sh`

- [ ] **Step 1: Create the hook script**

Write `.claude/hooks/elm-architecture-lint.sh` with the following content:

```bash
#!/usr/bin/env bash
# Hook: lint internal/ui/ files for Elm Architecture violations
# PostToolUse on Edit/Write targeting internal/ui/**/*.go

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only check internal/ui/ Go files, skip tests
if [[ "$file" != *"/internal/ui/"* ]] || [[ "$file" == *"_test.go" ]]; then
    exit 0
fi

if [[ ! -f "$file" ]]; then
    exit 0
fi

warnings=""

# Rule 1: No package-level mutable state (var with slice, map, or struct)
if grep -nE '^\s*var\s+\w+\s+((\[\]|\*|map\[)|\w+\{)' "$file" | grep -vq '^\s*//'; then
    warnings+="  Rule 1 violation: Package-level mutable var — move to model struct\n"
fi

# Rule 1: No sync.Mutex in UI code
if grep -nq 'sync\.\(RW\)\?Mutex' "$file"; then
    warnings+="  Rule 2 violation: sync.Mutex in UI code — trust the tea event loop\n"
fi

# Rule 3: No blocking calls in Update or View functions
# Extract function bodies of Update and View, check for blocking patterns
if awk '/^func.*\) Update\(/,/^}/' "$file" | grep -qE '\b(backend|adapter)\.\w+\('; then
    warnings+="  Rule 3 violation: Blocking call in Update — move to tea.Cmd\n"
fi

if awk '/^func.*\) View\(/,/^}/' "$file" | grep -qE '\b(backend|adapter|os\.Open|os\.Read|http\.)\w*\('; then
    warnings+="  Rule 3 violation: Blocking call in View — move to tea.Cmd\n"
fi

# Rule 2/3: No goroutine launches in Update
if awk '/^func.*\) Update\(/,/^}/' "$file" | grep -qE '\bgo\s+(func\b|\w+\()'; then
    warnings+="  Rule 2/3 violation: Goroutine in Update — use tea.Cmd instead\n"
fi

if [[ -n "$warnings" ]]; then
    echo "ELM ARCHITECTURE: $(basename "$file") has violations:" >&2
    echo -e "$warnings" >&2
    echo "  See ~/.claude/docs/elm-conventions.md for correct patterns." >&2
fi

exit 0
```

- [ ] **Step 2: Make the hook executable**

Run: `chmod +x .claude/hooks/elm-architecture-lint.sh`

- [ ] **Step 3: Verify the script is syntactically valid**

Run: `bash -n .claude/hooks/elm-architecture-lint.sh && echo "OK"`
Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add .claude/hooks/elm-architecture-lint.sh
git commit -m "Add Elm architecture lint hook for poplar UI files

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Register the hook in settings.json

**Files:**
- Modify: `.claude/settings.json`

- [ ] **Step 1: Add the hook to the PostToolUse Edit matcher**

In `.claude/settings.json`, add the elm-architecture-lint hook to the existing PostToolUse Edit hooks array. The entry goes after the existing hooks:

```json
{
  "type": "command",
  "command": ".claude/hooks/elm-architecture-lint.sh"
}
```

The Edit matcher's hooks array should now be:

```json
{
  "matcher": "Edit",
  "hooks": [
    {
      "type": "command",
      "command": ".claude/hooks/claude-md-size.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/filter-live-verify.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/dotfiles-sync.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/elm-architecture-lint.sh"
    }
  ]
}
```

- [ ] **Step 2: Add the hook to the PostToolUse Write matcher**

Same addition to the Write matcher's hooks array:

```json
{
  "matcher": "Write",
  "hooks": [
    {
      "type": "command",
      "command": ".claude/hooks/claude-md-size.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/filter-live-verify.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/dotfiles-sync.sh"
    },
    {
      "type": "command",
      "command": ".claude/hooks/elm-architecture-lint.sh"
    }
  ]
}
```

- [ ] **Step 3: Verify the JSON is valid**

Run: `jq . .claude/settings.json > /dev/null && echo "OK"`
Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add .claude/settings.json
git commit -m "Register Elm architecture lint hook in settings.json

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Verify the hook end-to-end

**Files:**
- Create (temporary): `/tmp/test-elm-hook.go`

- [ ] **Step 1: Create a test file with intentional violations**

Create `/tmp/test-elm-hook.go` with:

```go
package ui

import "sync"

var globalFolders []string

type Bad struct {
	mu sync.Mutex
}

func (m Bad) Update(msg any) (Bad, any) {
	go func() {}()
	_ = backend.ListFolders()
	return m, nil
}
```

- [ ] **Step 2: Run the hook against the test file**

The hook reads the file at the path from the JSON input. Copy the test file to the right location temporarily:

```bash
mkdir -p internal/ui
cp /tmp/test-elm-hook.go internal/ui/test_violation.go
echo '{"tool_input":{"file_path":"'"$(pwd)"'/internal/ui/test_violation.go"}}' \
  | bash .claude/hooks/elm-architecture-lint.sh
rm internal/ui/test_violation.go
rmdir internal/ui 2>/dev/null
```

Expected: stderr output with multiple rule violations (package-level var, sync.Mutex, goroutine in Update, blocking call in Update).

- [ ] **Step 3: Test that non-UI files are ignored**

Run:
```bash
echo '{"tool_input":{"file_path":"'"$(pwd)"'/internal/mail/jmap.go"}}' \
  | bash .claude/hooks/elm-architecture-lint.sh 2>&1
```

Expected: No output (hook exits silently for non-UI files).

- [ ] **Step 4: Test that test files are ignored**

Run:
```bash
echo '{"tool_input":{"file_path":"'"$(pwd)"'/internal/ui/sidebar_test.go"}}' \
  | bash .claude/hooks/elm-architecture-lint.sh 2>&1
```

Expected: No output (hook skips test files).

- [ ] **Step 5: Clean up**

Run: `rm -f /tmp/test-elm-hook.go`
