# Poplar Pivot Plan

**Date:** 2026-04-12
**Status:** Planned, awaiting execution
**Scope:** Pivot the `beautiful-aerc` repository from a four-binary
aerc-supporting toolkit into a single-binary poplar project, scrub
aerc-the-client references, and refactor the docs/Claude
infrastructure to 2026 best practices.

## Context

`beautiful-aerc` was started as a themeable filter pipeline and
supporting tools for the aerc email client. Poplar — a bubbletea
email client — grew inside the same repository to reuse the filter
and theme code. As of Pass 2.5b-3.5, poplar is the primary focus.
The user has decided to abandon the aerc supporting toolkit and
focus purely on poplar.

At the same time, the docs and Claude infrastructure have
accumulated drift across 15+ development passes:

- `CLAUDE.md` auto-loads ~1,349 lines via three `@`-imports.
- `docs/poplar/architecture.md` is 791 lines of append-only ADRs.
- `docs/poplar/STATUS.md` is 246 lines mixing current state with
  forward plans and embedded starter prompts.
- 12 of 13 plan files and 12 of 13 spec files are completed work
  not archived, adding ~18K lines of grep noise.
- Two files named `styling.md` sit at different tree depths with a
  disambiguation block in CLAUDE.md.
- Global conventions are prose-referenced docs rather than skills.

The pivot and the refactor are interleaved in a single execution
because many of the changes (deleting mailrender, trimming CLAUDE.md,
rewriting architecture.md into ADRs) are entangled.

## Goals

1. **Single binary.** Repository produces one Go binary: `poplar`.
2. **Aerc references scrubbed.** No `beautiful-aerc`, `mailrender`,
   `fastmail-cli`, `tidytext` CLI names, `aerc.conf`, `nvim-mail`,
   or similar anywhere in the active tree, docs, or workstation.
   The forked worker code keeps aerc provenance comments — see
   Phase 8 — because that provenance is load-bearing metadata.
3. **Repo renamed to `poplar`** locally and on GitHub. Go module
   becomes `github.com/glw907/poplar`.
4. **Lean docs** aligned to 2026 best practices:
   - CLAUDE.md ≤100 lines with one @-import.
   - `invariants.md` is the single always-loaded doc of binding
     facts, edited in place.
   - ADRs in `docs/poplar/decisions/NNNN-*.md`, on-demand.
   - STATUS.md is current-state-only (~60 lines).
   - Pass-end consolidation is a ritual encoded in the
     `poplar-pass` skill, not a buried checklist.
5. **Global Claude infrastructure** aligned to pivot:
   - `go-conventions` and `elm-conventions` become global skills.
   - `~/.claude/CLAUDE.md` aerc sections removed.
   - Workstation cleanup: old dotfiles stow package removed,
     orphan scripts deleted.

## Non-goals

- **No human-facing docs.** The user will draft a README, contrib
  guide, and user docs at the end of the project. Until then, there
  is no `README.md`, `docs/contributing.md`, or similar.
- **No feature work.** This plan does not advance any poplar pass.
  Pass 2.5b-3.6 resumes after this plan lands.
- **No scrubbing of "aerc" from the forked worker code.** The
  code in `internal/mailworker/` (née `internal/aercfork/`) is
  forked from aerc and that provenance is documented in package
  comments and ADRs. "Remove aerc references" means remove the
  *aerc-the-client* surface, not scrub every mention of the
  upstream origin.

## Decisions (answered before planning)

| # | Question | Answer |
|---|----------|--------|
| 1 | Repo rename | **`poplar`**, local dir + GitHub repo + Go module |
| 2 | `internal/aercfork/` rename | **`internal/mailworker/`**, provenance in code comments and docs |
| 3 | tidytext / fastmail-cli disposition | fastmail-cli **deleted**; tidytext **becomes core poplar compose feature** (library retained, CLI deleted) |
| 4 | Global Claude cleanup scope | **Full cleanup** — all references, stow package, scripts |
| 5 | Corpus | **Keep** |
| 6 | Memories | **Keep** (update two that reference aerc) |

