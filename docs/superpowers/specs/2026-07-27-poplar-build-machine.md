# Poplar build machine

**Date:** 2026-07-27
**Status:** Revision 2, for the Phase 5 machine gate. Revision 1
was adversarially reviewed by two independent lenses
(spec-consistency attack, evidence-and-feasibility attack; ~40
findings, 9 blockers); this revision folds them.
**Charter:** `2026-07-19-poplar-refounding-charter.md` (Phase 5:
machine before build, designed against the real architecture).
**Binding inputs:** the technical design revision 2 and its ADRs,
the design language revision 2 (responsive grammar included), the
requirements' build-order spine (section 15), ADR-0014 (testing
strategy), and the Phase 5 tooling audit
(`docs/poplar/research/2026-07-27-phase5-tooling-audit.md`), whose
findings this design remediates. ADR-0017 (mouse) rides the same
gate, as does the requirements amendment in section 11.

The machine is everything that builds, checks, and reviews the
product without being the product: the gate, the analyzers, the
agents, the plan format, the verification harness, and the
boundary ritual that retires the legacy tree. The audit's verdict
stands: nothing carries forward by default. Each piece below is
either new against the settled architecture or named salvage.

## 1. The build boundary

Two commits: the first retires the legacy tree, the second
re-founds the module; `make check` runs green from the
re-founding commit onward. The ritual, in order:

1. **Create the `legacy` branch.** The branch every standing
   document cites does not exist (review finding, verified):
   only the `poplar-legacy` tag does, and it predates the Phase 1
   and Phase 4 spike tools, which live nowhere but master
   (`cmd/corpusharvest`, `cmd/renderspike`, `cmd/llm-render`,
   `cmd/perfspike`). Branch `legacy` is created from the final
   pre-boundary master commit and pushed, which makes the
   charter's, CLAUDE.md's, and the STATUS's claims true and
   preserves the spike tools — the QA-9 grading harness (pass 3)
   and the perf harness (pass 1) name them as salvage lineage.
   Salvage thereafter is copy-with-rewrite from `legacy`,
   reviewed like new code; no file crosses the boundary
   unreviewed (the no-migration-scars rule).
