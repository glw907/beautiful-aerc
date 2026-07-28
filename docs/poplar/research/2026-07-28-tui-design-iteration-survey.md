# Adjudication: poplar's design-tooling proposal against six angles of field evidence

Date: 2026-07-28. Inputs: six parallel research dispatches covering Charm's own repos, design-forward
Go TUIs, the Rust/ratatui ecosystem, size-and-breakpoint practice, and how terminal UI work actually
reaches a human reviewer in 2026. Three follow-up dispatches then answered questions the original
sweep left open: whether any maintainer reviews a regenerated golden diff as a design decision, which
motion-recording tool to use, and which golden library to stand on. Reader: whoever authors and
executes poplar's pass 2 plan.

Throughout, claims are labeled by the weight they carry. A claim that **the field supports** is backed
by a cited artifact in a real repository. A claim that is **poplar's own judgment** rests on this
project's constraints and has no field precedent behind it. The distinction matters, because two of
the strongest recommendations below are local judgment calls.

## Verdict

The proposal is right in shape and wrong in three of its four load-bearing dependencies. Keep it,
amend it hard.

What survives contact with the evidence: fixture-driven rendering of the real components through the
real theme, a shared fixture set feeding both the review artifact and the test oracle, a systematic
sweep across sizes, and wireframes first. Every one of those has a working analog in a real project.

What does not survive:

1. **teatest v2 as the golden engine.** This is the single most dangerous commitment in the current
   design, and the proposal adds a third consumer to it. The field's only two documented abandonments
   are both teatest abandonments, and one of them was caused by exactly the bubbletea v2 migration
   poplar is standing on.
2. **VHS tapes for the four timing behaviors.** No project in six angles uses VHS as a development or
   review tool. It is a marketing-GIF pipeline everywhere it appears. The four behaviors named
   (optimistic paint, a 10-second undo window, wheel coalescing, an Esc ambiguity window) are
   assertable with a virtual clock and unassertable by watching a GIF. A dedicated tool comparison
   also finds VHS the wrong choice for the one job it might have kept, the v1.0 README demo. See
   amendment E.
3. **Simulated rungs inside your host terminal as the size oracle.** The field gets size confidence
   from layout math tested as a pure function of width and height, with named boundary cases. A
   simulated viewport is fine for eyeballing composition. It cannot verify pointer coordinates, glyph
   widths, or the emulator's own wrapping, which are three things poplar has specifically committed
   to getting right.

The gallery target, the least emphasized item in the proposal, is the highest-value piece and should
be built first. Nothing in the field supports it. The argument for it is entirely local, and it is
strong on its own terms.

One thing the evidence cannot see, and which changes the calculus more than any pattern in it: every
surveyed project's reviewer is a maintainer who runs the app himself. Poplar's builder is an agent
that cannot see a terminal and cannot take a screenshot. The field's dominant review artifact is
unavailable here. That absence, not any positive precedent, is the real justification for building a
harness at all.

---

## 1. Where the proposal is wrong or unsupported

### 1.1 teatest v2 is the riskiest thing in the design, and the proposal leans harder on it

The proposal makes the fixture table feed "the teatest golden matrix." poplar's build machine already
commits to that engine at section 6, layer 2, naming teatest's experimental status as a risk with a
"mechanical swap path." The evidence says the risk has already landed on two projects.

- `gh-dash/internal/tui/ui_test.go` and `gh-dash/internal/tui/components/tabs/tabs_test.go` have every
  teatest test commented out, including the import statement. The `.golden` fixtures are still
  committed under `tabs/testdata/TestTabs/*.golden` and are now orphaned. Two angles independently
  attribute this to the migration to `charm.land/bubbletea/v2` while `charm.land/x/exp/teatest` lagged.
  poplar is on bubbletea v2.
- `bubbletea/examples/simple/main_test.go` has both of its own teatest tests out of compilation. One
  carries "TODO: Enable this test again." The other is skipped as flaky with the note that "we need a
  more concrete way to set the initial terminal size." Setting the initial terminal size is precisely
  what poplar's rung matrix would do on every case.
- `grep -rl teatest` over crush returns nothing. The flagship bubbletea application, poplar's named
  exemplar, tests its UI with `golden.RequireEqual` on rendered strings
  (`crush/internal/ui/diffview/diffview_test.go`) and with direct struct construction at a fixed size
  (`crush/internal/ui/model/layout_test.go`, `newTestUI` hardcodes width 140, height 45). It never
  starts a program.

The pattern across the three is consistent. `charmbracelet/x/exp/golden` is load-bearing and widely
adopted: `golden.RequireEqual` writes and compares `testdata/<Test>.golden`, escapes control
sequences, diffs with go-udiff, regenerates under `-update`, and is used by bubbles, lipgloss,
glamour, crush, and freeze. `teatest`, the layer that boots a `tea.Program` against a simulated
terminal, is the fragile part and is the part nobody depends on.

The proposal does not distinguish them. It should. Static screen renders do not need a running
program.

### 1.2 VHS is a documentation tool everywhere it exists, and cannot assert what the proposal wants it to

Five separate angles converge on this. Nobody watches a tape in a dev loop and nobody gates on one.

