# Poplar Status

**Current pass:** Pass 15b — sidebar folder hierarchy on a v2
tree component. Third of four bubbles-adoption passes (15a,
15a.5, 15b, 15c, 15d) before Polish II and the v0.9.0 freeze.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Background body sync + status indicator (substrate for #38; ADR-0187) | done |
| 13.1 | Search (#38; ADR-0188) | done |
| 13.1.5 | Claude infrastructure v2 prep (skills, conventions, pass ritual) | done |
| 13.2a | `charm.land/v2` substrate — mechanical migration + AdaptiveColor (ADR-0189a) | done |
| 13.2b | `charm.land/v2` reframes — paste; chrome + cursor absorbed by 13.2a (ADR-0189b) | done |
| 14a | Probe + config substrate (#27 part 1, #29; ADR-0190) | done |
| 14b | Wizard domain + huh UI (#27 part 2; ADR-0191) | done |
| 14c | First-run integration (#27 part 3; ADR-0192) | done |
| 14.1 | OAuth refresh (#42; ADR-0193) | done |
| 15a | `bubbles/v2/list` adoption — `movepicker`, `reader/linkpicker`, `reader/attachpicker` (ADR-0194) | done |
| 15a.5 | `compose/attachpicker` filepicker-lessons — symlink + atomic id + size column; ADR-0195 names the deviation | done |
| 15b | Sidebar folder hierarchy on a v2 tree component (`Digital-Shane/treeview` candidate; ADR the dep) | pending |
| 15c | `messagelist` on `bubbles/v2/list` (custom item renderer for thread-prefix walk; ADR if it survives) | pending |
| 15d | `bubbles/v2/help` audit + ADRs for any bubbles deviations that survive 15a–c (`helppopover`, `schedulepicker`) | pending |
| 16 | Polish II — popover dim (#14) + items surfaced during 10–15d | pending |
| 17 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 15b)

> **Goal.** Replace the sidebar's flat-folder rendering with a
> v2 tree component so nested folder names render as a real
> tree (expand/collapse, keyboard-driven). Third of four
> bubbles-adoption passes (15a, 15a.5, 15b, 15c, 15d) before
> Polish II and the v0.9.0 freeze.
>
> **Scope.** `internal/ui/sidebar/` — `Model`, `Column`, and
> the folder-group rendering. The Primary/Disposal/Custom
> group ordering stays; what changes is how Custom folders
> with `/` in their display names render. Candidate library:
> `Digital-Shane/treeview` (vet first per memory's "prior art
> = major clients / Charm-ecosystem first" rule; if it's
> abandoned or non-idiomatic, hand-roll on top of
> `bubbles/v2/list` with indent + glyph). The chosen dep gets
> its own ADR.
>
> **Settled (do not re-brainstorm):** The current invariant
> "nested folder names render flat; `/` is the only
> affordance" is **being lifted** in this pass — that's the
> point. Three-group order (Primary / Disposal / Custom) and
> the synthetic Outbox row in Disposal both stay. Folder
> selection mechanics (`J/K` nav, `Tab` cycle to search) stay.
>
> **Still open — brainstorm these:** how to render the tree
> at the spartan tier (width 80–89, sidebar 14 cells) — one
> indent level cap? overflow with `…`?; whether collapsed
> nodes still show unread counts (probably yes, sum of
> descendants); whether tree state persists across folder
> reloads or resets like fold state in messagelist; key
> bindings for expand/collapse (probably `Space` on a folder
> row, or `l`/`h` for vim-style); whether Primary group can
> contain nested folders or stays single-level.
>
> **Approach.** Brainstorm the open questions, write a plan
> doc at
> `docs/superpowers/plans/YYYY-MM-DD-sidebar-tree.md`, then
> implement. Standard pass-end checklist applies.
