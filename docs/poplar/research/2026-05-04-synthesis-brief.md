# Go Comment Style Guide — Synthesis Brief

Inputs: the four files under `sources/` produced by parallel research
agents on 2026-05-04.

Output: `docs/poplar/research/2026-05-04-go-comment-voice.md`.

## Scope

Not just voice. The guide covers **placement, density, and detail
level** alongside voice and phrasing. The end consumer (Claude
writing Go for poplar and other projects) needs to answer three
questions for every potential comment:

1. **Should this comment exist at all?** (placement + circumstance)
2. **How long and how detailed?** (density appropriate to context)
3. **What voice and shape?** (sentence structure, tone, vocabulary)

A guide that answers only #3 leaves #1 and #2 underdetermined,
which is how AI overcommenting persists.

## Required structure

### 1. Decision rubric (top of document)

A short, mechanical flowchart for "should I write a comment here?"
that an agent can apply without re-reading the whole guide. Inputs:
placement category + circumstance flags. Output: yes/no + rough
length target.

### 2. Placement × circumstance matrix

Rows (placement):
- Package doc (`// Package foo …`)
- Exported type (struct, interface, alias)
- Exported function / method
- Exported const / var
- Struct field (exported and unexported)
- Unexported symbol (type, func, var)
- Inline mid-function `//` comment
- TODO / HACK / FIXME / BUG markers
- Test function name / table-case label
- Error string (`errors.New`, `fmt.Errorf`)

Columns (circumstance modifier):
- Trivial / self-evident
- Tricky behavior or surprising default
- Public API contract
- Concurrency hazard
- Security note
- Workaround / known limitation
- Platform / version-specific
- Deprecation
- Cross-package coupling that's hard to discover from code

Per cell: comment? target length? what to include? what to omit?
1–3 verbatim positive examples (cite source file:line).

### 3. Density expectations by file type

