# Phase 5 directed tooling audit

**Date:** 2026-07-27
**Directive:** Phase 2 gate (Geoff): audit the Go and bubbletea
Claude infrastructure — CLAUDE.md, the go-conventions and
elm-conventions skills, linters, the `make check` gate, the
implementer and reviewer agents — against current best practice
for the settled stack (Go 1.26, bubbletea v2) before the build
machine is finalized. The standing live-research directive
applies: every claim below was verified online or on this machine
on 2026-07-27, by five research dispatches (in-repo inventory, Go
toolchain practice, Charm ecosystem state, keybinding machinery,
mouse support). This report is the evidence base for the build
machine design (`2026-07-27-poplar-build-machine.md`). Revision
2: the machine design's adversarial review caught four defects
in revision 1 of this report, corrected in place and flagged
inline below.

## Verdict

The current machine is not merely stale; parts of it are broken
in ways that fail silently. Nothing in it should carry into the
re-founding by default. The gate scripts worth keeping are
`skipcheck` and `vale-comments.sh`, the latter only after its
vale-missing guard inverts from skip-quietly to hard-fail and
its dotfiles reach-out is made optional-by-explicit-flag — as
found it would pass green on any machine without Vale installed
(review correction; revision 1 called it "portable and verified
working"). Everything else is superseded, legacy-bound, or dead.

## Broken today

1. **The voice catalogue is a dangling symlink.**
   `~/.claude/docs/go-comment-voice.md` was retired
   workstation-wide on 2026-06-22 (dotfiles `8f5e71d`, the
   re-anchor onto external standards plus Vale). Five poplar
   artifacts still cite it as canonical: CLAUDE.md, both Sonnet
   agent definitions, `scripts/voice-check.sh`, and the
   `simplify` skill, whose voice agent cannot execute its own
   first instruction — plus a sixth the review found: the
   archived research copy
   (`docs/poplar/research/go-comment-voice.md`) still names the
   dangling path as "the binding artifact" in its header. Poplar
   runs two parallel voice systems: a grep gate citing a dead
   catalogue, and the live Vale overlay.
2. **golangci-lint cannot run, and the gate never ran it
   anyway.** The installed binary is v2.12.2; `.golangci.yml`
   is v1-schema (no `version:` key), so every run fails with
   "unsupported version of the configuration" — and
   `make lint`'s `command -v` guard masks that as "not
   installed, skipping". Review correction to revision 1's
   diagnosis: `lint` was never a `check` dependency at all, so
   the gate has had no linter by construction, not by
   breakage; the broken config and lying guard are what anyone
   running `make lint` by hand met.
3. **All three agents pin dead model IDs** (`claude-sonnet-4-6`,
   `claude-opus-4-8`; the workstation convention repointed
   2026-07-26). The implementer also misdescribes `make check`
   (claims golangci-lint runs in it; omits vale-comments) and
   asserts a `prose-guard` hook that was retired for Vale.
4. **A live hookify rule enforces deleted doctrine.**
   `.claude/hookify.fix-inline-not-defer.local.md` block-quotes a
   CLAUDE.md "Pre-beta" section removed in the Phase 0 reset.
5. **The `legacy` branch does not exist** (found by the
   machine-design review, verified: `git branch -a` and
   `git ls-remote` show only `master`; the only legacy refs are
   tags). The charter, CLAUDE.md, the routing rule, and the
   STATUS all cite branch `legacy` as the archived client's
   home, and the `poplar-legacy` tag predates the Phase 1 and
   Phase 4 spike tools, which exist on master alone. Every
   salvage instruction in the standing docs points at a ref
   that resolves to nothing; the machine design's boundary
   ritual creates and pushes the branch to make the record
   true.

## Legacy-bound, and a context hazard at the build boundary

6. **Five `.claude/rules/*-invariants.md` files auto-load on path
   globs the new tree will re-create** (`internal/ui/**`,
   `internal/catkin/**`, `internal/search/**`, `internal/mail/*`,
   `internal/cache/**`). The moment Phase 5 lands code under
   those names, archived-client facts (old schema versions, the
   IMAP/JMAP dual backend, the old FTS design) silently re-enter
   agent context. They must be removed at the boundary, not
   after.
7. **Three hooks are legacy-bound, not two** (review
   correction): `bubbletea-conventions-lint.sh` (points at
   `docs/poplar/bubbletea-conventions.md`), `claude-md-size.sh`
   (caps `docs/poplar/invariants.md`), and
   `elm-architecture-lint.sh` (missed by revision 1; greps for
   the archived client's layering on `internal/ui/**`). All
   three are registered in `.claude/settings.json`, which any
   removal must edit in the same commit or every Edit/Write
   invokes missing commands.
8. **Makefile and scripts:** `test-imap` and `internal/mailimap`
   are dead (the design is JMAP-only); `-tags=dev` is an
   undocumented legacy tag; `audit.sh` drives a different binary
   against a third-party client's cache; `check-deep.sh`'s
   mutation gate keys on legacy package names and its tool
   (`gremlins`) is not installed — it has never run in this
   environment.
9. **The global `elm-conventions` skill is load-bearing on
   poplar's archived doc tree** — four `docs/poplar/` citations,
   not three (review correction), of which three point into
   `docs/poplar/research/` and survive the boundary; the one
   that breaks is `docs/poplar/bubbletea-conventions.md`, whose
   content needs a workstation home and a skill repoint, or
   every bubbletea project loses its UI-conventions contract
   silently. The skill's API content is
   otherwise current — it is already bubbletea-v2-native
   (KeyPressMsg, tea.View, cursor hoisting), matching the settled
   stack.
10. **The global `go-conventions` skill is current** (rewritten
   2026-06-22 onto Go Doc Comments, Effective Go, modern-stdlib
   idioms accurate for 1.21-1.26) with one defect: its example
   `.golangci.yml` is the same broken v1 schema.

## Current best practice adopted for the new gate

Findings from the live toolchain research, verified against
release notes and repositories:

- **golangci-lint v2** (v2.12.2 installed; v2.9.0+ supports Go
  1.26) with the v2 config schema, `default: none` plus an
  explicit enable list for legibility, staticcheck consumed
  through it (standalone staticcheck is redundant), and the
  `modernize` analyzer suite built in since v2.6.0 — which
  supersedes `modern-go-check.sh`'s grep approximation.
  `golangci-lint fmt` with gofumpt configured as the formatter
  gives one formatting entry point (gofumpt remains a separate
  maintained superset of gofmt; nothing merged upstream).
- **The go.mod `tool` directive** (Go 1.24+) pins dev tools
  through `go.sum`, replacing install-on-demand and the
  `command -v` guards that just swallowed failures — with one
  large exception the review caught: golangci-lint's own docs
  refuse `go install`/tool-directive installs (dependency-graph
  pollution; they recommend binary or isolated-module installs).
  The workable shape is upstream's own escape hatch, a separate
  `tools/go.mod`, which keeps the pin-through-go.sum property
  without polluting the product module.
- **govulncheck** as a scheduled CI step, v1.6.0 (2026-07-09;
  revision 1 cited v1.1.3 dated 2026 — a stale-year error, that
  release was 2024). The review also established it is wrong as
  a local commit gate: no offline cache, and its failure exit
  code cannot distinguish a CVE from a network failure.
- **`testing/synctest` is stable stdlib** in Go 1.26 (GA since
  1.25; the experimental API is gone) — the ADR-0014 engine
  suites need no build tags. `go test -race` is compatible with
  the pure-Go store driver (modernc.org/sqlite). `testing.B.Loop`
  plus benchstat is the perf-harness idiom; `T.ArtifactDir`
  (1.26) fits harness output.
- **Custom analyzers:** wrap each in `go/analysis` with a single
  `multichecker` binary; `singlechecker`/`multichecker` binaries
  double as `go vet -vettool` plugins with no glue. Pin the
  binary as a module tool. This is the shape for the four design
  analyzers (technical design section 18 item 7).
- **deadcode and unparam remain current** (unparam via
  golangci-lint); mutation testing via gremlins is dropped — the
  tool is absent, the config was never portable, and the new
  test economy (ADR-0014) covers the intent better.

## Ecosystem state confirmed for pinning

- Charm v2 line is healthy and coordinated: bubbletea v2.0.8,
  bubbles v2.1.1, lipgloss v2.0.5, glamour v2.0.1, all on
  `charm.land/*/v2` module paths (hard rename, no shim; poplar's
  legacy go.mod already uses them, two patch bumps behind).
  goldmark stays v1 (v1.8.5); glamour itself pins goldmark v1.
  `ultraviolet` (the v2 renderer core) has no tagged release:
  track it transitively, never hand-pin.
- **teatest v2** lives at `github.com/charmbracelet/x/exp/teatest/v2`
  (not charm.land), still experimental, with an open Charm
  proposal for a successor test framework — the ADR-0014 named
  risk stands, goldens stay plain files via `x/exp/golden`.
- **bubbles/key + bubbles/help are the keybinding machinery**
  (unchanged shape in v2; `help.KeyMap`'s
  `ShortHelp()`/`FullHelp()` is exactly the derivation seam
  ADR-0011's registry needs). bubbletea v2 requests basic kitty
  key disambiguation by default — ADR-0012's relief valve is
  effectively the default behavior; only the extra enhancements
  (release events, repeat, alternate keys) stay declined. A bare
  Esc carries a fixed 50 ms ambiguity timeout in v2's input
  engine, not app-configurable — recorded against UX-8.
- **Mouse:** v2 exposes mouse as a per-frame `View.MouseMode`
  with four typed events; SGR negotiation automatic; no
  double-click detection (apps hand-roll, crush's 400 ms
  delayed-click pattern is the exemplar); hit-testing by
  component-owned `image.Rectangle` bounds (crush's pattern;
  bubblezone v2 documents a caveat against the lipgloss v2
  compositor). Mouse tests inject typed messages at the
  Update/teatest layer; terminal-level mouse injection has no
  usable tooling, so the tmux verification layer stays
  keyboard-driven.
- **Reference implementations catalogued** for the build:
  crush's `internal/ui/list/` (windowed list with version-keyed
  render cache and frozen entries — the closest existing solution
  to poplar's QA-5 list problem), its dialog Overlay stack,
  its `streaming_markdown.go` boundary detection (the honest
  cross-check for Catkin's incremental renderer, which remains
  unvalidated by precedent), and matcha (an active bubbletea v2
  email client; lazy-fetch list shape, mailbox UI conventions).
  Nothing in the ecosystem renders HTML mail or virtualizes a
  list over external storage; both stay poplar-built as designed.

## Remediation

Every fix lands through the build machine design and its
boundary ritual rather than as patches to the legacy machine:
the machine design doc specifies the new gate, agents, and
skills wiring; the boundary ritual deletes the dead and
legacy-bound artifacts listed here. Two items escalate outside
the repo: the workstation-level `elm-conventions` reference fix
(finding 9) and the `go-conventions` example-config fix
(finding 10).