### New pass added to roadmap

The user has also added a tidytext integration pass to the poplar
roadmap. Slot: **Pass 9.5 "Tidytext in compose"**, scheduled after
Pass 9 (Compose + send with Catkin editor). This confirms that
`internal/tidy/` must be kept as a library package through Phase 6
(already planned) and wired into poplar's Catkin compose flow once
the editor exists. Phase 4 must insert Pass 9.5 into the trimmed
`STATUS.md` pass table.

## Investigation summary (Phase 1 output)

### Internal package consumers

| Package | Current Go consumers | Post-pivot disposition |
|---------|----------------------|------------------------|
| `internal/filter/` | `cmd/mailrender/*` (6 files) | **Keep as library** — Pass 2.5b-4 viewer consumer |
| `internal/content/` | `cmd/mailrender/*` (3 files) | **Keep as library** — same |
| `internal/theme/` | `internal/ui/*` (8 files), `internal/content/*` (2), `cmd/poplar/*` (2), `cmd/mailrender/*` (3) | **Keep** — active poplar consumers remain |
| `internal/tidy/` | `cmd/tidytext/*` (2 files) | **Keep as library** — Pass 9 compose consumer |
| `internal/compose/` | `cmd/mailrender/compose.go` | **Delete** — no surviving consumer |
| `internal/jmap/` | `cmd/fastmail-cli/*` (6 files) | **Delete** — distinct from `internal/mailjmap/` |
| `internal/rules/` | `cmd/fastmail-cli/*` (2 files) | **Delete** |
| `internal/header/` | `cmd/fastmail-cli/*` (3 files) | **Delete** — poplar does not consume it |
| `internal/mail/` | `internal/ui`, `internal/config`, `internal/mailjmap`, `cmd/poplar` | Keep |
| `internal/ui/` | `cmd/poplar` | Keep |
| `internal/config/` | `internal/ui`, `internal/mailjmap`, `cmd/poplar` | Keep |
| `internal/mailjmap/` | `internal/mail`, `cmd/poplar` | Keep |
| `internal/aercfork/` | `internal/mailjmap`, `cmd/poplar`, plus internal cross-package imports (109 occurrences / 55 files) | **Keep + rename to `internal/mailworker/`** |

After Phase 6 delete, `internal/filter/`, `internal/content/`, and
`internal/tidy/` will have no Go consumers. They still compile and
their `*_test.go` tests still run — they sit idle until their
respective consumers ship (viewer in 2.5b-4, compose in Pass 9).

### Hooks

| Hook | Disposition | Reason |
|------|-------------|--------|
| `claude-md-size.sh` | Keep | Enforces the 200-line CLAUDE.md limit |
| `make-check-before-commit.sh` | Keep | Pivot-neutral |
| `elm-architecture-lint.sh` | Keep, update reference | Points at `~/.claude/docs/elm-conventions.md`, needs update to skill path |
| `filter-live-verify.sh` | **Delete** | Tells user to verify in aerc; aerc is gone, poplar viewer doesn't exist yet |
| `dotfiles-sync.sh` | **Delete** | Syncs `.config/aerc/` and `.config/nvim-mail/` to the beautiful-aerc stow package; all three are going away |

### Rules

| Rule | Disposition |
|------|-------------|
| `.claude/rules/aerc-config.md` | **Delete** (no aerc config) |
| `.claude/rules/fastmail-cli.md` | **Delete** (CLI deleted) |
| `.claude/rules/tidytext.md` | **Delete** (scope paths going away; `internal/tidy/` is small enough not to need a path-scoped rule) |
| `.claude/rules/poplar-development.md` | **Update** — point at new `poplar-pass` skill and new doc structure |

