# Audit: Library Package Shape (filter / content / tidy)

> Run **after** the invariants/code-drift audit lands — that audit
> may turn up claims about these packages that need to be fixed
> first.

**Goal.** Decide whether `internal/filter/`, `internal/content/`,
and `internal/tidy/` are still well-shaped for their intended
poplar consumers, or whether they've ossified around assumptions
that no longer match where the project is going.

**Why now.** `invariants.md` flags these three as "library packages
awaiting their poplar consumers." Some were written ahead of need;
the consumers will land in:

- `content/` — already partially consumed by the viewer in Pass
  2.5b-4 (footnote harvesting via `RenderBodyWithFootnotes`). The
  question is whether the *rest* of the package fits the consumer
  it was built for.
- `filter/` — intended for triage automation, no consumer in
  STATUS.md yet.
- `tidy/` — intended for compose-time prose cleanup, scheduled for
  Pass 9.5 (tidytext in compose).

This is the last cheap moment to course-correct shape before
consumer code starts depending on the current API.

## Scope

For each of the three packages:

1. **Read the package** — every file, including tests and
   doc-comments. Build a mental API surface.
2. **Identify the intended consumer** — what pass plans to use it,
   and how. Cross-reference STATUS.md, BACKLOG.md, and the package's
   originating ADR.
3. **Evaluate shape fit** — does the current API match what the
   consumer will want? Are there abstractions that exist for
   possible-but-unlikely needs? Are there gaps the consumer will
   immediately hit?
4. **Verdict per package:**
   - **keep** — shape is right, no action.
   - **refactor** — shape is wrong; describe the change.
   - **collapse** — over-abstracted; flatten or simplify.
   - **inline-into-consumer-when-built** — package boundary doesn't
     earn its keep; let the consumer pass absorb it.
   - **delete** — built ahead of need that has since vanished.

## Out of scope

- Implementation correctness. This is an architecture/shape review,
  not a code review. Don't open bugs you find unless they break the
  package's stated contract; log via `log-issue` and move on.
- Performance. None of these run in a hot path yet.
- The `mailworker` and `mailjmap` packages — those have their own
  fork-vs-emersion question tracked as BACKLOG #10.

## Inputs to load

- `internal/filter/` (whole package)
- `internal/content/` (whole package — note the partial consumer
  in `internal/ui/viewer.go` and `account_tab.go`)
- `internal/tidy/` (whole package)
- `docs/poplar/invariants.md` — sections that touch any of the three
- `docs/poplar/STATUS.md` — for upcoming pass goals
- ADRs that introduced each package (use the decision index in
  `invariants.md` to find them; likely 0042-area for content,
  unspecified for filter/tidy)

## Method

A single Claude session on Opus 4.7. Work one package at a time.
For each, write the full verdict (a paragraph or two of reasoning,
plus the verdict label) before moving to the next. Do **not** try
to hold all three in working memory at once — they share little.

When evaluating "fit," ask the inversion: if you were writing the
consumer pass tomorrow with no library yet, would you reach for
this API? Or would you write something different?

## Done

A findings document at
`docs/poplar/audits/2026-04-25-library-packages-findings.md` with
one section per package:

- Current shape (one-paragraph summary of the public API).
- Intended consumer (which pass, how it'll use the package).
- Fit assessment (where the API matches and mismatches the
  consumer's needs).
- Verdict + recommendation.

## Follow-up

- **keep** verdicts → no-op.
- **refactor** / **collapse** verdicts → a small per-package pass
  before the consumer pass lands.
- **inline-into-consumer-when-built** → defer all action; mark the
  consumer pass plan to absorb the package.
- **delete** verdicts → a small docs+removal commit.