- Charm's `examples/` directories across bubbletea, lipgloss, huh, and gum pair each example 1:1 with
  a `.tape` whose only output is a README GIF hosted at vhs.charm.sh.
- `vhs` itself has no watch or live-reload mode. It is a batch script interpreter. Its Dockerfile and
  `go.mod` show what the batch actually is: it shells out to `ttyd`, drives a headless Chromium
  through `github.com/go-rod/rod`, and post-processes with ffmpeg. Render time is minutes because a
  real browser is painting xterm.js frame by frame.
- No repo in any angle asserts a rendered GIF or frame against a golden. VHS does ship a `-test` mode
  with a `Golden` field in `testing.go` that dumps the raw terminal buffer as text (`README.md`
  lines 814-822). Nobody outside VHS uses it, and it captures the plain buffer rather than the GIF
  that is the actual deliverable.
- `bottom/demos/demo.tape` documents the real cost in its own comments: multi-minute renders, then
  manual resizing and optimization to hit a 2-3MB GIF target.
- `superfile/vhs/*.tape` all set 1920x1080 or 2560x1500 for cinematic demos. `ratatui/examples/vhs/`
  commits its GIFs to an orphan images branch. lazygit stores demo assets on a dedicated `assets`
  branch to keep main history light.

VHS is not abandoned, and no claim here rests on it being so. `gh api repos/charmbracelet/vhs`
reports 753 commits with the latest on 2026-06-27, a push on 2026-07-24, and 167 open issues. Its
headless behavior is the fragile part: open issues #755 ("VHS freezes when I do vhs test.tape"), #754
(freeze on Windows cmd), #721 (blank frames, canvas renderer fails in headless Chrome), and #715, a
request that VHS be "skillized" for AI-automated TUI testing, which confirms it is not that today.

Then the decisive point. The four behaviors the proposal assigns to VHS are timing and state-machine
behaviors, and a recording asserts none of them. A GIF cannot tell you the undo window is 10 seconds
rather than 9.4. A virtual clock can, deterministically, in milliseconds of wall time. poplar already
owns this machinery: the build machine's layer 3 runs synctest scenario suites under virtual time,
and ADR-0017 already rules that mouse behavior including double-click windows and wheel coalescing is
tested by injecting typed mouse messages, never at terminal level. Wheel coalescing and the Esc
ambiguity window are already spoken for by a ruling poplar has ratified. Routing them to VHS
contradicts ADR-0017.

There is a residual case for a recording, and it is small. Motion feel is a taste question, and taste
questions go to Geoff. One recording, once, near the README, off the critical path. Amendment E names
the tool, and it is not VHS.

### 1.3 A gallery binary has no precedent, and the one honest attempt is unwired scaffolding

State this precisely, because the temptation is to overstate it. No project abandoned a component
gallery. No project built one either.

- `atuin/crates/atuin-ai/` ships `test-renders.json`, a file of named UI states with descriptions,
  and `render-tests.sh`, which calls a `debug-render` subcommand for each. That subcommand does not
  exist in `commands.rs` at that commit, which lists only `Init` and `Inline`. This is the closest
  thing in the entire survey to the proposal's fixture-table-plus-gallery shape, and it does not run.
- ratatui's widget showcase lives in a different repository
  (`ratatui-website/code/showcase/widget-showcase/`), takes a `--widget` enum flag, and exists to be
  screenshotted by VHS one widget at a time for the docs site. It is a batch screenshot binary.
- Charm's `examples/` dirs and huh's 44 runnable examples are documentation source. Each is a
  standalone `go run` main paired with a tape. No maintainer launches a gallery to iterate.
- The harnesses that genuinely survived are heavy. lazygit's `pkg/integration/` drives the real
  binary against real git repos with a scripted robot user, run via `just e2e`. superfile's
  `testsuite/` launches the real `spf` binary inside tmux via libtmux and asserts on captured pane
  content. television's `tests/common/mod.rs` spawns the real `tv` binary in a PTY through
  phantom-test. **zellij belongs in this list, not in the component-render list.** Its 46 `.snap`
  files under `src/tests/e2e/snapshots/` are produced by `remote_runner.rs`, which opens a real SSH
  PTY, spawns the compiled binary, feeds the output through a VT parser, and asserts with
  `insta::assert_snapshot!`. zellij does not depend on ratatui at all and has zero `TestBackend`
  usages. All four test the compiled product end to end. None isolates a component.

The honest reading: the field's cheap harnesses die and its expensive ones live, and neither category
is a gallery. That is a warning about durability, not a proof of uselessness. Section 5 explains why
poplar is the case where it pays anyway, and section 6 restructures the proposal so the durable part
is the cheap part.

### 1.4 Simulated rungs are not a size oracle

The proposal simulates 60/80/100/140/200 columns inside whatever terminal you happen to have. The
field's size confidence comes from somewhere else entirely.

- `lazygit/pkg/gui/controllers/helpers/window_arrangement_helper_test.go` passes width and height as
  plain ints to `WindowArrangementHelper` and asserts an exact ASCII box diagram of the resulting
  panel arrangement. No terminal, no PTY, no rendering. Its portrait-mode table names cases at exactly
  the threshold and one unit past it, width 50 versus 49, height 20 versus 21, with names like
  "disabled because width is too large."