### e2e tests

`e2e/e2e_test.go` builds and runs `mailrender html` on HTML fixtures
in `e2e/testdata/` (10 files) against golden outputs. **Delete
entirely** in Phase 7. The filter pipeline gets regression testing
via poplar's viewer tests when Pass 2.5b-4 ships. Corpus (1 file)
stays as a future fixture source.

## Target repo state (post-pivot)

```
~/Projects/poplar/                          (renamed from beautiful-aerc)
├── CLAUDE.md                               ~80 lines, 1 @-import
│
├── cmd/
│   └── poplar/                             sole binary
│       ├── main.go
│       ├── root.go
│       ├── config.go
│       ├── config_init.go
│       ├── config_init_test.go
│       └── themes.go
│
├── internal/
│   ├── ui/                                 bubbletea components (active)
│   ├── mail/                               Backend + classifier (active)
│   ├── mailjmap/                           JMAP adapter (active)
│   ├── mailworker/                         forked IMAP+JMAP workers (renamed)
│   │   └── README.md                       NEW — aerc provenance + fork policy
│   ├── config/                             accounts + UI config (active)
│   ├── theme/                              compiled lipgloss themes (active)
│   ├── filter/                             email cleanup pipeline (idle library)
│   ├── content/                            block model + renderer (idle library)
│   └── tidy/                               prose tidying (idle library)
│
├── docs/
│   ├── poplar/
│   │   ├── invariants.md                   NEW, @-imported, ~150 lines
│   │   ├── STATUS.md                       ~60 lines
│   │   ├── system-map.md                   NEW, on-demand
│   │   ├── decisions/                      NEW, ~40 ADRs
│   │   ├── styling.md                      kept
│   │   ├── wireframes.md                   kept
│   │   └── keybindings.md                  kept
│   └── superpowers/
│       ├── archive/
│       │   ├── plans/                      12 archived
│       │   └── specs/                      12 archived
│       ├── plans/
│       │   ├── 2026-04-12-poplar-pivot.md  this file
│       │   └── 2026-04-12-poplar-ui-config.md  active
│       └── specs/
│           └── 2026-04-12-poplar-ui-config-design.md  active
│
├── corpus/                                 1 file, fixture source
│
├── .claude/
│   ├── rules/
│   │   └── poplar-development.md           sole surviving rule
│   ├── skills/
│   │   ├── fix-corpus/                     kept
│   │   └── poplar-pass/                    NEW — pass-end ritual
│   ├── hooks/
│   │   ├── claude-md-size.sh               kept
│   │   ├── make-check-before-commit.sh     kept
│   │   └── elm-architecture-lint.sh        kept, reference updated
│   ├── docs/
│   │   └── tmux-testing.md                 kept
│   ├── settings.json                       kept
│   └── settings.local.json                 kept
│
├── BACKLOG.md                              kept
├── Makefile                                single-binary rewrite
└── go.mod                                  module github.com/glw907/poplar
```

### Deleted entirely

- `cmd/mailrender/`, `cmd/fastmail-cli/`, `cmd/tidytext/`
- `internal/compose/`, `internal/jmap/`, `internal/rules/`,
  `internal/header/`
- `.config/aerc/`, `.config/nvim-mail/`
- `e2e/` (entire directory)
- `docs/contributing.md`, `docs/power-users.md`, `docs/nvim-mail.md`
- `docs/styling.md`, `docs/themes.md` (mailrender-specific)
- `docs/poplar/architecture.md` (split into ADRs)
- `.claude/rules/{aerc-config,fastmail-cli,tidytext}.md`
- `.claude/hooks/{filter-live-verify,dotfiles-sync}.sh`

### Workstation cleanup (Phase 13)

- `~/.claude/CLAUDE.md`: delete `## aerc (Email)` section, delete
  `### fastmail-cli` subsection, trim `## Neovim` to drop
  `nvim-mail` references, update `## Go Development` to reference
  the new global skill.
