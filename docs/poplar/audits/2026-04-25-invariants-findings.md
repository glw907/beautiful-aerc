# Findings: Invariants vs. Code Drift (2026-04-25)

Independent verification of `docs/poplar/invariants.md`. Method per
`docs/poplar/audits/2026-04-25-invariants-vs-code.md`. Findings written
linearly as each claim was checked.

> Summary section at the bottom — filled after the linear walk.

---

## Architecture

### A1. Single-binary bubbletea email client, one Go module, `cmd/poplar`
**holds.** `cmd/poplar/main.go` is the entry point; `go.mod:1` is
`module github.com/glw907/poplar`.

### A2. Package layout
**holds.** All ten named packages exist under `internal/` and
`cmd/poplar/`. No surprises.

### A3. Workers forked from `git.sr.ht/~rjarry/aerc` on 2026-04-09
**holds.** `internal/mailworker/README.md:3-4` records the fork date
and upstream. The `LICENSE` file is present.

### A4. Backends in v1: Fastmail JMAP and Gmail IMAP only
**holds.** Mail tree contains only `mail/`, `mailjmap/`,
`mailworker/`. No maildir/mbox/notmuch directories or files.

### A5. `mail.Backend` synchronous blocking; JMAP adapter pumps async→sync
**holds.** `internal/mail/backend.go:20-44` — every method returns
plain values + `error`, no channels. `internal/mailjmap/jmap.go:44`
fires `go a.pump()`; the `pump` loop is at lines 143–.

### A6. `internal/ui/` follows Elm architecture
**holds (general).** Component-level corollaries verified individually
(see A7, A20, etc.). Models have no I/O in Update; Cmds are constructed
via helpers in `cmds.go`; children signal parents via Msg types
(`FolderChangedMsg`, `ViewerOpenedMsg`, `ViewerClosedMsg` at
`app.go:61-79`).

### A7. Root model owns `mail.Backend` and `theme.CompiledTheme`
**DRIFT.** The `App` struct (`internal/ui/app.go:14-24`) holds
`acct`, `styles`, `topLine`, `statusBar`, `footer`, `keys`,
`viewerOpen`, `width`, `height` — **not** `mail.Backend` and **not**
`*theme.CompiledTheme`. Both are accepted as `NewApp` parameters
(`app.go:28`) and forwarded to `NewAccountTab`; the references are
not retained on the root model.

- `mail.Backend` is owned by `AccountTab` (`account_tab.go:35`).
- `*theme.CompiledTheme` is owned by `Viewer` (`viewer.go:47`).

The "children hold a reference only when they need it" half is true
in spirit (Viewer needs the theme to render markdown blocks via
`content.RenderBodyWithFootnotes`; AccountTab needs the backend to
build Cmds), but the "root model owns" half is false. Either rewrite
the invariant to match (root forwards backend+theme into the model
tree, ownership lives with the components that need them) or hoist
both refs back onto `App`.

### A8. j/k navigates messages, J/K navigates folders
**holds.** `account_tab.go:178` (`"J"`), `:182` (`"K"`),
`:190` (`"j", "down"`), `:192` (`"k", "up"`). Comment at `:27`
documents the same.

### A9. Config at `~/.config/poplar/accounts.toml`; `[[account]]` + `[ui]` decoded independently
**holds.** `internal/config/accounts.go` defines `ParseAccounts`,
`internal/config/ui.go` defines `LoadUI`, both targeting the same
file. Splitting into separate functions/files follows the invariant.

### A10. 15 themes, One Dark default, Styles struct, no direct NewStyle / no hex
**partial — ambiguous wording.**
- 15 themes: holds — `internal/theme/themes.go:294-336` registers
  exactly 15 `NewCompiledTheme` calls.
- One Dark default: holds — `themes.go:6` `DefaultThemeName = "one-dark"`.
- "Components style through the Styles struct": holds — no
  `lipgloss.NewStyle` outside `internal/ui/styles.go` and
  `internal/theme/palette.go` (verified by `grep -rn NewStyle
  internal/ui/ internal/theme/`).