2. **Clear master.** Delete the legacy Go tree (`cmd/`,
   `internal/`, `go.mod`, `go.sum`), the dead machine
   (`scripts/audit.sh`, `scripts/check-deep.sh`,
   `scripts/voice-check.sh`, `scripts/modern-go-check.sh`,
   `scripts/build-wordlist.sh` — the spellchecker it feeds is
   archived-client scope no approved requirement carries —
   and `.golangci.yml`), the five `.claude/rules/*-invariants.md`
   files, all three PostToolUse hook scripts
   (`bubbletea-conventions-lint.sh`, `claude-md-size.sh`, and
   `elm-architecture-lint.sh`, which encodes the archived
   client's layering and fires on `internal/ui/**`) **and their
   registrations in `.claude/settings.json`** — deleting scripts
   while the settings file still invokes them breaks every
   Edit/Write in every agent session. Delete the stale hookify
   rule and `.claude/docs/tmux-testing.md` (superseded by
   section 6's harness). Legacy-only docs under `docs/poplar/`
   (`invariants.md`, `STATUS.md`, `wireframes.md`,
   `bubbletea-conventions.md`, `styling.md`, `keybindings.md`,
   `system-map.md`, `feature-matrix.md`, `v2-roadmap.md`,
   `release-stance.md`, `oauth-setup.md`, `audit-plan.md`,
   `audits/`, `testing/`, `decisions/`) leave master; `legacy`
   remains their home. Three deliberate survivors:
   `docs/poplar/research/` (the shared research home of both
   eras — `docs/poplar/responsive-design.md` moves into it and
   the design language's section 9 citation retargets, so no
   binding spec points at a deleted path), `BACKLOG.md` (the
   live tracker; its pre-re-founding entries stay caveated as
   CLAUDE.md already states), and the archived plans under
   `docs/superpowers/plans/` (immutable by standing rule; new
   pass plans are dated and coexist). `scripts/skipcheck` and
   `scripts/vale-comments.sh` survive as named salvage, the
   latter amended per section 2.
3. **Re-found.** Fresh `go.mod` (module `github.com/glw907/poplar`,
   `go 1.26`), the day-one Makefile and gate (section 2), the
   `tools/` module (section 2), the pinned dependency set
   (section 7), the CI workflow (section 2), the rewritten
   CLAUDE.md (build commands, conventions skills, the
   re-founding docs as the only binding docs), the rewritten
   `.claude/rules/poplar-development.md` (its current text cites
   the deleted invariants files and the retired poplar-pass
   skill), and the rewritten agents (section 4).
4. **Amend the specs the reviews found stale**, so build-pass
   implementers cite live text: the requirements advance to
   revision 4 carrying the Phase 4 gate's ratified numbers
   (QA-2 at 25 ms p95, QA-5 as store size at or under 1.6x
   retained body bytes, CO-6's debounce-plus-admission wording,
   the strengthened UX-3 analyzer rule) plus the UX-6 amendment
   (section 11) — today those live only in the STATUS record
   while the requirements still print the superseded numbers.
   ADR-0001 gains a revision block (one store file for all
   accounts, matching technical design section 3); ADR-0011
   gains one (`LayoutMode` carries per-pane rectangles; registry
   entries bind pointer targets alongside keys); ADR-0012 gains
   one (bubbletea v2 requests basic key disambiguation from the
   terminal unconditionally, so the relief valve is the shipped
   default; the decline now covers only the opt-in enhancements,
   and the goldens-representative consequence restates at the
   message layer, where inputs are normalized). The research
   copy of the retired voice catalogue
   (`docs/poplar/research/go-comment-voice.md`) gets its header
   stamped as an archived copy; it still names the dangling
   workstation path as binding.
5. **Escalate the workstation fixes** the audit found outside
   the repo, now correctly scoped (review finding): of
   `elm-conventions`' four `docs/poplar/` citations, three point
   into `docs/poplar/research/` and survive; the one that
   breaks is `docs/poplar/bubbletea-conventions.md`, whose
   content moves to a workstation home (`~/.claude/docs/`) with
   the skill repointed. `go-conventions`' example `.golangci.yml`
   updates to the v2 schema.

## 2. The quality gate

`make check` exists from the re-founding commit and every build
task clears it before review. One tier locally, no advisory
steps: a check either gates or does not exist (the audit found
what advisory guards decay into). CI runs the same gate verbatim
plus named depth jobs that are too slow, too network-dependent,
or too platform-bound for an every-commit local loop — depth is
scheduled, never advisory.

**Tool pinning.** golangci-lint's own documentation refuses the
go.mod `tool` directive and the tools pattern (dependency-graph
pollution; upstream recommends binary or isolated-module
installs — review finding, verified against the install docs).
The machine uses upstream's named escape hatch: a `tools/`
directory with its own `go.mod` pinning golangci-lint,
the analyzers binary's x/tools dependency, and deadcode; Makefile
steps invoke them via `go run -C tools`. The product `go.mod`
stays product-only, and `tidy-check` never re-tidies tool
dependencies. Vale is not a Go module: the workflow installs a
pinned release binary, and `vale-comments.sh` is amended to
hard-fail when Vale is absent (its current skip-quietly guard is
the same silent-decay class the audit condemned; "verified
working" on the workstation is not portability).

**The local gate:**

```
check:  tidy-check     go mod tidy leaves go.mod/go.sum unchanged
        build          go build ./... plus GOOS=darwin GOARCH=arm64
                       go build ./... (test compilation misses main
                       packages; C10 promises macOS builds)
        fmt-check      golangci-lint fmt (gofumpt + goimports), diff-clean
        lint           golangci-lint run, v2 config, default: none,
                       explicit enables: errcheck, govet, ineffassign,
                       staticcheck, unused, modernize, unparam,
                       misspell, gosec, nolintlint
        analyzers      the poplar multichecker (section 3)
        vale-comments  the Vale comment gate (salvaged, guard inverted)
        skipcheck      the unconditional-skip AST gate (salvaged)
        test           go test ./... — goldens, EXPLAIN QUERY PLAN,
                       grammar/registry/switch, contrast, unit and
                       synctest suites
        perf           go test -run 'QA[123]' -count=1, never under
                       -race: race instrumentation costs 2-20x time
                       and 5-10x memory, so a p95 asserted under it
                       measures the detector (review finding);
                       CI-tolerant thresholds gate here, gate-platform
                       numbers are recorded per pass by the same
                       harness run directly
```

The enable list is explicit because v2's `default: none` drops
errcheck, unused, and ineffassign unless named (review finding:
"the staticcheck set" implies none of them), and losing errcheck
on a project whose review lens is silent-failure hunting is not
an option. gosec ships with the `common-false-positives`
exclusion preset enabled and a triage policy: a `//nolint` must
carry the rule id and a reason, `nolintlint` gates the format,
and the pass-end reviewer reads every suppression added that
pass. Race coverage runs in CI (below), not in the local loop.

**CI**, one workflow from the first build pass (QA-10's
conventions-gate half):

- Per push: `make check` verbatim on Linux; `go test -race ./...`
  as its own job; a darwin job building and running the test
  suite, whose golden comparisons are QA-7's cross-platform
  byte-identity check.
- Nightly: govulncheck (moved out of the local gate — it is
  network-dependent, fails closed offline with an exit code that
  cannot distinguish a CVE from no Wi-Fi, and a newly published
  CVE turning an unchanged tree red mid-pass is a scheduled-job
  event, not a commit event; review finding), deadcode, the QA-5
  30-minute soak with its 5% RSS-growth bound, and the QA-8
  five-minute idle-CPU harness.
- The QA-6 kill harness scales with its subjects: a one-seed
  smoke run lives in `test` from pass 1 (store actions exist
  then); the script grows as compose, send, RSVP, and event
  edit land (passes 4-5); the full ADR-0014 scope (30 actions,
  200 SIGKILL points, three seeds) becomes a per-push CI job at
  pass 6, where the spine places it (review finding: revision 1
  demanded it from pass 1, when four of its seven actions have
  no subjects).

Mutation testing is dropped (audit: tool absent, config legacy,
ADR-0014 covers the intent). The voice system consolidates on
Vale plus the rewritten conventions skills; the dead T-catalogue
greps are gone with `voice-check.sh`, and `modern-go-check.sh`'s
job moved into the `modernize` linter.

## 3. The four analyzers

Technical design section 18 item 7, now owned. One binary under
`tools/analyzers` (wrapped with `multichecker`, which doubles as
a `go vet -vettool` plugin with no glue) carries four
`go/analysis` passes. Editor integration is separate work if
wanted (gopls loads no third-party analyzers); the gate is the
contract.

1. **import-boundary** — the section 2 dependency rules: `ui`
   never imports `backend`/`sync`/`outbox`; `catkin` imports no
   poplar package; `render`/`when`/`search`/`calendar` logic
   packages import neither store handles nor I/O packages. A
   package-level import fact, cleanly expressible.
2. **write-call** — ADR-0003's actual ask: store-write API calls
   construct only inside `internal/store`'s writer path, and
   nothing outside `internal/store` reaches a write entry point
   except through the intent types. Revision 1 also claimed "no
   `Exec` on read connections", which is not statically
   checkable (`database/sql` read and write handles are the same
   types, and name-matching is a heuristic sold as a gate —
   review finding). The type system owns that half instead: the
   store's read API returns a named read-only handle type that
   exposes no Exec methods, so a write on a read connection
   fails to compile. The analyzer keeps the package-boundary
   half only.
3. **styling (UX-3)** — positional, because revision 1's
   "reaches rendered output" wording is interprocedural taint
   and not decidable in `go/analysis` (review finding): outside
   `internal/theme` and `internal/catkin`, in non-test files, no
   rune or string literal containing a non-ASCII code point, no
   lipgloss constructor calls, no ANSI escape literals. An
   inline `//poplar:allow-unicode <reason>` escape exists for
   the legitimate non-theme cases the review named
   (`internal/render`'s entity handling, `internal/when`'s
   tokens, corpus fixtures); the gate counts escapes and the
   pass-end reviewer reads new ones. The numeric-spacing rule
   collapses into the constructor ban: spacing is reachable only
   through the theme's spacing-role API once constructors are
   banned, so it needs no separate check.
4. **error-construction (ER-1)** — `uerr` construction outside
   `internal/uerr` only through the exported constructor.

## 4. The agents

All three rewritten at the boundary against the settled
architecture; the audit retires their dead model pins, dead
catalogue references, and wrong gate descriptions.

- **poplar-implementer** (Sonnet). Implements one plan task,
  test-first, clears `make check`. Rewritten facts: the
  three-layer architecture and package map, the writer/intent
  mutation discipline (never write the store directly from UI
  code), the registry/keymap/theme contracts, the pointer rules
  (ADR-0017), the analyzer set and suppression policy it will
  be graded by, and the accurate gate. Keeps the
  DONE / DONE_WITH_CONCERNS / BLOCKED / NEEDS_CONTEXT report
  contract. Still invokes `go-conventions` before Go and
  `elm-conventions` before `internal/ui`.
- **poplar-reviewer** (`claude-opus-5`, high effort). Spec
  compliance against the task's acceptance criteria, then the
  quality lenses (reuse, simplification, correctness,
  silent-failure hunting). Gains the design-language and
  ADR-conformance lens — a diff that hardcodes a width, styles
  outside theme, or adds an unregistered key or pointer target
  fails review before the analyzers run — and the suppression
  audit (every new `//nolint` and `//poplar:allow-unicode`).
- **poplar-go-reviewer** (Sonnet). Convention pattern-matching
  against the rewritten skills and the Vale-era voice system;
  every T-catalogue reference gone. Dispatched in parallel with
  poplar-reviewer.

The in-repo `simplify` skill is rewritten in place: its voice
agent re-anchors on Vale plus `go-conventions`' principles (its
current first instruction reads a dead file), and its
gate-awareness table updates to section 2. The `poplar-pass`
skill and its templates retire with the legacy docs; the plan
format below replaces them. No new agents: tmux and visual
verification reuse the workstation `visual-verifier` pattern at
screen-pass gates rather than a bespoke agent.

## 5. Plan format and pass structure

The build follows the requirements' spine: six numbered passes
(foundation; design language and shell; mail read path; compose;
calendar; hardening). Each pass is one plan doc under
`docs/superpowers/plans/`, authored in the pass's execution
session from the spine and the specs, and executed
subagent-driven with per-task review.

Per-task plan format (supersedes the poplar-pass templates):

- **Requirement IDs** the task discharges (traceability is how a
  pass knows it is done; the spine maps every MUST to a pass).
- **Outcome**, stated as observable behavior, never
  implementation code.
- **Acceptance criteria**, each naming the test that proves it —
  written test-first by the implementer, and including the
  golden/analyzer surfaces the task touches.
- **Boundaries**: packages the task may touch, contracts it must
  not move (ADR references), and named non-goals.
- **Salvage pointers** where legacy or reference code exists
  (file on the `legacy` branch, or the catalogued exemplars:
  crush's windowed list, dialog stack, input-filter wheel
  coalescing, and multi-click state; matcha's fetch-message
  shape; the spike tools for the perf and grading harnesses).

Screen passes add the design ritual: a text wireframe per screen
per responsive class that changes its layout (design language
section 9), pointer targets included (ADR-0017), reviewed by
Geoff before the screen's tasks dispatch. Every screen pass's
close checklist carries one manual real-terminal item: the
screen's pointer vocabulary exercised by hand on the gate
terminal, because terminal-level mouse injection has no tooling
(audit finding). Pass ends run consolidation (simplify, reviewer
fan-out, STATUS update, plan archival) and close at a pass gate
with Geoff.

## 6. Verification harness

Four layers, from cheapest to most real, per ADR-0014 (its full
inventory carried here; revision 1 dropped items the review
restored):

1. **Pure-logic tables and fixture corpora** (render rules, JWZ,
   parsers, iTIP, recurrence): with their packages from pass 1
   on; the license-clean specimen corpus lands in pass 3.
2. **The golden matrix**: teatest v2 plus `x/exp/golden`, per
   screen state, per capability profile, per responsive class
   and boundary sizes (design language section 9's testing
   clause), floor state included. Goldens are plain files;
   teatest's experimental status stays a named risk with a
   mechanical swap path. Scripted keystroke tests cover the
   optimistic-paint criteria (LT-2); mouse behavior is tested
   here and in scripted Update tests by injecting typed mouse
   messages — click-to-cursor, double-click timing windows,
   wheel coalescing, drag selection — never terminal-level.
3. **Store and engine harnesses**: transaction tests,
   migration-from-N-1 tests, the randomized mutation-search
   script with its closing FTS5 integrity check, SY-8's three
   failure tests (forced corruption, failed migration, full
   disk), the QA-6 kill harness at its pass-scaled scope with
   the FTS5 integrity check inside its restart assertions,
   synctest scenario suites over the scriptable backend fake
   (the seam's second implementation) including the QA-4
   convergence trials under virtual time, idempotent-replay per
   intent kind, EXPLAIN QUERY PLAN goldens, and the QA-1/2/3
   perf harnesses from pass 1 — `testing.B.Loop` plus benchstat
   as the idiom, artifacts via `T.ArtifactDir`, baselines
   recorded at first list render, never under `-race`
   (section 2). The QA-5 soak and QA-8 idle harnesses run
   nightly in CI.
4. **Real-terminal checks**: a tmux script layer
   (`scripts/tmux-check`) driving the built binary for smoke
   flows (launch, list, read, compose, quit) on the gate
   platform, keyboard-only by tooling necessity; the manual
   pointer checklist item per screen pass (section 5); and the
   live-account tagged suite, never in CI.

The QA-10 artifacts close the loop as pass-6 gate outputs:
`internal/ui` package documentation, the README, and the
architecture map at 1.0 — named here so no pass ends with them
unowned.

## 7. Dependency pins, day one

From the C9 survey (versions verified 2026-07-27) and audit
verification: `charm.land/bubbletea/v2` v2.0.8,
`charm.land/bubbles/v2` v2.1.1, `charm.land/lipgloss/v2` v2.0.5,
`charm.land/glamour/v2` v2.0.1, goldmark v1.8.5 (v1 line;
glamour agrees), modernc.org/sqlite v1.54.0,
git.sr.ht/~rockorager/go-jmap v0.5.3, emersion/go-webdav v0.7.0,
jhillyerd/enmime/v2 v2.4.1, emersion/go-message v0.18.2,
emersion/go-msgauth v0.7.0, x/net v0.57.0, chroma/v2 v2.27.0,
adrg/xdg v0.5.3, zalando/go-keyring v0.2.8, godbus/dbus/v5
v5.2.2, esiqveland/notify v0.14.0, gofrs/flock v0.12.1,
`_ "time/tzdata"`. teatest/v2 and `x/exp/golden` ride x/exp
pseudo-versions (no tags exist; recorded, not hand-pinned).
golang.design/x/clipboard v0.8.0 enters only if the clipboard
spike passes. `ultraviolet` is tracked transitively only (no
tagged release). Tools module: golangci-lint v2 (v2.12.2 or
current at boundary), govulncheck v1.6.0 (the audit's v1.1.3
citation was a stale-year error; corrected there), deadcode,
and the analyzers binary. Patch bumps ride each pass's start; a
render-set bump regenerates goldens and triggers the QA-9
regrade (technical design section 9).

## 8. Input machinery, settled

The registry (ADR-0011) is implemented over `bubbles/key`
bindings; each screen's registry entry implements
`help.KeyMap`'s `ShortHelp`/`FullHelp`, and the footer, help
overlay, grammar test, and switch-table test all derive from
those bindings — the audit confirmed this is both the canonical
v2 machinery and the production pattern (crush). Registry
entries also bind pointer targets (ADR-0017), and `LayoutMode`
carries per-pane rectangles so panes resolve clicks without a
zone library — both recorded as the ADR-0011 revision block
(section 1 step 4), not asserted as existing fact. Wheel input
runs through a program-construction `tea.WithFilter` coalescer
(~16 ms sample window, signed accumulation, direction reset —
crush's production pattern), because an unfiltered wheel burst
is one store round-trip per tick against QA-2's budget; the
filter is a recorded elm-conventions exception (pure coalescing,
no model state) that `elm-conventions` gains alongside the
boundary's skill fixes. The disambiguation and Esc-timeout facts
land as the ADR-0012 revision block, and bare Esc's fixed 50 ms
ambiguity window is recorded against UX-8's leave-field model.

## 9. Session conduct

Per the workstation model economy: this machine design ends the
Fable sitting at the gate; each build pass runs in a fresh
execution session (Opus 5 conductor), dispatching
poplar-implementer per task, reviewing each diff, running the
reviewer fan-out at pass end, with Geoff at wireframe reviews and
pass gates only. Pass plans are authored in the execution session
from the spine; Fable returns for taste forks, post-mortems, and
any hedge an Opus verdict flags.

## 10. Carried obligations, scheduled

- **Clipboard spike** (gate box: raw-mode coexistence,
  compositor coverage, static-binary fit): during pass 2, before
  the platform package hardens; OSC 52 posture ships regardless.
- **CalDAV RSVP and free/busy probes**: when the calendar-scoped
  token lands; default branch stays poplar-sends-iMIP. Needed
  before pass 5 closes, not before it starts.
- **go-ical/golang-ical bake-off plus rrule DST fixtures**:
  opens pass 5.
- **Column formulas re-derived from the spike harvest**: opens
  pass 3, with the wireframes.
- **QA-9 grading harness**: pass 3, with the specimen corpus;
  salvage lineage is the Phase 1 spike tools on `legacy`.
- **Catkin incremental-renderer spike**: added by this design —
  the audit found no precedent anywhere for live incremental
  markdown editing (crush's streaming layer is append-only);
  pass 4 opens with a bounded spike before the editor tasks
  dispatch.

## 11. What the gate is asked to rule on

1. This machine design as a whole (sections 1-10), the boundary
   ritual's `legacy`-branch creation included.
2. ADR-0017 and the requirements amendment it needs: UX-6
   ("Mouse basics", today a SHOULD) amends to MUST with the
   ADR's pointer vocabulary as its scope — revision 1 of both
   documents asserted a keyboard-only requirements base and
   silently re-rated the work (review finding); the honest form
   is this named amendment, landing in the same requirements
   revision 4 that applies the Phase 4 ratified numbers
   (section 1 step 4). Drag-select in the reader stays SHOULD
   inside it.
3. The gate composition (section 2): dropping mutation testing,
   consolidating the voice system on Vale, govulncheck and
   deadcode as nightly CI rather than commit-gates, the gosec
   suppression policy, and race coverage in CI rather than the
   local loop.
4. The spec amendments of section 1 step 4 (requirements
   revision 4; revision blocks on ADR-0001, ADR-0011, ADR-0012).
5. The workstation-level fixes (bubbletea-conventions.md
   relocation and skill repoint; go-conventions config example).

After approval: the boundary ritual executes, the machine
artifacts land (Makefile, tools module, configs, analyzers,
agents, CLAUDE.md, CI), `make check` passes green on the empty
module, and pass 1 (foundation) begins in a fresh execution
session.
