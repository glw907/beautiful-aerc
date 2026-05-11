# Poplar Status

**Current pass:** Pass 15a.5 — `bubbles/v2/filepicker` adoption
for `compose/attachpicker.go` (ADR-0179). Second of four bubbles-
adoption passes (15a, 15a.5, 15b, 15c, 15d) before Polish II and
the v0.9.0 freeze.

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
| 15a.5 | `bubbles/v2/filepicker` adoption — `compose/attachpicker` (multi-select gap to solve) | pending |
| 15b | Sidebar folder hierarchy on a v2 tree component (`Digital-Shane/treeview` candidate; ADR the dep) | pending |
| 15c | `messagelist` on `bubbles/v2/list` (custom item renderer for thread-prefix walk; ADR if it survives) | pending |
| 15d | `bubbles/v2/help` audit + ADRs for any bubbles deviations that survive 15a–c (`helppopover`, `schedulepicker`) | pending |
| 16 | Polish II — popover dim (#14) + items surfaced during 10–15d | pending |
| 17 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 15a.5)

> **Goal.** Adopt `bubbles/v2/filepicker` for
> `internal/ui/compose/attachpicker.go` (ADR-0179) so the multi-
> select TUI file browser composes from the upstream bubble
> instead of hand-rolled directory traversal. Second of four
> bubbles-adoption passes (15a, 15a.5, 15b, 15c, 15d) before
> Polish II and the v0.9.0 freeze.
>
> **Scope.** `internal/ui/compose/attachpicker.go` and tests.
> Keep ADR-0179's keymap (Space toggle, `a` accept, Enter single-
> attach shortcut, `.` toggle hidden, `Esc` cancel) and the
> external `AttachAcceptedMsg` / `AttachCancelledMsg` surface.
> Reuse `uicore.NewListStyles` if filepicker exposes a list-
> styled internal; otherwise add a `uicore.NewFilePickerStyles`
> sibling.
>
> **Settled (do not re-brainstorm):** External Msg surface
> unchanged. Multi-select stays the poplar-side responsibility
> (filepicker is single-select upstream). View-state stack on
> ascend stays the same UX. Footer hint `^O attach` at rank 6
> unchanged.
>
> **Bundled bubble-shape fix (#45 gap 3).** Change
> `compose.New()` to return `Model` (value), not `*Model`
> (`compose/model.go:135`). Every other subpackage returns
> `Model` by value; the pointer return is the only deviation
> and conflicts with the elm value-receiver convention. Update
> the call site in `internal/ui/app.go` and any test
> constructors. Land inline with 15a.5 since this pass already
> touches `compose/`.
>
> **Still open — brainstorm these:** how to layer multi-select
> on top of filepicker's single-select API (own a parallel
> `selected map[string]bool`, intercept Space before dispatching,
> render selected chips above the file list?); whether the async
> readDir + id-guard contract still applies or filepicker handles
> it; whether `.` (hidden toggle) is exposed by filepicker or
> stays poplar-side; whether the partial fit warrants an ADR'd
> deviation instead of full adoption.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-bubbles-filepicker-attach.md`,
> then implement. Standard pass-end checklist applies.