- `~/.claude/docs/aerc-setup.md` — delete.
- `~/.claude/docs/neovim-setup.md` — trim the nvim-mail section,
  keep nvim-journal content.
- `~/.claude/docs/go-conventions.md` — delete (content → skill).
- `~/.claude/docs/elm-conventions.md` — delete (content → skill).
- `~/.dotfiles/beautiful-aerc/` — `stow -D beautiful-aerc`, then
  `rm -rf`.
- `~/.local/bin/` — delete: `mail`, `aerc-save-email`, `nvim-mail`,
  `mailrender`, `fastmail-cli`, `tidytext`, `beautiful-aerc` (if
  aerc-specific).
- `~/.claude/projects/-home-glw907-Projects-beautiful-aerc/` —
  rename the parent dir to match new project path after Phase 12.

## Phases

Phase order is chosen so every intermediate state builds and the
destructive steps happen after the planning and staging are in
place. Each phase ends green (`go build ./...`) or is a no-op for
the Go toolchain.

### Phase 3: Create global go-conventions + elm-conventions skills

Non-destructive, orthogonal to the pivot.

1. Create `~/.claude/skills/go-conventions/SKILL.md` with
   `description` frontmatter (under 250 chars) and body = current
   `~/.claude/docs/go-conventions.md` content.
2. Create `~/.claude/skills/elm-conventions/SKILL.md` with
   `description` frontmatter scoped to bubbletea / `internal/ui/`
   and body = current `~/.claude/docs/elm-conventions.md` content.
3. Do **not** delete the old docs yet — they're deleted in
   Phase 13 after the `elm-architecture-lint.sh` reference is
   updated.

### Phase 4: Build new poplar doc structure

Non-destructive. Old `architecture.md` stays until Phase 7.

1. Create `docs/poplar/decisions/` directory.
2. Split every `###` decision heading in `docs/poplar/architecture.md`
   into a numbered file `NNNN-slug.md`:
   ```
   ---
   title: <heading>
   status: accepted
   date: <from architecture.md>
   ---
   ## Context
   <rationale lead-in>
   ## Decision
   <the decision text>
   ## Consequences
   <follow-on effects, implicit supersession notes>
   ```
   Number in chronological order (0001 = monorepo, 0002 = clean
   fork, etc.). Target ~40 ADRs.
3. Write `docs/poplar/invariants.md`:
   - Binding-facts-only. No rationale, no dates, no pass numbers.
   - Sections: **Architecture**, **UX**, **Build / verification**,
     **Decision index**.
   - Target ≤150 lines.
   - Every binding fact is extracted from an ADR; the decision
     index points at the ADR.
4. Write `docs/poplar/system-map.md`:
   - Package/layer overview table from architecture.md's header.
   - Short inventory of what lives where.
   - Loaded on-demand, not @-imported.
5. Create `.claude/skills/poplar-pass/SKILL.md`:
   ```
   ---
   name: poplar-pass
   description: Invoke at the start or end of a poplar development
   pass. Covers the pass-end consolidation ritual (ADR writing,
   invariants update, archival) and the starter-prompt format.
   Trigger on "continue development", "next pass", "finish pass",
   "ship pass", or explicit invocation.
   ---
   ```
   Body:
   - **Pass-end consolidation ritual** (the anti-drift core):
     1. `/simplify`.
     2. For each design decision made this pass, write
        `docs/poplar/decisions/NNNN-*.md`.
     3. If a decision supersedes a prior ADR: mark the old ADR
        `status: superseded by NNNN` with a link.
     4. **Edit `invariants.md` in place** — add new binding
        facts, remove or rewrite facts this pass changed. Never
        append without considering what to remove.
     5. Update `STATUS.md`: mark pass done in table, replace the
        current starter prompt with the next one.
     6. Move this pass's plan + spec from `docs/superpowers/plans/`
        and `docs/superpowers/specs/` to
        `docs/superpowers/archive/{plans,specs}/`.
     7. `make check` → commit → push → `make install`.
   - **ADR template** (context / decision / consequences /
     supersedes).
   - **Starter-prompt format** (scope, settled vs open, approach).