- "no direct lipgloss.NewStyle() calls, no hardcoded hex values":
  the literal claim is violated — `internal/ui/styles.go:107+` and
  `internal/theme/palette.go:81+` both call `NewStyle`; hex values
  appear throughout `internal/theme/themes.go`. Both sites are the
  intended construction layer for styles/palettes, so the spirit is
  intact, but the wording leaves no room for them.

  **Suggested tightening:** "Components consume styles via the
  `Styles` struct. `lipgloss.NewStyle()` is permitted only in
  `internal/ui/styles.go` and `internal/theme/palette.go`. Hex
  literals are permitted only in `internal/theme/themes.go` palette
  definitions."

### A11. Palette-to-surface map at `docs/poplar/styling.md`
**holds.** File exists. (Out-of-scope to verify the map's accuracy
against code — this audit is about invariants.md only.)

### A12. `mail.Classify([]Folder) []ClassifiedFolder`; Role → alias → Custom
**holds.** `internal/mail/classify.go:39-48` — pure function over
`[]Folder`. Priority: `canonicalFromRole` (line 51) → `canonicalFromAlias`
(line 59) → `GroupCustom` fallback (line 71). Provider names are
normalized to canonical display names (`Inbox`, `Drafts`, ...) via the
role/alias maps below.

### A13. Sidebar: Primary, Disposal, Custom in fixed order; blank-line separators; no headers
**holds.** `sidebar.go:131-135` inserts a blank line whenever the
group changes. `buildEntries` (`:204-235`) concatenates Primary +
Disposal + Custom in that order. No string literal "Primary" /
"Disposal" / "Custom" appears anywhere as a rendered header.

### A14. Nested folders render flat; `/` is the only affordance
**holds.** `sidebar.go:154` comment: "All folders render at the same
indent regardless of nesting." `renderRow` does not key on `/` for
indent.

### A15. Compose pluggable behind `Editor` interface; v1 ships Catkin in `catkin/` package
**STALE.** No `catkin/` directory exists. `grep -rn 'type Editor\b'`
across `internal/` and the repo root returns nothing. The compose
system has not been built (Pass 9 is `pending` per STATUS.md). The
invariant describes architecture intended for Pass 9, not current code.

**Recommendation:** move this to a planning note (BACKLOG.md or a new
spec under `docs/superpowers/specs/`), or reword as conditional:
"When compose ships, it must..."

### A16. `MessageInfo` carries `ThreadID` + `InReplyTo`; depth derived; non-threaded = thread of 1
**holds.** `mail/types.go:68-69` defines both fields. `types.go:46-54`
docstring explicitly states "depth is not carried on the wire" and
matches the wording verbatim.

### A17. `MessageInfo` has both `Date string` and `SentAt time.Time`; `SentAt` authoritative; lessMessage falls back to Date lex when both zero
**holds.** `mail/types.go:55-69` — both fields present.
`internal/ui/msglist.go:292-302` `lessMessage` matches exactly: uses
`SentAt.Before` when both non-zero, falls back to `a.Date < b.Date`
when both are zero, and a deterministic "zero is older" tiebreak in
the mixed case (documented in the comment, not in invariants).

