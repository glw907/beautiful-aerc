# Poplar Status

**Current pass:** Pass 17a — Sidebar folder hierarchy on bubbles/v2
tree component. First of the bubbles-adoption remainder (17a sidebar
tree, 17b messagelist on `bubbles/v2/list`, 17c help audit) after
the 16-series modernization locked modern-Go conventions.

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
| 16a | Claude infra: modern-Go defaults (skill bump + simplify Agent 3 + `modern-go-check.sh`; ADR-0196) | done |
| 16b | Mechanical Go modernization sweep — `slices.SortFunc`, `slices.Sort`, `for range N`, `sync.OnceValue`, `slices.Chunk`, `slices.Sorted+maps.Keys`; ~60 sites, no ADR | done |
| 16c | `iter.Seq` for `catkin/style.go` `walkSpans` + 3 callers; no ADR | done |
| 16d | `log/slog` adoption + logging-convention ADR (ADR-0197) | done |
| **17a** | **Sidebar folder hierarchy on a v2 tree component (plan + spec on disk; ADR-0197)** | **pending — next** |
| 17b | `messagelist` on `bubbles/v2/list` (custom item renderer; absorbs BACKLOG #46 `iter.Seq2`; ADR if it survives) | pending |
| 17c | `bubbles/v2/help` audit + ADRs for bubbles deviations that survive 15a/17a/17b (`helppopover`, `schedulepicker`) | pending |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 17a)

> **Goal.** Render Custom folders whose names contain `/` as a real
> tree in the sidebar — expand/collapse via `→`/`←`, collapsed
> parents sum descendant unread, Spartan tier caps indent at depth 1.
> Also adds `sidebar.KeyMap` + `Update`, retiring the imperative
> `MoveUp`/`MoveDown`/`MoveToTop`/`MoveToBottom` mutators (BACKLOG
> #45 item 4).
>
> **Scope.** `internal/ui/sidebar/` (new `tree.go`, `model.go`,
> `styles.go`); `internal/ui/account/model.go` (route sidebar keys
> through Update); `internal/ui/uicore/layout.go` (confirm Spartan
> depth signal); `internal/theme/palette.go` + `internal/ui/styles.go`
> (wire `SidebarTreeRule`); `docs/poplar/styling.md`; ADR-0197
> (sidebar tree decision). Plan:
> `docs/superpowers/plans/2026-05-10-sidebar-tree.md`. Spec:
> `docs/superpowers/specs/2026-05-10-sidebar-tree-design.md`.
>
> **Settled (do not re-brainstorm):** Hand-rolled tree on the
> existing `renderRow` — transient `*node` map built from
> `folderEntry`s during view, DFS-walked, yields `[]rowMeta`, then
> discarded (mirrors `messagelist.appendThreadRows`). No new
> library. Expand state in `map[string]bool` keyed by provider
> folder name; pruned at `SetFolders`. `→` expand, `←` collapse;
> Spartan caps `maxDepth = 1`. Primary and Disposal groups stay
> flat. All decisions in the spec are settled — no open questions.
>
> **Approach.** Read the plan and spec, then use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement task by task.
> Standard pass-end ritual; `make check` green before commit.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
