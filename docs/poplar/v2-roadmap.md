# Bubbletea v2 Alignment + Showcase Roadmap

Strategic plan for migrating poplar to charm.land/{bubbletea,
lipgloss,bubbles}/v2 and positioning it as a standout reference
implementation by v0.9.0.

This doc is the detailed companion to the **`v2-showcase`** entry
in `ROADMAP.md`. STATUS.md remains authoritative for the current
pass; this doc is the multi-pass plan it pivots through.

## Strategic framing

Pre-beta posture (ADR-0105) gives us license to align poplar
aggressively with v2 idiom before v0.9.0 ships. The user's
explicit framing: *infinite slack, position poplar as a standout
example of what's possible with bubbletea v2.*

That converts what would otherwise be a "minimum-viable migration"
into a multi-phase initiative spanning 22 passes:

- **Substrate** (13.2–13.4) — v2 migration, ansix decision, finish
  Pass 13.1's search UX.
- **Foundation** (14–14.1) — first-run wizard, OAuth refresh,
  capability resolution extending `term.Resolve`.
- **Internal idiom alignment** (15–17) — Styles cohesion, sidebar
  tree refactor, overlay system refactor.
- **UX completion** (18–19) — reader and compose finished.
- **Showcase features** (20–22) — inline images, compose
  text-entry parity, inline mode subcommands.
- **Rendering polish** (23–25) — HTML email, color, interactions.
- **Pattern audit** (26) — idiomatic bubbles where they fit.
- **State polish** (27–28) — empty/error states, chrome.
- **Compatibility** (29–30) — themes, terminals.
- **Showcase deliverables** (31) — public docs, reference-apps
  submission.
- **Release** (32) — v0.9.0 ships.

## Sequencing principle

**Structural alignment before user-facing polish, polish before
showcase features, showcase features before compatibility audit.**

Counter to the natural instinct (ship UX wins early, defer
internal refactors), the structural-first ordering means showcase
features are written against the fully-idiomatic v2 foundation
rather than retrofit onto legacy primitives. Ordering UX-first
would mean every Tier A surface (reader, compose) gets touched
twice — once for v2 patterns and again when the structural
refactors land.

The structural refactors (Styles cohesion, sidebar tree, overlay
system) are internal-only; visible behavior is unchanged. Tests
catch regressions. The risk profile favors landing them early.

## Pass sequence

