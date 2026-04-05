# Save & Fix Corpus Design

## Goal

Provide a low-friction way to flag emails with rendering issues from
within aerc, accumulate them as a corpus, and batch-fix the pipeline
with a Claude skill. Also restructure the project's Claude
infrastructure to be self-contained per 2026 best practices.

## Components

### 1. `beautiful-aerc save` subcommand

A new cobra subcommand that reads an email part from stdin, detects
whether it is HTML or plain text, and writes it to the corpus directory
with a timestamped filename.

**Content sniffing:** Check if the input contains any of `<html`,
`<head`, `<body`, `<!doctype`, or `<table` (case-insensitive). If yes,
save as `.html`; otherwise `.txt`.

**Filename format:** `YYYYMMDD-HHMMSS.html` or
`YYYYMMDD-HHMMSS.txt`. Timestamps use local time. If a file with that
name already exists (two saves in the same second), append `-2`, `-3`,
etc.

**Corpus directory location:** The save command finds the project
root using the same resolution strategy as other project resources
(palette.sh): check `$AERC_CONFIG`, then relative to the binary,
then `~/.config/aerc/`. From the resolved config directory, the
corpus path is `../../corpus/` (since config is at
`.config/aerc/` within the stow package, two levels up is the
project root). The directory is created automatically if it does
not exist.

**Output:** Print the saved filename to stderr (aerc displays stderr
as a status bar message). Example: `saved corpus/20260404-143022.html`

**Error handling:**
- Empty stdin: return error "no input"
- Write failure: return error with context
- Directory creation failure: return error with context

### 2. Keybinding

Add to `[view]` section in `.config/aerc/binds.conf`:

```ini
b = :pipe -m beautiful-aerc save<Enter>
```

The `-m` flag pipes the raw MIME part. This works for both `text/html`
and `text/plain` parts -- whichever the viewer is currently displaying.

### 3. Corpus directory

- Location: `corpus/` at project root
- Added to `.gitignore` (contains personal email content)
- No metadata files, no index -- just raw email parts with timestamps
- Claude processes them by iterating the directory

### 4. `fix-corpus` Claude skill

A project-level skill at `.claude/skills/fix-corpus` that drives the
batch fix workflow when the user has accumulated enough flagged emails.

**Invocation:** `/fix-corpus`

**Workflow:**

1. **Scan** -- list all files in `corpus/`, group by type (html/txt)
2. **Preview** -- for each corpus email, render it using aerc's actual
   viewer via tmux:
   - Start a tmux session sized to typical terminal width
   - Pipe the corpus file through `beautiful-aerc html` (or `plain`)
   - Display the rendered output so the user can see the problem
   - Ask: "What's wrong with this one?" if the issue is not obvious
3. **Triage** -- after reviewing all emails, group issues by root cause
   pattern. Look for commonality before making fixes (the holistic
   approach produces better code than fixing one email at a time)
4. **Fix** -- make pipeline changes in `internal/filter/` to address
   the identified patterns. Add test cases for each fix.
5. **Verify** -- re-render all corpus emails, confirm fixes work,
   check for regressions against existing emails
6. **Quality gates** -- `make check`, go-review skill, simplify skill
7. **Ship** -- commit, push, `make install`
8. **Cleanup** -- ask the user:
   - Which fixed emails should become e2e test fixtures (copy to
     `e2e/testdata/` with golden files)?
   - Remove fixed emails from `corpus/`?

**tmux preview pattern:**

```bash
tmux kill-session -t corpus-review 2>/dev/null
tmux new-session -d -s corpus-review -x 80 -y 40
cat corpus/20260404-143022.html | beautiful-aerc html \
  | tmux load-buffer - && tmux paste-buffer -t corpus-review
tmux capture-pane -t corpus-review -p
```

### 5. Project Claude infrastructure

Restructure Claude configuration so the project is self-contained.
Future conversations in this project directory get full context without
depending on global memories or docs.

**Directory structure:**

```
.claude/
  settings.json              # project-level settings
  memory/
    MEMORY.md                # index of project memories
    pipeline_architecture.md # current Go pipeline: stages, ordering, what each does
    problem_senders.md       # sender patterns that stress the pipeline
    edge_case_workflow.md    # feedback: fix-and-ship autonomously
    debug_methodology.md     # feedback: fix cause not symptoms
  skills/
    fix-corpus               # batch fix workflow (described above)
  docs/
    tmux-testing.md          # tmux patterns for previewing filter output in aerc
```

**Memory migration:**

| Global memory | Project memory | Action |
|---------------|---------------|--------|
| `project_aerc_html_pipeline.md` | `pipeline_architecture.md` | Rewrite: current Go pipeline, not stale perl description |
| `feedback_email_edge_cases.md` | `edge_case_workflow.md` | Copy, update to reference corpus workflow |
| `feedback_fix_cause_not_symptoms.md` | `debug_methodology.md` | Copy as-is (general but highly relevant) |
| (from aerc-setup.md lines 479-491) | `problem_senders.md` | Extract problem sender patterns list |

**CLAUDE.md updates:**

The existing project `CLAUDE.md` is already good. Add:
- Reference to `.claude/docs/tmux-testing.md` for filter verification
- Mention of `corpus/` directory and its purpose
- Reference to `fix-corpus` skill

**tmux-testing.md:**

Extract the filter-relevant subset of the global TUI testing patterns
into a project-level doc. Focus on:
- Rendering a file through the filter and capturing output
- Displaying rendered email in a tmux pane for visual review
- Comparing rendered output against expected results

This is a subset of the global doc, tailored to this project's needs.

**settings.json:**

Minimal project settings. The global `settings.json` and
`settings.local.json` handle permissions and hooks. The project
settings only need to declare project-specific preferences if any
exist. Start empty or with just the project name -- do not duplicate
global settings.

**Global cleanup:**

After migration, clean up global Claude infrastructure:
- `project_aerc_html_pipeline.md`: remove from global memory (now
  lives in project)
- `feedback_email_edge_cases.md`: keep globally (applies to workflow
  pattern, not just this project)
- `feedback_fix_cause_not_symptoms.md`: keep globally (general feedback)
- Global `CLAUDE.md`: remove the "aerc (Email)" section entirely.
  That content now lives in the project CLAUDE.md and project memories.
  The global CLAUDE.md returns to its role as pure
  systems-management configuration (machine environment, dotfiles,
  sysadmin preferences, etc.).

## What this does NOT include

- Automated email monitoring or scheduled corpus collection
- Integration with aerc's notification system
- Any changes to the HTML or plain text filter pipeline itself
- Migration of global Go conventions doc (stays global, shared across
  projects)
- Migration of global skills like `ship-go` or `go-review` (stay
  global, shared across projects)

## Testing

- **`save` subcommand unit tests:** content sniffing, file creation,
  collision handling, empty input error
- **`save` e2e test:** pipe an HTML fixture through the save
  subcommand, verify file appears with correct extension and content
- **Keybinding:** manual verification in aerc (pipe a viewed email,
  check corpus directory)
- **`fix-corpus` skill:** manual invocation after accumulating a few
  test emails
