# Filepicker lessons — Pass 15a.5

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Audit `internal/ui/compose/attachpicker.go` against `charm.land/bubbles/v2/filepicker`, lift three concrete improvements, and document why full filepicker adoption was rejected. Closes the 15a.5 slot in the bubbles-adoption sequence (15a, 15a.5, 15b, 15c, 15d) without importing the upstream bubble.

**Architecture:** No new packages, no new exports. Three surgical edits inside `compose/attachpicker.go` plus an ADR.

**Tech stack:** Go 1.26, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/key`, stdlib `path/filepath`, `sync/atomic`.

**Predecessor:** Pass 15a (ADR-0194). The bundled `compose.New()` value-return fix already landed in 62ade64.

---

## Why not adopt `bubbles/v2/filepicker`

The starter prompt asked whether the partial fit warranted an ADR'd deviation. Reading the upstream source (`charm.land/bubbles/v2@v2.1.0/filepicker/filepicker.go`):

- **Single-select model.** `Model.selected` is the cursor index; `Model.Path` is the one accepted path. There is no marked-set abstraction. Our picker is multi-select with a `selected map[string]bool` and an `a` accept binding.
- **Overloaded Enter.** `KeyMap.Open = l/right/enter` and `KeyMap.Select = enter` both match the same key. Our model uses Enter for "open dir, or single-attach shortcut when nothing is marked, otherwise toggle this file". Composing this with filepicker's stateful Update would require intercepting Enter before its switch sees it.
- **No `ShowHidden` keybinding.** The field exists; the keymap doesn't expose a toggle. We bind `.`.
- **`Esc` is bound to `KeyMap.Back`.** We bind `Esc` to Cancel. Overriding the keymap removes the cost-free `h`/`backspace`/`left` ascend.
- **Different visual language.** Upstream renders permissions + file size columns through its own `Styles` struct. We render icons (`uicore.IconSet.Attachment` / `CustomFolder`), a `✓` selection mark, and a humanized size, inside a `uicore.ModalShell` Box with title + footer hints. A real adoption replaces upstream's `Styles` wholesale, which defeats the adoption.
- **Async readDir + id-guard structure is already identical.** Upstream uses a package-level `atomic.AddInt64` counter and a `readDirMsg{id, entries}` envelope; we use a per-model `id++` counter with the same envelope shape. No incremental win from swapping.

The net of a "real" adoption: wrap filepicker, override its Styles entirely, intercept Space/Enter/`.`/Esc before its Update sees them, keep our multi-select map + ModalShell wrap, and couple ourselves to upstream's Back-includes-Esc and Open-includes-Enter quirks. The resulting file would be longer than today's ~400 lines and structurally less clear.

The current `compose/attachpicker.go` is already idiomatic-bubbletea: upstream `key.Binding` + `key.Matches`, `tea.WindowSizeMsg`/`SetSize`, id-guarded readDir Cmd, no state in `View()`. It just doesn't import filepicker.

ADR-0195 codifies this as an accepted deviation. ADR-0194's pattern ("each picker imports the upstream bubble where it fits") still holds; this one names the file as the exception with its rationale.

---

## Files

**Modify:**
- `internal/ui/compose/attachpicker.go` — three surgical edits (symlink handling, atomic id counter, size column).
- `internal/ui/compose/attachpicker_test.go` — symlink coverage; existing tests stay green.
- `.claude/rules/ui-invariants.md` — narrow the AttachPicker bullet to name the deviation explicitly.

**Create:**
- `docs/poplar/decisions/0195-compose-attachpicker-no-filepicker.md`

**No deletes.**

---

## Tasks

### Task 1 — Symlink resolution in `readDirCmd` and `descend`

- [ ] In `readDirCmd`, when classifying an entry, call `os.Lstat` to detect symlinks via `info.Mode()&os.ModeSymlink != 0`. For symlinks, resolve via `filepath.EvalSymlinks` + `os.Stat` and use the resolved target's `IsDir()` for `attachEntry.isDir`. On resolution error, treat the symlink as a non-traversable file (current behavior).
- [ ] Add a `target string` field to `attachEntry` (zero when not a symlink). Populated only when symlink resolution succeeds; used by `formatEntry` to append ` → <target>` to the row.
- [ ] `formatEntry` appends the target arrow before size-column padding when `e.target != ""`. Truncate the target if it would push the row over `contentW` — the row's primary content is the name, not the target.
- [ ] Test: create a tempdir with one regular file, one regular directory, one symlink to a directory inside the tempdir, one broken symlink. Assert `entries` classifies the dir-symlink as `isDir=true` and the broken symlink as `isDir=false` with empty `target`.

### Task 2 — Atomic package-level id counter

- [ ] Replace per-model `p.id++` with a package-level `var nextAttachID atomic.Int64` and `func nextID() int { return int(nextAttachID.Add(1)) }`. Convention matches `filepicker.lastID` + `atomic.AddInt64`.
- [ ] `Open`, `descend`, `ascend`, and the `ToggleHidden` branch all call `nextID()` instead of mutating `p.id`. Drop the `id int` field from `AttachPicker`; stamp `id` into `readDirMsg` directly and compare against a freshly-captured value via a local `p.currentID int` that the helpers update through `p.currentID = nextID()`.
- [ ] Test: existing id-guard test (stale readDirMsg dropped) stays green.

### Task 3 — Size column via right-aligned style

- [ ] Replace the `PadOrTruncate(body, contentW-len(size)-1) + " " + size` construction in `formatEntry` with a styled right-aligned size column: `p.styles.PickerDim.Width(7).Align(lipgloss.Right).Render(size)`. The 7-cell budget matches `filepicker.fileSizeWidth` and fits `999.9MB`.
- [ ] Body left-segment is `PadOrTruncate(body, contentW-7-1)` (one cell of gutter between body and size). Empty size for directories renders as 7 cells of padding.
- [ ] Use `ansix.Width(size)` for the gutter math if we ever go non-ASCII; current `humanize.Bytes` output is ASCII so the budget is exact.
- [ ] Test: row width assertion for a dir row (no size) and a file row (with size) — both must equal `contentW` cells.

### Task 4 — ADR-0195 + invariants narrowing

- [ ] Write `docs/poplar/decisions/0195-compose-attachpicker-no-filepicker.md` with the rationale from the "Why not adopt" section above, stated as a binding fact: "`internal/ui/compose/attachpicker.go` is a deliberate hand-rolled deviation from the bubbles-adoption pattern. Multi-select + icon UX + ModalShell wrap do not compose with filepicker's single-select + permissions/size columns + freestanding View. Three concrete patterns lifted from upstream: symlink resolution, atomic id counter, right-aligned size column."
- [ ] Update `.claude/rules/ui-invariants.md` AttachPicker bullet: name the deviation, point at ADR-0195.
- [ ] Update `docs/poplar/decisions/INDEX.md` row for the new ADR.

### Task 5 — Pass-end ritual

- [ ] `/simplify` over the diff.
- [ ] Idiomatic-bubbletea §10 checklist against the diff and a live tmux capture at 120×40 with the picker open over a compose draft.
- [ ] `make check`.
- [ ] Update `docs/poplar/STATUS.md`: mark 15a.5 done, write the 15b starter prompt (sidebar folder hierarchy on a v2 tree component).
- [ ] Archive this plan to `docs/superpowers/archive/plans/`.
- [ ] Commit, push, install.

---

## Non-goals

- No multi-instance picker support (one picker open at a time stays the rule; the atomic counter is convention, not capability).
- No `AllowedTypes` filter — no current consumer.
- No `HighlightedPath()` accessor — no current consumer.
- No keymap widening (`ctrl+n`/`ctrl+p` aliases are modifier-bound and violate poplar's modifier-free policy).
- No filepicker import. ADR-0195 is the accepted deviation.