- `crush/internal/ui/model/layout_test.go` constructs the `*UI` struct at width 140 and height 45 and
  calls the resize hook directly.
- Where the field does sweep, it sweeps headlessly in a for loop. `crush/internal/ui/diffview` produces
  `WidthOf001` through `WidthOf110` goldens. lipgloss's table package produces `HeightOf01` through
  `HeightOf25` plus width-shrink series. These are the two clearest size sweeps in the whole survey and
  both are hand-rolled per test file. No shared helper exists in `x/exp/golden` or teatest.

Three things a simulated viewport inside a larger terminal actively cannot check, all of which poplar
has committed to:

- Pointer coordinates. ADR-0017 binds pointer targets to pane rectangles from `LayoutMode`. A
  simulated 80-column viewport drawn at an offset inside a 200-column terminal reports click
  coordinates in host space. Getting hit-testing right in the simulator and wrong in the app is a
  live failure mode, and the mouse is a MUST in requirements revision 4.
- Cell widths. Ambiguous-width and emoji glyphs render at a width the host terminal's font decides.
  Simulation inherits the host's answer and hides the target's.
- The emulator's own wrapping and scrollback behavior at the real edge.

The cheap real alternative is already planned. `scripts/tmux-check` drives the built binary on the
gate platform, and tmux resizes a pane to an exact geometry on demand. A resize escape (`\e[8;24;80t`)
does the same in one line on kitty. Simulation should not be the authority when the real thing is a
shell command away.

Second gap in the same item. poplar's ladder is two-dimensional. Design language section 9 defines
height classes: under 20 rows compresses chrome, below 15 rows is the floor state, and grid calendar
views degrade by name below their declared rows. The proposal's rungs are columns only. A
column-only matrix will not exercise the floor at 80x14 or the chrome-compression boundary at 19
versus 20 rows.

### 1.5 freeze for design-doc stills is right for poplar and wrong as a reading of the field

The evidence on what reaches a human reviewer is unusually clean, and it does not say freeze.

- Every merged PR sampled across five projects attaches a static PNG from the OS screenshot tool via
  GitHub user-attachments, usually a before/after pair. lazygit #5711, bottom #2162 (a rendering
  regression fix), gh-dash #918 and #626, superfile #1524.
- PR templates that say "Images/Videos" or "screenshots or recordings" (gh-dash, bottom) get only the
  still-image half in practice. `superfile/CONTRIBUTING.md` lines 40-76 codifies a per-state screenshot
  checklist for theme PRs and says "screenshot," never "recording."
- freeze's own repo is the one place a rendered fixture doubles as a reviewed artifact:
  `freeze/test/golden/svg/*.svg` are diffed against generated output with go-udiff via
  `freeze_test.go`, regenerated by `make golden`, and `README.md` embeds `shadow.svg` directly as a doc
  image. That is a good pattern. It is freeze testing freeze, not any project using freeze to review.

So the proposal's freeze item has no field precedent as a review pipeline. It is still correct for
poplar, for a reason no surveyed project shares. See section 5.

### 1.6 No maintainer anywhere reads a regenerated golden diff to decide how something should look

This finding came from a dedicated follow-up dispatch, and it contradicts an assumption the earlier
draft of this document carried into its own recommendation. Record it plainly, because the gallery
recommendation in section 6 has to be rebuilt on top of it.

The question asked was narrow: does any maintainer across crush, lipgloss, bubbles, gitui, ratatui,
television, superfile, lazygit, or insta use a regenerated `.golden` or `.snap` diff as the medium for
judging "yes, that is the look I want"? The answer is no. Every deliberate visual-change PR that
surfaced routes the look-judgment through a separately pasted before/after PNG. The golden diff in the
same PR is discussed, when it is discussed at all, as regression-lock bookkeeping.

- **lipgloss #526** (`fix(table): fix some table rendering bugs`) carries six before/after PNG pairs in
  the body as the design-judgment artifact. `bashbunni`: "Confirmed all test results look good, code
  is way tidier." `table/testdata/TestTableHeightShrink` and siblings changed in the same PR. The one
  inline comment anchored on a `.golden` file (`TestTableShrinkWithYOffset.golden`) is `andreynering`
  arguing a behavioral edge case, whether the table should be forced to show more records than asked.
  That is a correctness question about row counts. It is the closest thing found to the target
  phenomenon and it is still not it.
- **lipgloss #671** (`fix(table): prevent columns from shrinking to zero width`) states in its body:
  "Updated golden files that previously encoded the buggy behavior." The single review is an approval
  with an empty body.
- **bubbles #823** (viewport refactor): `meowgorithm` writes "it looks like the golden files need to be
  updated. Do you mind doing that?" That is a mechanical instruction. The approvals from `caarlos0`
  and `aymanbagabas` never reference golden content.
- **crush #136** and **crush #3034** (a Charmtone bump touching 100 files, including dozens of
  `internal/ui/diffview/testdata/**/DarkMode.golden` and `LightMode.golden`) are both stated as
  mechanical consequences of a dependency change. #3034 has zero comments and zero reviews. A
  whole-palette golden regeneration merged with no discussion.