6. Trim `docs/poplar/STATUS.md`:
   - Keep the pass table.
   - **Insert Pass 9.5 "Tidytext in compose"** between Pass 9 and
     Pass 10. Scheduled after Catkin editor exists; wires
     `internal/tidy/` into poplar's compose flow.
   - Keep ONE current starter prompt (2.5b-3.6). Delete the
     2.5b-3.7 starter; it gets written at the end of 2.5b-3.6.
   - Delete the "Plans" link list entirely — archived files don't
     need a hand-maintained index.
   - Delete the "Pass-end checklist" (now in the skill).
   - Target ≤60 lines.

Intentionally left for later phases:
- Delete `docs/poplar/architecture.md` — Phase 7.
- Rewrite `CLAUDE.md` to point at `invariants.md` — Phase 10.

### Phase 5: Archive completed plans and specs

Non-destructive.

1. `mkdir -p docs/superpowers/archive/plans docs/superpowers/archive/specs`.
2. `git mv` 12 completed plan files to `archive/plans/`.
3. `git mv` 12 completed spec files to `archive/specs/`.
4. Leave the 1 active pair (`2026-04-12-poplar-ui-config*`) and
   this plan doc in place.

### Phase 6: Delete aerc CLIs and their internals — DESTRUCTIVE

First one-way door. After this phase, reversing means git revert.

1. `git rm -r cmd/mailrender cmd/fastmail-cli cmd/tidytext`.
2. `git rm -r internal/compose internal/jmap internal/rules internal/header`.
3. `go build ./...` — confirm green. If any package now has
   orphan imports, investigate. Per Phase 1 findings, none
   expected.
4. `go test ./...` — confirm green. `internal/filter`,
   `internal/content`, `internal/tidy` tests should pass
   standalone (their `*_test.go` files don't import the deleted
   CLIs).

### Phase 7: Delete aerc configs, human docs, and old architecture.md — DESTRUCTIVE

1. `git rm -r .config/aerc .config/nvim-mail`.
2. `git rm -r e2e`.
3. `git rm docs/contributing.md docs/power-users.md docs/nvim-mail.md docs/styling.md docs/themes.md`.
4. `git rm docs/poplar/architecture.md` — its content now lives
   in `docs/poplar/decisions/` and `docs/poplar/invariants.md`.
5. `git rm .claude/rules/aerc-config.md .claude/rules/fastmail-cli.md .claude/rules/tidytext.md`.
6. `git rm .claude/hooks/filter-live-verify.sh .claude/hooks/dotfiles-sync.sh`.
7. `go build ./...` — confirm green (the e2e package deletion is
   Go-visible).

### Phase 8: Rename aercfork to mailworker — DESTRUCTIVE

Preserve provenance in code and docs.

1. `git mv internal/aercfork internal/mailworker`.
2. Global find/replace across `.go` files:
   `github.com/glw907/beautiful-aerc/internal/aercfork` →
   `github.com/glw907/beautiful-aerc/internal/mailworker`
   (109 occurrences in 55 files — Go module rename in Phase 9
   will rewrite the prefix a second time).
3. Write `internal/mailworker/README.md`:
   ```
   # mailworker

   IMAP and JMAP workers forked from aerc
   (git.sr.ht/~rjarry/aerc) on 2026-04-09.

   ## Why a fork

   Aerc does not maintain a stable library API — internal
   packages change without notice. A clean fork with cherry-pick
   upstream tracking is more stable than chasing `go get -u`
   breakage. See `docs/poplar/decisions/0002-clean-fork-over-direct-import.md`.

   ## Upstream tracking

   When cherry-picking upstream protocol fixes, preserve the
   aerc commit hash in the commit message for traceability.
   ```
