# ADR-0195: compose/attachpicker — deliberate deviation from bubbles/filepicker

**Status:** Accepted  
**Pass:** 15a.5  
**Date:** 2026-05-10

## Context

Pass 15a established the bubbles-adoption pattern: each picker imports the upstream bubble where it fits (`reader.AttachPicker`, `reader.LinkPicker`, `movepicker.Model` each wrap a `bubbles/v2/list.Model` — ADR-0194). The compose-side `AttachPicker` (`internal/ui/compose/attachpicker.go`) was flagged for a parallel audit against `bubbles/v2/filepicker`.

## Decision

`internal/ui/compose/attachpicker.go` is a hand-rolled deviation from the bubbles-adoption pattern. A real `filepicker` import would produce a file longer than the current ~400 lines and structurally less clear.

The incompatibilities that make adoption infeasible without a net-negative result:

- **Single-select model.** `filepicker.Model.selected` is the cursor index; `Model.Path` is the one accepted path. Our picker's `selected map[string]bool` and `a`-accept binding have no filepicker analogue.
- **Overloaded Enter.** Upstream's `KeyMap.Open` and `KeyMap.Select` both match `enter`. Our Update uses `enter` for three cases: open directory, single-attach shortcut, or toggle-into-selection. Composing that with filepicker's Update requires intercepting `enter` before its switch — erasing the point of adoption.
- **No `ShowHidden` keybinding.** The `ShowHidden` field exists in filepicker; the keymap has no toggle. We bind `.`.
- **`Esc` bound to `KeyMap.Back`.** We bind `Esc` to Cancel and `h`/`backspace`/`left` to ascend. Overriding `Esc` removes the back bindings.
- **Different visual language.** Filepicker renders permission columns and its own `Styles`. Our picker renders `uicore.IconSet` glyphs, a `✓` selection mark, and humanized sizes inside a `uicore.ModalShell` box with title and footer hints.
- **Async readDir is already identical.** Upstream uses a package-level `atomic.AddInt64` counter + `readDirMsg{id, entries}`; we used a per-model `id++` counter with the same envelope. Swapping counter style is convention, not adoption.

## Patterns lifted from filepicker (Pass 15a.5)

Three concrete improvements from reading the upstream source are applied without importing it:

1. **Symlink resolution.** `e.Type()&os.ModeSymlink` detects symlinks via `os.DirEntry.Type()` (unresolved type bits). On match: `filepath.EvalSymlinks` + `os.Stat` sets `attachEntry.isDir` and `attachEntry.target`. On resolution error (broken link): treat as a non-traversable file. `formatEntry` appends ` → <target>` when `target` is non-empty, truncated to fit inside `contentW`.
2. **Atomic package-level id counter.** Replaces per-model `id++` with `var nextAttachID atomic.Int64` and `nextID() int`. Matches `filepicker.lastID` convention. `AttachPicker.currentID` field holds the last-issued value; all four callers (`Open`, `descend`, `ascend`, `ToggleHidden`) call `nextID()`.
3. **Right-aligned size column.** `p.styles.PickerDim.Width(7).Align(lipgloss.Right).Render(size)` replaces the manual pad+concatenate. 7-cell budget matches `filepicker.fileSizeWidth` and fits `999.9MB`. Directories render 7 cells of padding (empty `size` string).

## Consequences

- `internal/ui/compose/attachpicker.go` does not import `charm.land/bubbles/v2/filepicker`.
- ADR-0194's pattern ("each picker imports the upstream bubble where it fits") still holds; this file is the named exception.
- The three lifted patterns close the substantive delta between this picker and its upstream reference implementation. Future improvements to filepicker can be assessed individually against the same cost model.