- **crush #2607** (`feat: generally render output that looks like a diff as a diff`) judges the actual
  feature through two pasted PNGs labeled Before and After. `andreynering`: "Nice idea!" The golden
  fixtures underneath are never mentioned.
- **gitui #2411 and #2813** add the project's only insta snapshots under `src/snapshots/*.snap`.
  Inspection of the committed files confirms they do capture the rendered box-drawing layout, so the
  artifact could serve a design loop. The discussion between `cruessler` and `extrawurst` is entirely
  about flakiness, timing races, and app layering. No comment reads a snapshot to judge how something
  looks.
- **superfile #1114** (Everforest Dark Hard theme) and **#1442** (a sidebar aesthetics fix) carry only
  PNGs. superfile has no rendered-output fixtures at all; **#1384** asserts numeric layout structs in
  the lazygit style.
- **ratatui and television** returned nothing. `gh search prs` and a code search for `extension:snap`
  found no insta snapshot changed as part of a deliberate look decision. ratatui-core does not carry
  `insta` in its top-level `Cargo.toml`.
- **insta's own docs** frame `cargo insta review` generically ("check if the result is okay"). That is
  generic-data framing, and no PR in a terminal-UI consumer of insta applies it to a visual judgment.

The search covered `gh search prs` and `gh pr list` across all nine repos, PR bodies, reviews, and
inline diff-anchored comments through `gh api`, full commit histories for gitui's `src/snapshots/`,
crush's `internal/ui/diffview/`, and freeze's `test/golden/`, plus code search and the insta docs. The
absence is reported as an absence, not inferred from a thin sample.

The consequence for this document: the gallery-diff-as-review-medium idea has zero field support. It
survives on poplar's own constraints alone. Section 6, amendment B says so in those words.

### 1.7 A shared fixture table is right, and "table" is the wrong noun

The coupling risk is real. atuin's `test-renders.json` is the shape and it is dead. gh-dash's goldens
outlived the code that produced them. When one data file feeds three consumers, the flakiest consumer
governs its schema, and the file becomes something that must be edited for every UI change.

poplar has already settled the analogous question in the other direction. Themes are compiled Go
values rather than config files, following Charm convention. Fixtures should be Go values in an
internal package for the same reasons: the compiler catches drift when a model field changes, no
parser is needed, and no schema negotiation happens between consumers.

### 1.8 Two smaller unsupported claims

"The harness turns an approved wireframe into something drivable the same day." Only after the
components exist. At wireframe time in a screen pass there is nothing to render. The sketch binary
verifies a built screen. It does not produce a pre-build mockup, which is what Geoff asked for.
Section 6 amendment H fixes this with task ordering rather than tooling.

"Excluded from the shipped app." Say how. A `cmd/sketch` in the same module is still built by
`go build ./...`, still linted, and still inside the styling analyzer's scope. That last part is good
and should be stated as an invariant, because a gallery binary is the natural place for someone to
reach for a raw lipgloss call. The build machine's UX-3 rule bans lipgloss constructors, non-ASCII
literals, and ANSI escapes outside `internal/theme` and `internal/catkin` in non-test files.
`cmd/sketch` and the fixtures package are non-test files, so they are already covered. Confirm the
analyzer runs over `cmd/` and keep it that way.

---

## 2. Where the proposal is right, and what backs each piece

**Render the real components through the real theme with fixture data.** This is the core idea and it
is well supported. `crush/internal/ui/diffview/diffview_test.go` renders the real component and
compares with `golden.RequireEqual`, with dedicated narrow cases (`TestNarrow`, `SmallWidthFunc` at
width 40, `LargeWidthFunc` at 120). ratatui's `TestBackend` in `ratatui-core/src/backend/test.rs`
renders the real widget into an in-memory `Buffer` and asserts on it. gitui snapshots the real
`TestBackend` render through insta into `src/snapshots/*.snap`. Note the shape of that support
honestly: both Rust instances run on ratatui's own `TestBackend`, so the evidence covers one Go
framework and one Rust framework. No claim is made here that the pattern is renderer-agnostic. The
earlier draft rested that claim on zellij, which turned out to run a PTY end-to-end harness with no
ratatui dependency at all (see 1.3).

**A size sweep as a first-class artifact.** `crush/internal/ui/diffview/testdata/WidthOf001..110` and
lipgloss's table `HeightOf01..25` are the two working precedents. Angle 4 states the gap plainly:
"No project runs VHS as a breakpoint matrix; that is a gap poplar could fill, not a pattern to copy."
Poplar's ladder is documented, discrete, and small, which makes the sweep cheap and the artifact
readable.

**One artifact serving both the test oracle and the human review.** freeze's README embedding
`test/golden/svg/shadow.svg` is the single instance of a golden doubling as a reviewed visual
artifact anywhere in the survey, and it is freeze testing itself. The gallery output is poplar's
version of it. Read section 1.6 before leaning on this precedent, because it is one instance and the
dedicated follow-up found no second.