| # | Pass | Goal | Tasks | ADR |
|---|---|---|---|---|
| 13.2 | charm.land/v2 migration | mechanical drift + 3 v2-forced reframes (declarative chrome, cursor hoist, drop AdaptiveColor) + 3 v2-enabled coherence moves (cursored→tea.View, PasteMsg, VirtualCursor=false) | ~12 | 0189 |
| 13.3 | ansix audit + shrinkage | measure SPUA accuracy under v2 across terminal matrix; decide ansix's fate (delete / keep / shrink to fallback) | ~5 | 0190 |
| 13.4 | Search completeness | wire `viewport.SetHighlights` to search-result open path; `n`/`N` jump bindings; closes Pass 13.1 UX | ~5 | 0191 |
| 14 | First-run wizard (#27) + #29 + capability resolution | first-run wizard + extend `term.Resolve` to return `(IconMode, spuaCellWidth, colorprofile.Profile, isDark, terminalVersion)`; first-run uses background detection for theme suggestion | ~12 | (existing + 0192) |
| 14.1 | OAuth refresh | unchanged from prior plan | ~6 | (existing) |
| 15 | Styles cohesion (v2 idiom) | per-subpackage `Styles` restructured to v2 textinput's `Focused`/`Blurred` nested pattern; consistent shape across all 7 UI subpackages + uicore | ~8 | 0193 |
| 16 | Sidebar tree refactor | replace bespoke account/folder tree with `lipgloss.Tree`; preserve T9 contacts mode + per-letter cursor; same keymap, same wireframe | ~8 | 0194 |
| 17 | Overlay system refactor | replace `uicore.PlaceOverlay`/`CenterOverlay` + hand-rolled modal cascade with `lipgloss.Layer`/`Canvas` Z-ordered compositing; preserve cascade semantics | ~8 | 0195 |
| 18 | Reader experience | OSC 8 hyperlinks, viewport horizontal scroll, optional `LeftGutterFunc` line numbers in code blocks | ~8 | 0196 |
| 19 | Compose & cross-app I/O | textarea `DynamicHeight`, native clipboard via `tea.SetClipboard`/`ReadClipboard` (drop `atotto/clipboard`, gain SSH), textarea PageUp/PageDown, `Dropdown`-vs-`textinput`-suggestion measure-then-decide | ~8 | 0197 (+0198 if Dropdown replaced) |
| 20 | Inline images in reader | kitty image protocol + sixel; multipart/image rendering inline | ~10 | 0199 |
| 21 | Compose text-entry parity | Kitty keyboard protocol for shift+enter, ctrl+arrow word-jump, etc. — text-entry semantics only, NOT user keybindings (the modifier-free user-keybinding policy stands) | ~6 | 0200 |
| 22 | Inline mode subcommands | `poplar reply <msg-id>`, `poplar quick-send`, `poplar print <msg-id>` for shell-pipeline use; v2's declarative chrome makes inline mode a one-line conditional in `App.View()` | ~8 | 0201 |
| 23 | HTML email rendering polish | inline image rendering in HTML path, typography pass on goldmark, link styling | ~7 | 0202 |
| 24 | Visual polish I — color & cursor | gradient progress / sync indicator via `progress.WithColors`; underline color in error/warn; cursor styling per context (compose vs catkin vs search) via `tea.Cursor.Shape`/`Color` | ~8 | 0203 |
| 25 | Visual polish II — interactions | `tea.View.OnMouse` for clickable sender column → mailto, attachment chip → open, draggable sidebar resize; paste-in-progress feedback via `PasteStartMsg`/`PasteEndMsg`; hover preview on truncated subjects | ~7 | 0204 |
| 26 | Bubbles adoption audit | measure messagelist/outbox/filepicker for bubbles v2 fit; adopt where idiomatic, document why where bespoke | ~6 | 0205 |
| 27 | Empty-state + error-state polish | delightful empty inbox/search/contacts/no-accounts; OAuth expiration UX; connection-failure copy | ~7 | 0206 |
| 28 | Chrome polish | popover dim (#14), `tea.View.ProgressBar` (OSC 9;4) for sync/index, leftover items surfaced 10–14 | ~7 | 0207 |
| 29 | Theme parity audit | all 15 compiled themes through every screen at every responsive tier; goldens; fix any cosmetic regressions | ~6 | 0208 |
| 30 | Terminal compatibility matrix | explicit testing across kitty/ghostty/wezterm/foot/iTerm/Apple Terminal/gnome-terminal/xterm/mosh; document per-terminal capability + degradation; fix anything broken on major terminals | ~8 | 0209 |
| 31 | v2 showcase documentation | `docs/poplar/v2-features-used.md` (feature inventory + code references); blog-post draft; Charm reference-apps submission prep | ~5 | (no ADR; deliverable doc) |
| 32 | v0.9.0 prep | feature freeze, README, docs sweep, tag, ship | ~10 | (release notes) |

ADR numbers are placeholders; actual numbers depend on what
intervening passes assign.

## Explicit non-goals

These are reconsidered against the showcase positioning and
*still* rejected. Naming them now prevents litigating each one
mid-pass.

- **Modifier-key user keybindings.** Poplar is pine-style, not
  vim-style. The "no Ctrl+/Alt+ in user-facing keybindings"
  policy is poplar's identity. Pass 21 covers *text-entry
  semantics* (shift+enter for newline-vs-send, ctrl+arrow for
  word-jump in catkin) — those are standard editor behaviors,
  not keybindings users have to memorize. The policy stands.
- **`tea.WithEnvironment`/`EnvMsg`.** Wish-hosted apps only;
  poplar runs locally.
- **Mouse-driven primary navigation.** Vim-first identity. Mouse
  interactions stay supplementary, never required (Pass 25 adds
  clickable surfaces as *additional* affordance, never as
  required UI).
- **Runtime configurability of themes/keybindings.** ADRs
  0015/0024/0051/0068/0076. Themes are compiled Go values; key
  bindings are fixed.

## Items deferred post-1.0 (1.1+)

- **Neovim companion plugin.** Project goal beyond v1 (memory:
  `project_nvim_companion`). Architecture stays compatible.
- **`tidytext` standalone tool spinoff.** `internal/tidy/` may
  become its own tool (memory: `project_tidytext_spinoff`); don't
  compromise poplar architecture to keep it pure — extract at
  spinoff time.
- **Additional backends.** v1 ships JMAP + IMAP. No maildir,
  mbox, notmuch.
- **Kitty image protocol enhancements** beyond Pass 20's basic
  inline-images-in-reader (e.g., zoom, full-resolution view in a
  separate panel). Post-1.0 reader work.

## Items left in BACKLOG (not currently scheduled)

- **Color blending / gradient effects beyond Pass 24's progress
  indicator and underline color.** Cosmetic; only fold in if a
  polish pass has slack.
- **Tree component for sidebar** beyond Pass 16's adoption (e.g.,
  expand/collapse for nested folders). Currently nested folders
  render flat by invariant; revisit if user feedback demands.

## Showcase deliverables (Pass 31)

This pass produces externally-facing artifacts:

- **`docs/poplar/v2-features-used.md`** — running inventory of
  every bubbletea/lipgloss/bubbles v2 feature poplar uses, with
  file:line code references. Seeded at the end of Pass 13.2;
  grown each subsequent pass; finalized in Pass 31.
- **Blog-post draft** — case study of poplar's v2 adoption,
  intended for the Charm community blog or the user's personal
  blog. Anchored on the `v2-features-used.md` inventory.
- **Charm reference-apps submission** — submission to whichever
  Charm-community-maintained list of bubbletea reference apps
  exists at v0.9.0 ship. Uses the inventory + a short pitch.

## Cross-references

- **STATUS.md** — current pass + active starter prompt.
- **ROADMAP.md `v2-showcase`** — strategic framing entry.
- **`docs/superpowers/specs/2026-05-09-pass-13-2-charm-v2-design.md`**
  — Pass 13.2 spec (migration core).
- **`docs/superpowers/plans/2026-05-09-pass-13-1-5-claude-infra-v2-prep.md`**
  — Pass 13.1.5 plan (Claude infra prep ahead of 13.2).
- **`docs/poplar/invariants.md`** — binding facts, updated as
  passes land.
- **`docs/poplar/decisions/INDEX.md`** — themed map of binding
  facts to ADR numbers.

## Maintenance

This doc is a living plan. Update as passes complete:

- Mark passes done by appending `✓` to the pass number column.
- Adjust task counts and ADR numbers as actual passes land.
- Add new passes if scope shifts; remove or merge if reality
  diverges.
- When v0.9.0 ships, move the entire `v2-showcase` ROADMAP entry
  to `## Done` and archive this doc to
  `docs/poplar/archive/v2-roadmap.md` with a brief retrospective.
