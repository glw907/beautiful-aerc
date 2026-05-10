---
title: First-run wizard architecture — sections, strategies, huh adoption
status: accepted
date: 2026-05-10
---

## Context

#27 (first-run wizard) is a v1 blocker. The probe + config substrate
landed in Pass 14a (ADR-0190); Pass 14b is the wizard surface itself
plus a new `poplar config init --interactive` cobra entry point. The
first-run auto-launch + `--repair` flag defer to Pass 14c so this
pass stays inside the 8–12 task budget. OAuth interactive flow is
Pass 14.1.

The wizard has to handle several configurable surfaces (account today;
contacts, signatures, tidy in later passes) without growing a tangled
state machine, and each provider preset has its own credential ritual
(app password, API token, OAuth, plain IMAP/JMAP). Both axes wanted
plug-in seams.

## Decision

The wizard splits into two packages.

**Domain (`internal/wizard/`).** Pure data + dispatch. `Model` carries
the user's collected values. `Strategy` is an interface keyed by
`config.CredentialStrategy`; concrete impls (appPassword, apiToken,
oauth, plainIMAP, plainJMAP) exist for every preset. `Apply(Model)`
returns a ready-to-render `config.AccountConfig`. `Probe(ctx, cfg)`
dispatches on `cfg.Backend` to `mailimap.Probe` (and appends the SMTP
probe) or `mailjmap.Probe`. No bubbletea imports in this layer.

**UI (`internal/ui/wizard/`).** Bubbletea tea.Model parent + per-
section sub-models. Sections compose via the `defaultSections`
registry; the only sections wired in 14b are account, theme, confirm.
huh.Form drives every static form page; the probe screen is a custom
tea.Model because its live spinner + ProbeStep transcript don't fit
huh's static `Note` field.

The huh-rendered forms inherit from poplar's compiled theme via a
`HuhTheme` adapter (`huh.ThemeFunc` closure mapping
`theme.CompiledTheme` slots onto `huh.Styles`). Per-section `Styles`
struct lives at `internal/ui/wizard/styles.go`, matching the existing
bubbles-shaped subpackage convention (ADR-0163).

Logo: typographic wordmark for now. `art/poplar-logo.ans` carries a
cbonsai artifact embedded via `//go:embed` and committed alongside,
ready for a later switch — flipping `renderLogo` to render `LogoART`
(or any other artifact) is one line.

`AccountConfig.Preset string` field is added so the wizard's chosen
preset name round-trips through `config.Render` as
`provider = "fastmail"` (and friends), preserving SMTP and host
defaults on the next config load. The writer prefers `Preset` over
`Backend` when emitting the `provider =` key.

Cobra surface: `poplar config init --interactive` runs the wizard
(`--section=name1,name2` filters the registry). `tea.NewProgram` is
constructed with `WithContext`/`WithInput`/`WithOutput` so the
wizard cooperates with cobra's IO plumbing; `AltScreen` is set
declaratively on the returned `tea.View`.

Deviation from the bubbletea-conventions §1 default: huh.Form is the
form library rather than hand-rolled bubbles components. Reason: huh
covers Select/Input/Confirm/Note with a single API and ships
keymap+help wiring, password-mode echo, and field validation —
re-implementing those on top of bare `bubbles/textinput` would
duplicate ~700 lines for no architectural gain. The `tea.Model`
constraint differs (huh sub-models are v1-style `View() string`); the
wizard's section interface adopts the v1 shape, the parent's `View()`
returns the v2 `tea.View`.

## Consequences

- New direct dep: `charm.land/huh/v2`. Charm-maintained, MIT, fits
  the existing ecosystem.
- `AccountConfig.Preset` is a small additive schema change. Round-
  trip through `config.Render` now preserves preset names instead of
  collapsing them to `"imap"` / `"jmap"`. Pre-beta endorses the
  schema work.
- The wizard's confirm step writes the assembled TOML via
  `os.WriteFile` to `config.Resolve("")` and refuses to clobber an
  existing non-empty file. The first-run auto-launch wires that
  refusal into `runRoot` in Pass 14c.
- OAuth strategy currently writes a placeholder `password-cmd`. Pass
  14.1 replaces it with a device-code flow + keyring storage; no
  data migration since the placeholder is just a string in
  `config.toml`.
- huh's compat.Model (v1-shape `View() string`) ripples into the
  section interface choice — the wizard's `section` interface uses
  v1 shape so huh.Form satisfies it transparently, while the parent
  Model still satisfies v2's `tea.Model`. A future move to all-v2-
  shape would only matter if huh itself migrates.