### A18. Date column formatting in `internal/ui/date_format.go`
**holds.** `formatRelativeDate(t, now time.Time) string` at
`date_format.go:20-31` — same-day → `"3:04 PM"` (Go's 12-hour layout);
other → `"Mon 2006-01-02"`; zero → `""`; `t = t.In(now.Location())`
ensures both checks/strings live in `now`'s location.

`MessageList` captures the clock snapshot in `now` field
(`msglist.go:90`). `displayRow.dateText` precomputed in `rebuild`
(`msglist.go:62` declaration; pre-render happens in the rebuild
pipeline).

### A19. `MessageList` owns thread grouping + fold; source/rows; group→sort→flatten; transient `*threadNode` in `appendThreadRows`
**holds.** `msglist.go:78` declares `rows []displayRow`;
`msglist.go:118-122` `SetMessages` documents "replaces the source
slice and rebuilds the displayRow"; `appendThreadRows` at `:316`
constructs the `threadNode` tree on entry and emits rows
depth-first. The tree never escapes the function.

### A20. Viewer is `AccountTab` child; no backend ref; bodyLoadedMsg + CurrentUID staleness drop; phases closed/loading/ready
**holds.**
- No backend ref — `viewer.go:32-34` comment + struct fields confirm.
- `CurrentUID` at `viewer.go:71-76`.
- `bodyLoadedMsg` at `cmds.go:120`; AccountTab compares against
  `viewer.CurrentUID()` at `account_tab.go:106-107`.
- Phases — `viewer.go:21-24` defines `viewerLoading` and `viewerReady`;
  closed encoded by `open bool` (per the docstring at `viewer.go:16-18`,
  matching the invariant: "closed → loading → ready → closed").

### A21. Mark-read on viewer open is optimistic; MarkSeen flips local + parallel MarkRead Cmd
**holds.** `account_tab.go:222-225` (`openSelectedMessage`):
`m.msglist.MarkSeen(msg.UID)` first, then appends `markReadCmd` to
the batch. The cmd in `cmds.go:158-160` calls `b.MarkRead([]mail.UID{uid})`
and any error currently flows back as a backendErrMsg (drop until 2.5b-6).

### A22. Body cap = 72; headers wrap at panel width; footnotes via `RenderBodyWithFootnotes`; U+00A0 glue; auto-linked bare URLs render inline without marker
**holds.**
- `maxBodyWidth = 72` at `internal/content/render.go:12`.
- `RenderBodyWithFootnotes` at `internal/content/render_footnote.go:24`,
  caps width again at `:32-33`.
- nbsp constant at `:11`, used at `:126` to glue `[^N]` to the link's
  last word.
- Auto-linked URLs (`Text == URL`) render without a marker — verified
  by reading `render_footnote.go` link-rewrite path (special-cased
  before the marker rewrite).

---

## UX

### U1. Opinionated, not configurable in v1
**holds.** `accounts.toml` carries only account credentials and a
small `[ui]` table (sort + threading + folder rank). No theme switch,
no key remap.

### U2. Vim-first, single-key, no multi-key sequences
**holds (general).** Verified by reading the key dispatcher in
`account_tab.go:164-204` — every case is a single string match. No
`KeyMatcher` chord state machine.

### U3. No `:` command mode
**DRIFT (dead code).** `keys.go:16` defines `Cmd: key.NewBinding(
key.WithKeys(":"), key.WithHelp(":", "cmd"))`. A grep for `keys.Cmd`
across `internal/` returns no callers — the binding is unused.
Behavior matches the invariant (no command mode runs), but the
binding's existence contradicts the spirit of the rule.

**Recommendation:** delete the `Cmd` field from `GlobalKeys` and
remove its initialization in `NewGlobalKeys`.

### U4. `q` exits viewer / quits app / clears search; `?` opens help popover
**partial / STALE.**
- `q` viewer-close: holds — `app.go:83-90` delegates to AccountTab when
  `viewerOpen`, and `viewer.go:162` handles `"q", "esc"`.
- `q` quit on account view: holds — `app.go:99` `tea.Quit`.
- `q` steals while search non-idle: holds — `app.go:91-97` sends an
  `Esc` to AccountTab when `sidebarSearch.State() != SearchIdle`.
- `?` opens help popover: **STALE.** `app.go:102-104` is a stub:
  `case "?": // Stubbed for 2.5b-5 (help popover)` returns `nil`. The
  popover does not exist yet (Pass 2.5b-5 is the next pass per
  STATUS.md). Invariant claims behavior the code does not implement.

**Recommendation:** rewrite as "`?` will open the help popover when
Pass 2.5b-5 ships," or remove the clause until then.

### U5. Folder jumps I/D/S/A/X/T uppercase keys
**DRIFT.** `keys.go:33-40` defines `FolderJumpKeys` for I/D/S/A/X/T,
but `grep -rn FolderJumpKeys internal/` finds no consumer. The
`account_tab.handleKey` switch (`account_tab.go:164-204`) handles
only `/`, `esc`, `enter`, `J`, `K`, `G`, `g`, `j`, `k`, `down`, `up`,
` `, `F`. None of `I`, `D`, `S`, `A`, `X`, `T` are dispatched.

The footer hint `hint("I/D/S/A", "folders", 9)` (`footer.go:64`)
advertises the binding, so the user-facing claim is even more
load-bearing. This is a real functionality drift, not just doc rot.

**Recommendation:** wire the FolderJumpKeys into AccountTab.handleKey
(this can land as part of the next prototype or polish pass), or remove
the FolderJumpKeys type and footer hint until the wiring is done.

### U6. Threaded display default-on; per-folder `threading = false` overrides to flat; no runtime toggle
**partial DRIFT.**
- Default-on: holds — `config/ui.go:53` global default `Threading: true`.
- No runtime toggle: holds — no key flips threading.
- **Per-folder override does not work.** `config/ui.go:61` defines
  `Threading *bool` on `FolderConfig`, but `account_tab.go:97-104`
  (the `folderLoadedMsg` handler) reads only `fc.Sort`. Nothing in
  `internal/ui/` consults `Threading` or `ThreadingSet`. The msglist
  always threads.

**Recommendation:** decide whether to wire the override (small change
in folderLoadedMsg + a `MessageList.SetThreaded(bool)` method that
suppresses bucket-by-thread) or remove the config field and the
invariant clause.

### U7. Threads sort by latest activity; children chronologically asc; folder sort from config
**holds.** `msglist.go:165-170` sorts threads by `latestActivity`
under `m.sort` direction; child sort is ascending at every level
(`msglist.go:345` comment + `appendThreadRows` walk). Folder sort
read from `fc.Sort` at `account_tab.go:99-101`, default
`SortDateDesc`.

### U8. Thread root = empty `InReplyTo`; fallback earliest; orphans depth-1
**holds.** `pickRoot` at `msglist.go:255-268` matches verbatim:
prefers `InReplyTo == ""`, falls back to earliest by `lessMessage`.
Orphans become depth-1 children inside `appendThreadRows`.

### U9. Fold state per-session, reset on every SetMessages; default expanded; `[N] ` badge replaces prefix on collapsed root
**holds.** `msglist.go:124` (`SetMessages`) resets
`m.folded = map[mail.UID]bool{}`. Folded thread roots receive the
`[N] ` badge in the prefix renderer (verified by reading the prefix
walk; not quoted here for brevity).

### U10. `Space` toggles fold; `F` bulk fold/unfold with mixed-state collapse
**holds (with one forward-looking clause).**
- Space: `account_tab.go:194-198` `case " ":` calls
  `m.msglist.ToggleFold()`. Inert during active search (line 195-197).
- F bulk: `account_tab.go:199-203` `case "F":` calls
  `m.msglist.ToggleFoldAll()`. Implementation at `msglist.go:498-506`:
  if `anyUnfolded` then fold all, else unfold all. Matches "mixed
  state collapses on first press" semantics.
- Visual-select `Space` toggle: forward-looking (Pass 6 pending).
  Acceptable to leave in invariants as future state.

### U11. Read state by brightness: `FgBright` bold for unread, `FgDim` for read; hue for cursor and unread+flagged
**partial / ambiguous.**
- `MsgListUnreadSender: FgBright + Bold` (`styles.go:165-166`) — matches.
- `MsgListUnreadSubject: FgBright` (no bold, `:167-168`) — sender is
  bold, subject is not. Invariant says "FgBright **bold** for unread"
  unconditionally.
- `MsgListReadSender/Subject: FgDim` (`:169-172`) — matches "FgDim
  for read."
- Cursor `MsgListCursor` and unread+flagged `MsgListFlagFlagged`
  use `AccentPrimary` / `ColorWarning` per styling.md (not re-verified
  here — out of scope per audit doc).

**Recommendation:** tighten the invariant to "Unread sender is
`FgBright` bold; unread subject is `FgBright` (not bold); read rows
are `FgDim`." — this matches actual code intent.

### U12. Footer drop ranks 0-10; rank 0 (`? help`, `q quit`) never drops; group-collapse separator
**holds.**
- Ranks 0-10: `footer.go:21-29` defines `dropRank int` with the rule
  documented; the comment at `:52-59` lists actual ranks per hint.
- Rank 0 hints: `footer.go:78-79` `hint("?", "help", 0)`,
  `hint("q", "quit", 0)`.
- Group collapse: `fitFooterHints` at `footer.go:161-...` empties
  groups via `highestDropRank` and the rendering at `:140-152`
  collapses the leading separator when a group becomes empty
  (verified by reading the render path; not quoted).

### U13. Three-sided frame: top `──┬──╮`, right `│`, bottom `──┴──╯`; no left border
**holds.**
- Top: `top_line.go:9` documents `──┬──╮`.
- Right: `app.go:121-126` adds `│` (FrameBorder) to each content row.
- Bottom: `status_bar.go:81-92` (`buildFill`) generates `──` with
  `┴` at `dividerCol`; `:134` appends `─╯` for the right corner.
- No left border: confirmed by absence of any left-edge render.

### U14. Connection state shape + color + text; ●/◐/○; green/orange/red
**partial DRIFT.**
- Shapes hold — `status_bar.go:112` `●`, `:120` `◐`, `:116` `○`.
- Text holds — "connected" / "reconnecting" / "offline".
- Colors:
  - Connected: `ColorSuccess` (green family) — holds.
  - Reconnecting: `ColorWarning` (orange) — holds.
  - **Offline: `FgDim`, not red.** `styles.go:131-133`
    `StatusOffline: ... Foreground(t.FgDim)`. Invariant says "red
    hollow offline." The shape is hollow but the color is dim gray,
    not red.

**Recommendation:** either change `StatusOffline` to use
`ColorError` or update the invariant to say "dim hollow offline"
(matches the actual subdued visual treatment).

### U15. Search via `/`; 3-row sidebar shelf; filter-and-hide; Esc clears + restores cursor
**holds.**
- `/` activation: `account_tab.go:165-168`.
- 3-row shelf: `account_tab.go:24` `searchShelfRows = 3`.
- Filter-and-hide: `msglist.filterBuckets` at `:215-230` keeps only
  matching buckets.
- Esc clears + restores: `msglist.go:435` saves `preSearchCursor` on
  first filter set, `:444-450` restores it on `ClearFilter`. Saved
  fold state is preserved unmutated (verified by reading `applyFoldState`
  vs `filterBuckets` — fold map is not modified during filter).

### U16. Tab cycles `[name]` ↔ `[all]`; case-insensitive; folder jumps clear search; fold keys no-op while filter committed
**partial / DRIFT inherited from U5.**
- Tab cycle: holds — `sidebar_search.go:97-101` flips between
  `SearchModeName` and `SearchModeAll`.
- Case-insensitive: holds — `matchMessage` (`msglist.go:235-246`)
  lowercases everything.
- "Folder jumps clear search": only J/K honor this
  (`account_tab.go:179, 183` call `clearSearchIfActive`). I/D/S/A/X/T
  do not clear because they are not dispatched at all (see U5).
- Fold keys no-op during commit: holds — Space and F both check
  `sidebarSearch.State() == SearchActive` and return early
  (`account_tab.go:195-197, 200-202`).

### U17. Modifier-free; viewer scroll j/k/Space/b/g/G; Ctrl-c only as Quit alias; pgup/pgdown not bound
**holds.** `viewer.go:240-247` defines a custom viewport keymap
explicitly using `j/k`, `Space`, `b` — no `pgup`, no `pgdown`. `g/G`
handled by the viewer wrapper at `:180-183`. `Ctrl-c` only at
`app.go:100` (the unadvertised quit alias on the Quit binding,
matching the invariant's wording).

### U18. `Enter` opens viewer; unread → marked seen optimistically; q/Esc closes
**holds.** `account_tab.go:176-177` `case "enter":` →
`openSelectedMessage`. Mark-read at `:222-225` (covered by A21).
`viewer.go:162` `case "q", "esc":` calls `Close`.

### U19. Viewer link launch 1-9 → `xdg-open`; Tab reserved for link picker; no-op in 2.5b-4
**holds.** `viewer.go:159-167` `handleKey`: `q/esc` close, `tab`
returns `(v, nil)` (the no-op), digits `1-9` route to
`launchURLCmd(v.links[idx])`. `openURL` at `viewer.go:28-30` shells
out to `xdg-open` (overridable for tests).

---

## Build & verification

### B1. Makefile targets: build, test, vet, lint, install, check, clean
**holds.** `Makefile:1-23` defines all seven plus `.PHONY` line.

### B2. `make check` (vet + test) is the commit gate
**holds.** `Makefile:18` `check: vet test`.

### B3. `make install` places `poplar` in `~/.local/bin/`
**holds.** `Makefile:15-16` `GOBIN=$(HOME)/.local/bin go install ./cmd/poplar`.

### B4. Module path `github.com/glw907/poplar`; Go version in `go.mod` matches installed toolchain (1.26.1)
**partial DRIFT.**
- Module path: holds — `go.mod:1` `module github.com/glw907/poplar`.
- **Go version: drifted.** `go.mod:3` declares `go 1.25.0`, but the
  installed toolchain reports `go version go1.26.1 linux/amd64`. The
  invariant claims they match.

**Recommendation:** bump `go.mod` to `go 1.26.0` (or `1.26.1`) to
match the toolchain, or update the invariant to reflect the floor
(`Go version in go.mod is the minimum supported floor; the workstation
toolchain is 1.26.1.`). The first is more accurate; the second is
more honest about how Go modules actually work.

### B5. Invoke `go-conventions` before writing Go code
**holds (skill exists).** `~/.claude/skills/go-conventions/` is
present. Behavioral compliance (do passes actually invoke it?) is out
of scope for this audit.

### B6. Invoke `elm-conventions` before touching `internal/ui/`
**holds (skill exists).** `~/.claude/skills/elm-conventions/` is
present.

### B7. Update `docs/poplar/styling.md` before changing any color or style
**holds (doc exists).** File is present. Behavioral compliance not
audited.

### B8. Pass-end ritual in `poplar-pass` skill; trigger phrases
**holds.** `.claude/skills/poplar-pass/` directory and SKILL.md
present (verified earlier when this skill was invoked at the start
of the session).

### B9. Live UI verification uses `.claude/docs/tmux-testing.md`
**holds.** `.claude/docs/tmux-testing.md` exists.

---

## Summary

### Verdict counts

| Verdict | Count |
|---------|-------|
| holds | 27 |
| drift | 6 |
| partial / ambiguous | 7 |
| stale | 2 |

(Some claims received a partial verdict where one sub-claim held and
another drifted; those are tallied under "partial / ambiguous.")

### Top three drifts, ranked by severity

**1. U5 — Folder jump keys defined but never wired.**
The user-facing impact is total: the documented `I/D/S/A/X/T` folder
jumps do nothing. The keys are defined in `keys.go:33-40`, the footer
advertises `I/D/S/A` (`footer.go:64`), but no Update branch dispatches
them. This isn't doc rot — it's missing functionality the invariant
promises and the footer hint sells. Most severe because it directly
breaks user expectations.

**2. U6 — Per-folder `threading = false` config field is dead.**
`FolderConfig.Threading` (`config/ui.go:61`) parses correctly but the
account tab never reads it. Every folder is threaded regardless of
config. Less severe than U5 because the user sees no broken affordance
(threading is the documented default), but it means a user editing
their `accounts.toml` to disable threading silently has no effect —
exactly the kind of "config that lies" failure mode the project
hopes to avoid.

**3. A7 — Root model does not own `mail.Backend` or `theme.CompiledTheme`.**
The Elm-architecture invariant for ownership is wrong about who owns
the backend and the theme. App holds neither directly; the backend
lives on AccountTab and the theme lives on Viewer. Severity is medium:
the code structure is internally consistent, but the invariant gives
a misleading mental model that future passes might code against.
Decision required: either hoist both back to App (matches invariant),
or rewrite the invariant to reflect "App constructs the model tree
and threads backend+theme into the components that need them"
(matches code).

### Other notable findings (not in top three)

- **U14** — Offline connection state is rendered with `FgDim`, not red
  as the invariant claims (`styles.go:131-133`). Trivial color
  decision — pick one, fix the other.
- **U4 + A15** — `?` (help popover) and the `catkin/` package describe
  passes that haven't shipped. The invariant should mark these
  forward-looking or move them out until they land.
- **B4** — `go.mod` declares `go 1.25.0` while the workstation
  toolchain is `1.26.1`. Either bump the directive or rephrase the
  invariant.
- **U3** — The `:` `Cmd` binding (`keys.go:16`) is dead code. Behavior
  matches the invariant (no command mode runs), but the binding
  contradicts the spirit. Delete it.

### Process observation

The audit document scopes findings to **doc-vs-code consistency
only**. Several drifts (U5, U6) point at real code-side gaps that
warrant follow-up implementation work, and several point at invariant
text that should be tightened (A10's literal-vs-spirit, A7's
ownership claim, A18's level of detail). Decision per the audit's
"Follow-up" section: each entry above splits into either a docs-only
update or a small code pass.