4. Add top-of-file provenance comments to every package in
   `internal/mailworker/`:
   ```go
   // Package <name> is forked from aerc
   // (git.sr.ht/~rjarry/aerc) on 2026-04-09.
   ```
5. Update ADRs that mention `aercfork` to reference
   `mailworker` while keeping the historical narrative intact.
6. Update `docs/poplar/system-map.md` to reflect rename.
7. Update `docs/poplar/invariants.md` if it references the old
   path.
8. `go build ./...` green.

### Phase 9: Simplify Makefile and rename Go module — DESTRUCTIVE

1. Rewrite `Makefile`:
   ```makefile
   BINARY := poplar

   build:
   	go build -o $(BINARY) ./cmd/poplar

   test:
   	go test ./...

   vet:
   	go vet ./...

   lint:
   	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

   install:
   	GOBIN=$(HOME)/.local/bin go install ./cmd/poplar

   check: vet test

   clean:
   	rm -f $(BINARY)

   .PHONY: build test vet lint install check clean
   ```
2. `go mod edit -module github.com/glw907/poplar`.
3. Global find/replace:
   `github.com/glw907/beautiful-aerc` → `github.com/glw907/poplar`
   across all `.go` files.
4. `go mod tidy` — reconcile.
5. `go build ./...` green. `make check` green.

### Phase 10: Rewrite project CLAUDE.md

1. Replace `CLAUDE.md` with ~80 lines:
   - Project identity (one paragraph).
   - `@docs/poplar/invariants.md` (only @-import).
   - Conventions (on-demand reads): point at `go-conventions`
     and `elm-conventions` global skills, poplar styling,
     decisions archive, system-map.
   - Poplar development: trigger phrase → invoke `poplar-pass`
     skill.
   - Build: `make build | test | check | install`.
   - Live testing reference: `.claude/docs/tmux-testing.md`.
   - Backlog pointer.
2. Update `.claude/rules/poplar-development.md` to reference the
   new skill and doc paths.
3. Update `.claude/hooks/elm-architecture-lint.sh` — change the
   error message reference from `~/.claude/docs/elm-conventions.md`
   to something like "invoke the elm-conventions skill".

### Phase 11: Stage and commit

Commit stages (each a separate commit, `make check` green before
each):

1. **(a) global skills** — new `~/.claude/skills/{go,elm}-conventions/`.
   Commits in `~/.dotfiles/claude/` if the skills are stowed
   there, otherwise directly in `~/.claude/`.
2. **(b) new poplar docs + ADRs + skill** — `docs/poplar/{invariants,system-map}.md`,
   `docs/poplar/decisions/*`, `.claude/skills/poplar-pass/`,
   trimmed `STATUS.md`. Old `architecture.md` still present.
3. **(c) archive** — `docs/superpowers/archive/{plans,specs}/`
   populated.
4. **(d) delete CLIs and internals** — Phase 6 deletions.
5. **(e) delete configs, human docs, old architecture.md** —
   Phase 7 deletions.
6. **(f) mailworker rename** — Phase 8 rename + provenance.
7. **(g) Makefile + Go module rename** — Phase 9.
8. **(h) CLAUDE.md rewrite + rule/hook updates** — Phase 10.