**Enumerating states explicitly rather than trusting ad hoc checks.** `superfile/CONTRIBUTING.md`
codifies each panel focused, the help menu, popups, image preview, and command success and failure.
The sampled superfile PR (#1524) attaches a still per state including the error path. A named fixture
per state is the same discipline made mechanical.

**A single explicit floor state.** superfile's `terminalSizeWarnRender` and lazygit's Limit view both
render one whole-app too-small message. crush has no app-wide floor at all and handles smallness with
independent per-widget clamps, which angle 4 reads as the weaker choice. poplar's floor state under 60
columns and under 15 rows already matches the better pattern, and the proposal is right to include the
floor in the matrix.

**Wireframes first.** Nothing in the evidence contradicts it, and nothing in the evidence replaces it.
Keep it as the gate it already is in the routing rule.

**Excluding the harness binary from the product.** Standard everywhere. Charm's examples, poplar's
existing `tools/` module. No objection beyond stating the mechanism.

---

## 3. The field's dominant pattern, stated plainly

For iterating on terminal UI design in 2026, across Go and Rust, the pattern is:

Run the real application in a real terminal and look at it. Test layout as a pure function of width
and height with no terminal in the loop, using named boundary cases at N and N+1. Commit plain-text
ANSI goldens of rendered components, diff them with go-udiff, regenerate them with `-update`, and let
CI gate on them as a regression lock. When a human needs to see a change, paste a static
before-and-after PNG taken with the OS screenshot tool. Keep VHS for the README.

The regression-lock wording is deliberate. Section 1.6 establishes that the goldens are never the
medium for the look decision. They catch unintended change; the PNG carries the intended one.

Nobody has a Storybook. The two projects that wanted more built a full integration framework driving
the compiled binary with a scripted user (lazygit's robot user, superfile's tmux harness,
television's PTY sessions through phantom-test, zellij's SSH-PTY snapshot runner) and reused that same
framework to record their demos. lazygit's demo GIFs are recordings of its own integration tests
marked `IsDemo:true`.

Visual reputation implies nothing about visual test infrastructure. k9s, aerc, glow, sesh, and yazi
have no preview harness, no golden tests, and no automated capture pipeline. glow is a Charm project
with zero VHS wiring of its own.

---

## 4. What is missing from the proposal that the evidence says matters

**Golden churn policy.** A screen-by-state-by-rung matrix multiplied by theme variants means one
spacing change rewrites hundreds of files. That is the exact condition under which goldens become a
rubber stamp that gets regenerated without being read. Section 1.6 shows that is not a hypothetical
failure: crush #3034 merged a hundred-file golden regeneration with zero comments. Nothing in the
proposal says who reads the diff, what a reasonable churn budget per task is, or how a regeneration is
reviewed. gh-dash's orphaned `testdata/TestTabs/*.golden` are what the end state looks like.

**Determinism inputs.** gitui's insta configuration filters temp paths and hashes out of snapshots
because otherwise they never match twice. poplar's fixtures must pin the clock, the timezone, message
IDs, unread counts, and any sort that touches time. The build machine already imports `time/tzdata`
for the QA-7 darwin goldens. The fixture set has to carry the same discipline or the matrix flakes on
day two.

**The theme dimension.** poplar bans styling outside `internal/theme` and treats the design language as
a spec. The proposal's matrix has screen, state, rung, and capability profile, and no theme axis. ANSI
escapes survive into `x/exp/golden` output, so text goldens do capture color, and a theme change is
exactly the kind of change a human should look at rather than diff. This is where freeze earns its
keep.

**An explicit escape path off teatest.** The build machine promises a "mechanical swap path" and does
not describe it. Amendment A below is that path, and it should be written down before the matrix
exists rather than after it breaks.

**Height and boundary coverage.** Covered in 1.4. The matrix needs rows as well as columns, and it
needs N versus N+1 at every threshold, following lazygit's naming.

**Where the gallery output lives.** If gallery text files are committed they are a second full matrix
next to the goldens, doubling churn. If they are not committed, their diff cannot be reviewed and the
review artifact has no history. Pick one deliberately. Recommendation in section 6, amendment B, on
local grounds only.

---

## 5. What poplar's situation changes

**A single developer working through an agent, with no contributors.** This is the override that
matters most. Every surveyed project's review artifact is a PNG taken by a human with an OS
screenshot tool, and every one of those projects has a reviewer who can also just run the app. Neither
is available here. The agent doing the building cannot see a terminal, cannot screenshot one, and
cannot judge whether a render looks right. The gallery's text output is the only artifact the agent
can produce, read back, and put in front of Geoff. The field's harness-free default is not a choice
poplar can inherit, because the thing that makes it work is missing.

It also explains the field's absence of galleries in a way that argues for poplar building one. OSS
maintainers do not need a component gallery because they run their app all day and their reviewers
run it too. Poplar's reviewer wants accurate mockups before screens exist, produced by a builder who
cannot see.

**Styling banned outside the theme package by an analyzer.** This kills the standard failure mode of
example galleries, which is demo code drifting from application code. `cmd/sketch` cannot restyle
anything. Whatever it renders is what the app renders. That makes poplar's gallery more trustworthy
than any `examples/` directory in the survey, and it is the strongest structural argument for keeping
the sketch binary.

**Three rungs plus a floor, with formulas computed once into one `LayoutMode` struct.** Design language
section 9 has already built lazygit's `WindowArrangementHelper` in all but name. The single highest
value size test in poplar is a table over `LayoutMode` at boundary widths and heights, asserting pane
presence and rectangles, with no rendering at all. That test is cheaper, faster, and more exact than
any render matrix, and the field says it is where real confidence comes from. The render matrix sits
on top of it and catches composition, not arithmetic.

**bubbletea v2 with teatest v2 goldens.** poplar is standing exactly where gh-dash was standing when
its suite died. Treat teatest as the fragile dependency it is and keep the blast radius to a few
tests.

**Mouse as a MUST with pointer targets bound to pane rectangles.** Simulation is disqualified as the
pointer oracle. ADR-0017 already routes pointer testing to injected typed messages, and the manual
per-screen pointer checklist on the gate terminal covers the rest.

**A reviewer who wants accurate mockups before screens get built.** No tool solves this. Ordering
does. If the theme, `LayoutMode`, and the chrome primitives (status line, footer, switch bar, pane
frames) land before the first screen's behavior, an approved wireframe can be composed into a real
static screen from real primitives with fixture content and no model. That composed render is the
accurate mockup, it is produced by the same seam as everything else, and it becomes the screen's first
golden when the behavior lands.

---

## 6. Recommendation

Keep the proposal. Amend it as follows. The shape is right and the objections above are load-bearing
rather than decorative, so this is an amendment rather than a replacement.

### A. Make a pure render function the single seam, and demote teatest

Every consumer calls one function of the form `Render(screen, state, LayoutMode, theme) string`, a
pure composition with no program and no I/O. Then:

- Static screen goldens go through `x/exp/golden.RequireEqual` on that function's output. No teatest,
  no `WithInitialTermSize`, no program lifecycle. This is exactly crush's diffview pattern.
- teatest v2 is reserved for a deliberately small suite of keystroke and flow tests where a running
  program is genuinely required (the optimistic-paint criteria under LT-2, the quit path, focus
  traversal). If teatest breaks on a bubbletea bump, poplar loses a handful of tests instead of its
  entire visual regression net.
- Write the swap path the build machine promised into the pass 2 plan as one paragraph: what replaces
  teatest if it lags, and which tests are affected.

This is the most important amendment. Do it first.

**Keep `charmbracelet/x/exp/golden` as the golden library.** A dedicated comparison against
`sebdah/goldie` and rust-analyzer's `expect-test` settled this, and the reason is a real feature
rather than familiarity. `x/exp/golden/golden.go` (at commit `41c9e6b`) runs `escapeSeqs` over both
sides before diffing, `strconv.Quote`-ing each line so raw ANSI control codes appear as visible
`\x1b[...]` text in a failure diff instead of corrupting the terminal. Its doc comment states the
intent directly at lines 21-24: golden files "can contain control codes and escape sequences," and
`RequireEqual` escapes them before comparing. goldie has no equivalent. `grep -rni
"ansi\|escape\|terminal" goldie/*.go` returns only the string "ColoredDiff", a diff-highlighting
option (`goldie/interface.go:50-52`, `goldie/goldie.go:137`), and a GitHub issue search across
sebdah/goldie for ansi, color, or terminal returns one unrelated hit. goldie itself is healthy (264
stars, pushed 2025-11-22, three open issues), and it is solving a different, ANSI-naive problem.

Adoption checks came back empty in a way worth recording. No major non-Charm Go TUI golden-tests its
rendered output with goldie. k9s carries `sebdah/goldie/v2 v2.8.0` in `go.sum` only, transitively, and
no k9s source file imports it. lazygit and lazydocker have no goldie and no golden-file usage outside
vendored color libraries. A code search for `"goldie"` plus `".View()"` returns 15 hits, all small
repos.

The `x/exp` instability is real and should be named by the maintainers' own words. `x/README.md`:
"This repository contains experimental packages with no promises of backwards compatibility." The
commit history for `exp/golden/golden.go` shows one signature-widening change in roughly 16 months
(2025-06-02, `RequireEqual` going generic over `[]byte | string`, with the byte-only entry point kept
as a deprecated `RequireEqualEscape`), plus escape-sequence handling in May 2024 and a Windows
line-ending fix in February 2025. One file, about 80 lines, a stable core API for a year, consumed by
bubbletea, `x/pony`, and `x/exp/teatest`. Pin it and bump it deliberately.

`expect-test`'s inline-literal shape was considered and rejected for poplar. Its `expect!["literal"]`
macro rewrites the expectation in place inside the test source, and its own docs push toward
`expect_file!` once output gets verbose (`expect-test/src/lib.rs:1-90`). poplar's goldens are full
rendered screens of 20 to 80 lines with box drawing and ANSI. A named `testdata/*.golden` file is a
cleaner diff unit for both readers poplar has, the human and the agent. That last sentence is
poplar's own judgment, not a field finding.

### B. Build the gallery target first, and make the sketch binary a thin wrapper

The gallery is a for loop over fixtures and rungs calling the seam from A and writing text files. It
is nearly free, it is the artifact Geoff reads, and it is the artifact the agent can read back.
`cmd/sketch` becomes a small bubbletea model that calls the same seam with keys to cycle the axes.
Keep it, because feeling a screen is not the same as reading it, and because Geoff will use it. Do not
let it be the deliverable that the fixtures and the sweep depend on.

Commit the gallery output, and understand exactly what that decision rests on. **This is poplar's own
judgment and carries no field weight whatsoever.** Section 1.6 establishes the opposite of a
precedent: across nine repositories, no maintainer has ever been found reading a regenerated golden
or snapshot diff to decide how something should look. The field's design-loop artifact is a pasted
PNG, every time, and its goldens are approved without their content being read.

The justification is local and it is sufficient. poplar's builder is an agent that cannot run the
app, cannot see a terminal, and cannot paste a screenshot. A committed text render is the only
artifact both the agent and the human can read, and committing it is the only way its change over
time becomes reviewable at all. Every surveyed project could choose the PNG because a human with eyes
was already running the binary. That option does not exist here. The gallery diff is not a better
review medium than a screenshot. It is the only one available.

Two consequences follow from the same reasoning. Keep the teatest and static golden sets small enough
that there are not two full matrices in the tree; where the gallery covers a case, do not also golden
it. And treat the field's regeneration behavior as the failure mode to design against, which is what
amendment G is for.

### C. Restructure size verification into three tiers

1. A `LayoutMode` boundary table with no rendering, over columns and rows, at every threshold and one
   unit either side. Name cases the way lazygit does. Cover 59/60, 79/80, 99/100, 139/140 on columns
   and 14/15, 19/20 on rows, plus the calendar view's declared-row degradation.
2. Gallery renders at each rung and at the boundary sizes, floor state included, at 80x24 explicitly.
3. Real resize through `scripts/tmux-check` on the gate terminal for the human check, plus the
   existing per-screen manual pointer checklist.

The simulated rung inside `cmd/sketch` stays for composition eyeballing. Write one sentence in its
help text saying it does not verify pointer coordinates or glyph widths. Do not let a plan task claim
a size is verified because it looked right in the simulator.

### D. Fixtures as Go values, not a data file

`internal/ui/fixtures` (or equivalent), exported Go values, one named fixture per screen state.
Compiler-checked, refactor-safe, no schema. Pin the clock, the timezone, IDs, and anything else
non-deterministic in the package itself. The styling analyzer covers it, and it must not become the
place where a raw escape sequence sneaks in.

### E. Cut VHS entirely, and use asciinema when a recording is genuinely wanted

The four named behaviors go to message-level tests with a virtual clock, which is what ADR-0017
already requires for two of them. Optimistic paint is already LT-2's scripted keystroke test. The
undo window is a fake-clock assertion. Nothing in pass 2 records anything.

When a recording is wanted, a dedicated tool comparison says VHS is the wrong tool for both remaining
cases.

**For the v1.0 README demo: `asciinema rec`, then `agg` to GIF.** GIF is the only format with no
embedding risk in GitHub markdown. agg (`asciinema/agg`, Rust, 315 commits, latest 2026-06-01, tags
through v1.9.0) is a purpose-built `cast → frames → gifski` converter with no browser, no ttyd, and no
ffmpeg in its dependency chain. VHS's pipeline for the same one-time asset is ttyd plus headless
Chromium through go-rod plus ffmpeg (`vhs/Dockerfile`, `vhs/go.mod`), which is why its renders take
minutes and why `bottom/demos/demo.tape` instructs its own users to hand-optimize the result down to
2-3MB. agg's committed `demo.gif` is 4.4MB from a 1.76MB cast, so the size class is the same and the
saving is entirely in render time and dependencies. Two caveats to carry forward. agg has live glyph
fidelity bugs, most closed (#115 emoji treated as one column, #108 gaps between full-width characters,
#105 Braille replaced with question marks) and one open as of 2026-07-19 (#119, inconsistent rendering
of synthetic powerline and mosaic symbols). And agg's output determinism was not verified, because no
Rust toolchain was available in the checking environment.

**For showing a reviewer a timing-sensitive interaction during development: `asciinema rec` and
`asciinema play`, with no rendering step at all.** The `.cast` file is the literal terminal byte
stream with timestamps, replayable at any speed in any terminal, with `--idle-time-limit` to cut dead
air. asciinema is actively maintained (3.2.1 on 2026-06-16, pushed 2026-07-28). This is cheaper than
every alternative because there is no conversion. If the interaction has to go to someone who cannot
run `asciinema play`, convert that one cast with agg.

**Do not use termsvg, and do not treat its SVG as a committable artifact.** The determinism premise
was tested directly and falsified. The binary was built and `termsvg export` was run five times on
`examples/htop.cast`, producing five different md5 hashes. The cause is in
`pkg/renderer/svg/renderer.go`, which builds CSS class names by iterating `c.rec.Colors.All()`, a Go
map with randomized iteration order, so the class-to-color assignment shuffles between runs even
though the pixels are identical. A second, larger cast happened to render identically five times
because it uses too few distinct colors for the ordering to matter, which shows the defect is latent
and input-dependent. Two further disqualifiers: the SVG is emitted as a single line with no newlines
(`wc -l` on `examples/444816.svg` returns 0), so it is not line-diffable without a pretty-printer of
your own, and that same file is 2.09MB. Its terminal emulation depends on `hinshun/vt10x` pinned to a
March 2022 pseudo-version, materially staler than asciinema's own `avt` or VHS's xterm.js. termsvg is
genuinely fast (0.03 to 0.47 seconds per export, the only one of the three that could be benchmarked
directly) and that does not rescue it.

Rationale in one line: no project in six angles uses any recording in a dev loop, a GIF cannot assert
a timing window, and for the one-off README asset asciinema plus agg does in one CLI call what VHS
does with a browser.

### F. Keep freeze, scoped to theme and design-doc stills

freeze renders gallery text into a still with real colors. Use it for theme review, for design-doc
figures, and for anything where color fidelity is the subject. Commit the outputs the way freeze
commits `test/golden/svg/`, and embed them in the design docs the way freeze's README embeds
`shadow.svg`. Plain text stays the default review medium because it diffs, and freeze is the exception
for color.

### G. Add a golden and gallery churn policy

State in the pass 2 plan: the regeneration command, that a regenerated diff is read rather than
waved through, and roughly how much churn a task should produce. Add churn to the pass-end reviewer
fan-out's scope. Add the theme axis to whatever the matrix ends up being, or state explicitly that
theme variants are reviewed as freeze stills rather than goldened.

This amendment is where the section 1.6 finding bites hardest, and the policy should be written with
that finding in view. The field's own maintainers approve regenerated goldens without reading them,
in projects where a human is separately looking at a PNG. poplar has no PNG and no second look. The
read-the-diff rule is therefore not a nicety here, it is the only inspection that happens. Stating it
as a rule is poplar's own judgment; the field offers no example of a project that enforces one.

### H. Order pass 2 so an accurate mockup is possible before behavior exists

Theme, `LayoutMode`, and the chrome primitives land before the first screen's model. That makes the
wireframe-to-mockup step real: an approved wireframe gets composed into a static render from real
primitives with fixture content, reviewed by Geoff, and then becomes the screen's first golden. This
is the amendment that actually delivers "accurate mockups before screens get built," and it is task
ordering rather than tooling.

### Cost note

A through D and H are small. The gallery is a loop, the fixtures are values, the boundary table is a
table, and H is sequencing. E removes the most expensive item in the proposal. F is a shell out to an
existing binary. The amended proposal is cheaper than the original and depends on less.

---

## 7. What this research does not establish

Read this section before quoting anything above as settled.

**The golden-as-design-loop angle failed to return in the original sweep.** The six parallel
dispatches covered Charm's repos, design-forward Go TUIs, the Rust ecosystem, size practice, and the
human review path. None of them directly asked whether a golden diff is ever the medium for a design
decision. Section 1.6 was reconstructed after the fact, from the other angles plus one dedicated
follow-up across nine repositories. The follow-up's answer is a clean no with citations, and it is
still one dispatch rather than a converging set of independent ones.

**Absence of evidence is doing real work in three places.** No gallery binary was found. No maintainer
reading a golden diff for design was found. No non-Charm Go TUI golden-testing rendered output with
goldie was found. Each of those is a searched absence with the search method recorded, and none of
them is a proof that the thing does not exist somewhere unsearched.

**agg's output determinism is unverified.** No Rust toolchain was available, so agg was never built or
run. Its architecture (a pure cast-to-frames-to-gifski pipeline with no interactive component) makes
byte-stable output plausible, and plausible is all this document can claim. termsvg's determinism was
falsified by direct build-and-run testing; agg's was not confirmed by anything. If the README GIF is
ever regenerated in CI and diffed, verify agg first.

**VHS render times are documented by its consumers, not measured here.** The multi-minute figure comes
from `bottom/demos/demo.tape`'s own comments and the surrounding hand-optimization instructions. No
VHS render was timed in this research.

**The theme axis has no field data at all.** Section 4 names it as a gap, and no surveyed project runs
a theme-crossed render matrix. The freeze-stills recommendation in F is entirely poplar's own
judgment.

**Nothing here was tested against poplar's actual code, because pass 2 has not started.** The seam in
amendment A, the `LayoutMode` boundary table in C, and the fixtures package in D are designs, not
verified implementations. The claim that the styling analyzer already covers `cmd/` and the fixtures
package is read from the build machine's UX-3 rule and should be confirmed against the analyzer's
actual package scope before the plan relies on it.

**Sample size on the PR evidence.** The review-artifact finding rests on merged PRs sampled across
five projects for section 1.5 and nine for section 1.6. Both sampled deliberately toward
visual-change PRs, which is the right bias for the question and still a bias. No repository's full PR
history was read.

**Orchestrator verification (2026-07-28).** Section 7 asks that the
styling analyzer's package scope be confirmed before the plan relies
on it. Confirmed against `tools/analyzers/styling/styling.go`. The
exemption is role-based: `pkgrole.Of` resolves the package path, and
only the `theme` and `catkin` roles return early. A package with no
recognised role, `cmd/sketch` included, is analyzed. The structural
argument in section 5 holds.

One refinement the survey did not catch. The analyzer skips every
`_test.go` file, so fixtures written as test-file values fall outside
it. Amendment D's fixtures must live in a non-test package to stay
under the same gate as the code they render.