A page that says, roughly: a wire-protocol parser has different
healthy comment density than a UI renderer, which differs again
from a data-struct definition file or a test file. Quantify with
examples ("net/http/request.go has ~X comments per 100 LOC; this
is the upper end of healthy density for protocol code").

### 4. Voice palette

The 3–5 voice archetypes the essays-and-proverbs agent extracts
(Pike-terse, Cheney-conversational, stdlib-formal, Charm-warm,
HashiCorp-formal). Pick **one** as poplar's target with
justification, and note the conditions under which we'd shift
register (e.g., "package docs lean slightly more inviting; internal
helper comments lean terser").

### 5. Phrasing patterns

- Sentence shape: imperative? declarative? subject-first?
- Length distribution to target (range, not fixed).
- Vocabulary register (when "we" is OK, when "you" is OK, when
  third-person is required).
- Hedging policy ("maybe", "should probably" — almost never).
- Punctuation conventions inherited from godoc (period required,
  paragraph breaks, em-dash usage).

### 6. Error string phrasing

A dedicated subsection because this is the loudest AI tell.
Patterns from stdlib + Cheney essay; rule for `%w` vs. `%v` vs.
bare; rule for redundant context.

### 7. AI tells — catalogue and avoidance

A first-class section. The threat model is contributor recruitment:
readers who recognize AI-shaped Go disengage. This section names
every tell we can identify, shows what each looks like, and pairs
it with the human equivalent.

For each tell:
- **Name** (memorable, e.g., "the `failed to X: %w` chorus").
- **Where it appears** (placement category from §2).
- **What it looks like** — verbatim AI-style snippet (1–3 lines).
- **Why a reader spots it** — the cue (uniformity, hedging,
  redundancy, defensive padding).
- **Human equivalent** — verbatim excerpt from stdlib/Charm
  research showing how an experienced author handles the same
  situation.
- **Mechanical avoidance rule** — what Claude should do
  differently at write-time (not "use judgment" — a concrete check).

Tells to cover at minimum (synthesis may add more from research):

**Comment tells:**
1. WHAT-comments restating the next line.
2. Godoc on every unexported symbol regardless of need.
3. Comment density uniform across functions of wildly different
   complexity.
4. Hedge phrases: "for now", "note:", "TODO" without an issue link.
5. Task-framing comments: "added for the X flow", "used by Y",
   "fixes #N".
6. First-person plural ("we") used reflexively in unexported docs.
7. Sentence shape uniformity — every doc beginning "Foo does X."
8. Multi-paragraph docstrings on functions whose name + signature
   is self-describing.
9. Per-case docstrings on every table-test case.

**Error-phrasing tells:**
10. `fmt.Errorf("failed to X: %w", err)` template applied
    uniformly across unrelated call sites.
11. Adjacent error sites in one function reading identically when
    the failures are genuinely different.
12. Redundant context: error includes the function name the caller
    already knows.
13. Bare `%w` wrapping where the caller doesn't branch on the
    sentinel.

**Naming tells:**
14. `GetX` getter prefix.
15. Package-doubled types (`mail.MailMessage`).
16. `Manager` / `Helper` / `Util` / `Service` suffixes on
    single-field types.
17. Over-descriptive locals in tight scopes.
18. Exported names that read like docstrings.

**Structural tells:**
19. Reflexive `doc.go` / `errors.go` / `types.go` skeleton in
    every package.
20. Single-impl interfaces with no test fake, no DI seam, no ADR.
21. `New<X>` constructors that only set fields a struct literal
    would set.
22. Builder patterns where a literal would suffice.
23. Defensive nil checks between two functions in the same package.
24. Length checks before indexing on internal callers.

**Test tells:**
25. Identical assertion phrasing copy-pasted across files.
26. Tautological cases (function returns argument unchanged →
    case asserting that).
27. Subtests for trivial scalar functions.
28. Case `name:` fields written as full sentences.

**Voice tells:**
29. Uniform sentence length distribution within a file.
30. Identical rhythm — every paragraph the same shape.
31. Apologetic or hedging documentation ("This may not handle
    every case…").
32. Over-explanation of standard Go idioms.

This list is the core checklist that flows into:
- `/simplify` voice lens — agent scans diff for these by name.
- `go-conventions` skill — same list, with the avoidance rules.
- ADR-0138 — codifies the policy.

### 8. Anti-patterns catalogued by placement

For each placement row in the matrix, name the dominant AI failure
mode:
- Unexported helpers — overcommenting trivial code
- Struct fields — godoc on fields whose name is self-documenting
- Table-test cases — full-sentence `name:` fields with embedded
  documentation
- Error strings — uniform `failed to X: %w` across unrelated sites
- Inline mid-function — restating the next line in prose
- Package docs — boilerplate "Package X provides Y" with no
  information beyond the import path

Each anti-pattern paired with a positive counter-example from the
research.

### 8. Positive-example pool

10–20 verbatim excerpts (with citation) of comments the guide
holds up as exemplary. These do double duty: they teach voice and
they seed ADR-0138's example pool.

### 9. How this binds going forward

The §7 AI-tells catalogue is the load-bearing artifact — it's the
named, mechanical list code standards reference. Both poplar's
`Human voice` rules and the global `go-conventions` skill must
carry the catalogue (or a stable reference to it) in full, not a
paraphrase. Paraphrasing decays into "use judgment" within two
revisions.

Specific propagation:

- **`go-conventions` skill** (global, mandatory before writing
  Go) — append a "Human voice & AI tells" section that mirrors
  §5–§7 of this guide. Include the full §7 catalogue inline with
  the mechanical avoidance rule per tell. This skill is invoked
  before every Go file edit, so the tells list is in-context at
  write-time, not just at review-time.
- **`/simplify` voice lens** — the new fourth parallel reviewer
  scans the diff against the §7 catalogue by name. Each finding
  cites the tell number and the avoidance rule. Catches drift
  that slipped past write-time.
- **CLAUDE.md `Human voice` section** (poplar) — keep the
  short-form rules there; add a pointer to the full guide and
  the §7 catalogue. Path-scoped rules don't replicate the full
  catalogue, just point to it.
- **ADR-0138** — codifies the policy as a binding decision,
  references the guide and the skill update as the enforcement
  mechanism.
- **Cross-project reuse** — the `go-conventions` skill update
  benefits every Go project on this workstation, not just poplar.
  The §7 catalogue should be written generic (no poplar-specific
  examples in the skill copy; poplar examples stay in the guide
  and ADR-0138).

## Non-goals for the synthesis

- Don't invent examples — every excerpt traces to a source file.
- Don't replicate the C1–C8 anti-pattern list verbatim from the
  Pass 10.5 spec. The matrix supersedes the flat list; the spec
  gets updated to point here.
- Don't legislate beyond what the sources support. Where sources
  disagree, name the disagreement and pick a side with reasoning.
- Don't write rules that require taste calls without offering a
  decision rubric. "Use judgment" is the failure mode this guide
  fixes.