Push after each commit (or batched, at user's discretion).

### Phase 12: Rename local dir and GitHub repo — DESTRUCTIVE

1. `stow -D beautiful-aerc` (from `~/.dotfiles/`) — remove the
   aerc stow symlinks from `~/.config/` and `~/.local/bin/`.
   Verify with `ls -la ~/.config/aerc ~/.config/nvim-mail`
   (should return "no such file or directory").
2. `mv ~/Projects/beautiful-aerc ~/Projects/poplar`.
3. `cd ~/Projects/poplar && git remote -v` — still points at old
   repo URL. Fix after step 4.
4. `gh repo rename poplar` (from inside the renamed local dir,
   which talks to the old GitHub name) — renames the GitHub repo
   and updates the local remote. Verify with `gh repo view`.
5. Rename the auto-memory directory:
   `mv ~/.claude/projects/-home-glw907-Projects-beautiful-aerc ~/.claude/projects/-home-glw907-Projects-poplar`.
6. Spot-check any hardcoded paths in hooks, scripts, or dotfiles
   referencing the old path.

### Phase 13: Global workstation cleanup — DESTRUCTIVE

1. Edit `~/.claude/CLAUDE.md`:
   - Delete `## aerc (Email)` section (including
     `### fastmail-cli` subsection).
   - Trim `## Neovim` to remove `nvim-mail` line and reference.
   - Update `## Go Development` to reference the `go-conventions`
     skill instead of the doc path.
2. `rm ~/.claude/docs/aerc-setup.md`.
3. Edit `~/.claude/docs/neovim-setup.md`:
   - Delete `## nvim-mail Profile` section and all downstream
     nvim-mail references.
   - Keep `## nvim-journal Profile` and everything unrelated to
     nvim-mail.
   - Save as-is or rename to `neovim-journal-setup.md` — decide
     during execution based on what's left.
4. `rm ~/.claude/docs/go-conventions.md` (content now in skill).
5. `rm ~/.claude/docs/elm-conventions.md` (content now in skill).
6. `rm -rf ~/.dotfiles/beautiful-aerc/`.
7. Delete orphan scripts from `~/.local/bin/`: `mail`,
   `aerc-save-email`, `nvim-mail`, `mailrender`, `fastmail-cli`,
   `tidytext`. Check `beautiful-aerc` script first — if it's a
   launcher for aerc, delete; if unrelated, keep.
8. Commit dotfiles changes.

### Phase 14: Update aerc-referencing memories

1. Edit `reference_jmap_email_access.md`:
   - "filter debugging" → "poplar viewer debugging"
   - Everything else unchanged (JMAP endpoint, account ID, query
     pattern are still accurate).
2. Edit `project_nvim_companion.md`:
   - Update the "Why" line — drop the nvim-mail reference; the
     user's rationale is still "neovim is a primary tool for
     email-adjacent work," but the specific previous vehicle
     (nvim-mail) is gone.
   - "How to apply" unchanged.
3. Verify `MEMORY.md` index still accurate.

### Phase 15: Final verification

1. Start a fresh Claude Code session in `~/Projects/poplar`.
2. Confirm auto-load context is ~250 lines (CLAUDE.md ~80 +
   invariants.md ~150).
3. `go build ./...` green.
4. `make check` green.
5. `make install` green, `poplar` binary present in
   `~/.local/bin/`.
6. Launch poplar, verify mock backend still renders sidebar +
   message list.
7. Grep for residual references:
   - `grep -r "beautiful-aerc" .` — should return only the
     archived plans/specs (historical) and provenance comments
     in `mailworker/`.
   - `grep -r "mailrender" .` — same.
   - `grep -r "fastmail-cli" .` — same, if any.
   - `grep -r "aerc" .` — should return provenance comments and
     archived plans only.
8. Fix any stragglers.

## Risks and rollback

| Risk | Mitigation |
|------|------------|
| Hidden cross-package import from a delete target | Phase 6 runs `go build ./...` after deletion. Go toolchain catches any missed import at compile time. |
| `filter-live-verify.sh` deletion leaves a regression hole | Known — filter pipeline has unit tests in `internal/filter/*_test.go` and `internal/content/*_test.go`. Live verification returns when poplar's viewer ships in Pass 2.5b-4, at which point a new `poplar-viewer-verify` hook may be added. |
| GitHub repo rename breaks external links / old clones | GitHub maintains a redirect after rename. Local clones of old name still work read-only. Flag in commit message before rename. |
| Auto-memory path rename misses a file | Memory dir is renamed as a unit (`mv`), so all files move atomically. Claude Code reads from the new path on next session start. |
| Stow undeploy missed symlinks | `stow -D` handles it. Verify with `ls -la ~/.config/aerc` post-undeploy. |
| Global CLAUDE.md cleanup too aggressive | Edits are staged in dotfiles git; revert is `git checkout` of the file. |
| Module rename breaks dependent tools | No known external dependents of `beautiful-aerc` module. `go.sum` regenerates cleanly with `go mod tidy`. |

**Rollback points.** Every phase before Phase 12 is reversible via
git (`git revert` or `git reset`). Phase 12 (local + GitHub
rename) is the point of no effective return — reversing requires
renaming GitHub back (which user can do via `gh repo rename`) and
moving the local directory back. Phase 13 (workstation cleanup) is
reversible via dotfiles git revert.

The phase 12 decision is visible in commit history, so rollback is
always possible, just annoying. The plan's structure ensures the
user can stop at any phase boundary and the repo is still in a
working state.

## Verification checklist

At the end of Phase 15, all of the following must be true:

- [ ] `go build ./...` green
- [ ] `make check` green
- [ ] `poplar` binary runs, renders sidebar and message list
- [ ] `CLAUDE.md` ≤100 lines
- [ ] `docs/poplar/invariants.md` ≤150 lines
- [ ] `docs/poplar/STATUS.md` ≤60 lines
- [ ] `docs/poplar/architecture.md` no longer exists
- [ ] `docs/poplar/decisions/` has ~40 ADR files
- [ ] Auto-loaded context on fresh session ≤300 lines
- [ ] No `cmd/mailrender`, `cmd/fastmail-cli`, `cmd/tidytext`
- [ ] No `internal/{compose,jmap,rules,header,aercfork}`
- [ ] No `.config/aerc`, `.config/nvim-mail`, `e2e/`
- [ ] Module is `github.com/glw907/poplar`
- [ ] Local dir is `~/Projects/poplar/`
- [ ] GitHub repo is `glw907/poplar`
- [ ] `~/.dotfiles/beautiful-aerc/` gone
- [ ] `~/.claude/docs/{aerc-setup,go-conventions,elm-conventions}.md` gone
- [ ] `~/.claude/docs/neovim-setup.md` has no nvim-mail content
- [ ] `~/.claude/CLAUDE.md` has no aerc section
- [ ] `~/.claude/skills/{go-conventions,elm-conventions}/SKILL.md` present
- [ ] `~/.claude/skills/poplar-pass/SKILL.md` present (project-local)
- [ ] `~/.claude/projects/-home-glw907-Projects-poplar/memory/` has all memories
- [ ] `grep -r aerc` returns only provenance comments and archived plans

## Dependencies between phases

```
3 (global skills) ──────────────────────┐
                                        │
4 (new docs) ─── 5 (archive) ──┐        │
                                │        │
                                ├─► 6 (delete CLIs)
                                │        │
                                │        ├─► 7 (delete configs + human docs)
                                │                │
                                │                ├─► 8 (mailworker rename)
                                │                │
                                │                │   ├─► 9 (Makefile + module rename)
                                │                │   │
                                │                │   │   ├─► 10 (CLAUDE.md rewrite)
                                │                │   │   │
                                │                │   │   ├─► 11 (commit stages)
                                │                │   │   │
                                │                │   │   ├─► 12 (rename local + github)
                                │                │   │   │
                                │                │   │   ├─► 13 (workstation cleanup)
                                │                │   │   │
                                │                │   │   ├─► 14 (memory updates)
                                │                │   │   │
                                │                │   │   └─► 15 (verification)
```

Phase 3 is independent — it can run in parallel or be done before
anything else. All other phases are sequential.
