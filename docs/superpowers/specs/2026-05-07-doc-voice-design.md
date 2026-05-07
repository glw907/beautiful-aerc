# Doc voice: extend voice policy to public docs

Date: 2026-05-07
Status: Draft

## Problem

Poplar's voice policy (ADR-0141) covers Go source. Project Claude
infrastructure and public docs are nominally in scope ("voice rules
apply to Claude-authored docs too"), but enforcement leans Go-only:
`scripts/voice-check.sh` scans Go files, `/simplify`'s voice agent
runs on Go diffs, the catalogue in `go-comment-voice.md` calibrates
to comments and godoc.

Two gaps compound:

1. **Generation drift.** Claude reads `CLAUDE.md`,
   `docs/poplar/invariants.md`, the auto-loaded rules, and the
   skills it invokes on every turn. If those documents read
   AI-flavored, the draft they prime reads AI-flavored. Cleaning
   the inputs is more efficient than catching the outputs at
   review time.
2. **Public signal.** When the repo goes public at v1.0, prose
   under `docs/poplar/` ships with the binary. Em dashes and
   "comprehensive" headers in invariants and ADRs disqualify the
   project from reading like a thoughtful Go programmer wrote it.

The goal is to extend voice policy to all repo prose, calibrate
the positive palette per genre, and replace the bespoke grep gate
with a tool the OSS world already standardizes on.

## Decisions settled in brainstorm

- **Plan B over Plan A.** Tighten the gate first; let normal
  passes drain the backlog. No big audit pass.
- **Scope.** Everything voice-load-bearing in the poplar context:
  `docs/poplar/**`, `CLAUDE.md`, `.claude/rules/`,
  `.claude/skills/poplar-pass/`, `.claude/docs/`, active
  `docs/superpowers/plans/` and `specs/`. Archived plans and
  research docs are not rewritten. The catalogue extraction in
  `~/.claude/docs/go-comment-voice.md` is in scope because it's
  voice-load-bearing for this project even though it lives global.
- **Genre awareness.** Four genres with different natural voices:
  reference, tutorial, narrative, imperative. One global tone
  forces narrative ADRs into reference shape or vice versa.
- **Vale, not bespoke grep.** The agent's prior-art research
  surfaced Vale as the dominant pattern in serious docs-first OSS
  (CockroachDB, Grafana, Datadog, Meilisearch). Go-native, regex
  YAML rules, format-aware (Markdown and Go scopes), composable,
  no project in the survey targets AI tells mechanically. The
  field is open.
- **Em dashes are the loudest tell.** Community Go technical
  docs almost never use them. Replacements: period, comma,
  parens, colon.

## Architecture

```
docs/poplar/
  STYLE.md                     (NEW; the human-readable guide)
    §1 Voice & tone            Google-style named section
    §2 Genre playbooks         reference / tutorial / narrative / imperative
    §3 Mechanics               punctuation, headings, lists, code blocks
    §4 Word list & AI tells    catalogue with replacements

.vale.ini                      (NEW; root config; scans .md and .go)
.vale/
  config/
    vocabularies/Poplar/
      accept.txt               allowlist (Pike, Gerrand, lipgloss, etc.)
      reject.txt               extra-strict denylist
  styles/Poplar/
    AITells/                   one rule per catalogue entry
      Comprehensive.yml
      Ensure.yml
      Robust.yml
      Leverages.yml
      ItsWorthNoting.yml
      ...
    ProseTells/                prose-only rules
      EmDash.yml
      SemicolonClauseJoiner.yml
      RuleOfThree.yml
      CalloutTic.yml
      SignpostingNarration.yml
    Mechanics/
      SentenceCaseHeadings.yml
      InlineCodeForSymbols.yml

~/.claude/docs/
  go-comment-voice.md          (existing; positive palette stays;
                                the catalogue section retires in
                                favor of STYLE.md §4 plus the
                                Vale ruleset, single source)

Makefile                       make check runs vale ./.
                               voice-check.sh retires in the same commit
```

The catalogue lives once: in `STYLE.md §4` for humans, as Vale
rules for the gate. Each Vale rule's `message` field cites the
STYLE.md section so a `make check` failure points the contributor
at one row in one document.

## STYLE.md outline

### §1 Voice & tone

One screen. Anchors on a single sentence: a knowledgeable Go
programmer talking to peers. Three or four short paragraphs:

- Confidence without hedging.
- Concrete over abstract: name the type, the file, the function.
- Short sentences. Subordinate clauses earn their place.
- "We" appears in ADRs (decision narrative). Reference and
  tutorial prose addresses the reader as "you" or no-person.

This section sets the reading we measure against. It does not
enumerate rules.

### §2 Genre playbooks

Four sub-sections, ~150 words each. Each names: what files it
governs, dominant tense and person, three positive moves, three
pitfalls.

- **Reference.** Files: `invariants.md`, `keybindings.md`,
  `styling.md`, `system-map.md`. Declarative, present tense, no
  first or second person. Tables and lists carry the load.
  Pitfall: narrating what the section will do.
- **Tutorial.** Files: `bubbletea-conventions.md`,
  `responsive-design.md`. Instructional, second person OK,
  examples woven in. Each rule has a why. Pitfall: padding with
  transitions.
- **Narrative.** Files: ADRs, `docs/poplar/research/*`.
  First-person plural, past tense for context, present for the
  decision. Decision then rationale. Pitfall: hedging the
  decision after stating it.
- **Imperative.** Files: `.claude/rules/*`, skill `SKILL.md`,
  `CLAUDE.md` rule sections. Commands. "Do X. Don't Y." No voice
  in the literary sense. Pitfall: explanatory prose creeping in.

### §3 Mechanics

Punctuation rules first.

- **Em dash (—)**: do not use. Replacements: period, comma,
  parens, colon.
- **En dash (–)**: ranges only (`80×24`, `1990–2000`). Not as a
  sentence break.
- **Semicolon**: tables, lists, parallel constructs. Not as a
  clause-joiner. Break into two sentences.
- **Colon**: introduce lists or definitions. One per paragraph.
- **Parens**: short asides. If longer than the surrounding
  sentence, it's a sentence.
- **Ellipsis (…)**: omitted content in quotation. Not for
  trailing thoughts.

Structure rules: heading capitalization (sentence case), list
parallelism, code-block language tags, inline code for symbols.

### §4 Word list & AI tells

Two-column table: tell to replacement (or rephrase guidance).
One row per tell. Rows grouped: rhetorical signposting, hedge
words, padding adjectives, AI-tic phrases, code-comment-only
tells. Each row links to the Vale rule that catches it.

Closing paragraph: the catalogue evolves. New tells log here and
add a Vale rule in the same commit.

## Vale ruleset shape

`.vale.ini`:

```ini
StylesPath = .vale/styles
MinAlertLevel = warning
Vocab = Poplar

[*.md]
BasedOnStyles = Poplar

[*.go]
BasedOnStyles = Poplar.AITells, Poplar.Mechanics
```

Go scope is narrow on purpose: AI tells in code comments still
fire (T16, T27, T33), prose-only mechanics (heading case,
em-dash) skip Go files.

Representative rule:

```yaml
# .vale/styles/Poplar/AITells/Ensure.yml
extends: substitution
message: "Tell #27. 'Ensure' is AI-flavored. Use 'make sure', 'check', or restate as the action."
level: error
ignorecase: true
swap:
  ensure: make sure
  ensures: makes sure
  ensured: made sure
  ensuring: making sure
```

```yaml
# .vale/styles/Poplar/ProseTells/EmDash.yml
extends: existence
message: "Em dashes read as AI-generated. Replace with period, comma, parens, or colon. STYLE.md §3."
level: error
tokens:
  - '—'
```

### Severity policy

- **error**: em-dash, T16, T27, T33, T35. `make check` fails.
- **warning**: softer tells with legitimate uses (rule-of-three,
  semicolon clause-joiner). Surfaces in `/simplify` and review.
- **suggestion**: heading case nits, sentence-length hints.

### Allowlist mechanics

When a flagged term has a legitimate use:

1. `accept.txt` for project-vocabulary terms (proper nouns,
   library names).
2. Fenced code block. Vale skips by default.
3. `<!-- vale Poplar.Ensure = NO -->` ... `<!-- vale Poplar.Ensure = YES -->`
   for narrow exceptions. STYLE.md §4 uses this to quote tells
   without recursive flagging.

## Pipeline integration

### Makefile

```make
VALE := vale
VALE_PATHS := docs/poplar docs/superpowers/plans docs/superpowers/specs \
              CLAUDE.md .claude/rules .claude/skills internal cmd

vale:
	@command -v $(VALE) >/dev/null || { \
	  echo "vale not installed. brew install vale | go install github.com/errata-ai/vale@latest"; \
	  exit 1; \
	}
	$(VALE) --minAlertLevel=error $(VALE_PATHS)

check: fmt-check vet vale test
```

`vale` slots between `vet` and `test`. Sub-second on a tree this
size. `scripts/voice-check.sh` is deleted in the same commit. No
compat shim. The Go-source tells in voice-check.sh migrate to
`Poplar.AITells/`.

### `/simplify` voice agent

The fourth review agent currently scans diffs against the
catalogue. After this lands, it has two sources:

- Vale's report on the diff (`vale --output=JSON` on changed
  files): the mechanical layer.
- STYLE.md §1 and §2: the squishy layer Vale can't see
  (sentence rhythm, false confidence, genre-mismatched voice).

Brief gets a one-line update: treat Vale findings as floor, not
ceiling.

### ADR

One ADR records the policy extension and tooling choice. Three
sections:

1. *What changes.* Voice policy expanded from Go source to all
   repo prose. Genre-aware (four playbooks). Vale becomes the
   mechanical gate for both Go and Markdown.
2. *Why now.* Repo goes public at v1.0; prose ships with the
   binary. Generation-time drift wastes tokens. One tool is
   cheaper than parallel mechanisms.
3. *Migration.* `voice-check.sh` retires. STYLE.md ships. Ten
   Vale rules ship with the pass; the rest land in subsequent
   passes.

ADR-0141 gets a one-line back-reference.

## Cleanup scope (Plan B)

The pass that lands this work fixes Vale `error`-severity
findings on the in-scope tree. One-time backlog drain.

- All `error` findings across `docs/poplar/`, `CLAUDE.md`,
  `.claude/rules/`, `.claude/skills/poplar-pass/`,
  `.claude/docs/`: fixed.
- Em-dash sweep across the same tree: fixed.
- ADRs in `docs/poplar/decisions/`: fixed for errors. ADRs are
  edited in spirit only when revisiting; warnings deferred.
- Archived plans (`docs/superpowers/archive/`): not touched
  (archived plans are immutable).
- Active `docs/superpowers/plans/` and `specs/`: fixed for
  errors. These trees are in `VALE_PATHS` going forward, so
  every new spec and plan ships under the same gate as the rest
  of the repo. The spec you are reading now is a working example:
  brainstorm-time drafting drifted toward em dashes, and a
  pre-Vale self-check caught them. With Vale in `make check`,
  the pre-commit gate catches them automatically.
- `docs/poplar/research/*`: fixed for errors. Dated artifacts.
  We remove tells, we don't rewrite voice.

`warning`-severity findings stay. They get fixed when files get
touched, per the existing fix-inline rule.

## Testing

Each rule ships with a fixture: one good example, one bad,
named alongside the rule (`Ensure.test.md` next to `Ensure.yml`).
A `vale-test` make target runs Vale against fixtures and asserts
the bad cases flag, the good cases don't:

```make
vale-test:
	@for f in .vale/styles/Poplar/**/*.test.md; do \
	  $(VALE) --output=line "$$f" || true; \
	done | scripts/check-vale-fixtures.sh
```

Lightweight. Catches regressions when rules tighten or relax.

## Pass shape

One pass, twelve tasks (right at the budget ceiling):

1. Spec and ADR draft.
2. Install Vale; commit `.vale.ini` + empty `Poplar/` style;
   verify `vale` runs.
3. Author STYLE.md §1 and §2.
4. Author STYLE.md §3 and §4 skeleton.
5. Translate the 10 highest-value tells to Vale rules with
   fixtures: em-dash, T16, T27, T33, T35, T39, T40,
   "comprehensive", "robust", "leverages".
6. Wire `make check` to call Vale; retire `voice-check.sh`.
7. Sweep `docs/poplar/` for `error`-severity findings.
8. Sweep `CLAUDE.md`, `.claude/rules/`,
   `.claude/skills/poplar-pass/`, `.claude/docs/`.
9. Sweep ADRs (errors only) and active `docs/superpowers/`
   plans/specs.
10. Trim the catalogue from `~/.claude/docs/go-comment-voice.md`;
    cross-reference STYLE.md.
11. Update `/simplify` voice agent brief to consume Vale JSON.
12. ADR finalize, consolidation ritual.

If rules-to-ship grows past 10, split at task 5: Pass A lands
STYLE.md, Vale, the 10 tells, and the sweep. Pass B adds the
remaining prose-only rules and re-sweeps.

## Risks

- **Vale install friction.** Contributors without Vale see
  `make check` fail with a clear install message. CI installs
  Vale in the workflow. One-time user-side cost.
- **False positives in quoted text.** `<!-- vale Off -->` covers
  it. STYLE.md §4 uses the escape to quote tells.
- **Rule drift.** Vale rules and STYLE.md can diverge. Mitigation:
  every rule's `message` cites a STYLE.md section. Reviewer
  checks both move together.
- **Catalogue growth.** By design. New tells log to §4 and add a
  Vale rule in the same commit.
- **Bikeshed on voice wording.** §1 is short on purpose. We anchor
  on the Google "knowledgeable friend, but for Go programmers"
  test and move on.

## References

- ADR-0141: voice policy for Go source.
- `~/.claude/docs/go-comment-voice.md`: the positive palette for
  code comments. The catalogue section retires from this doc;
  the palette stays.
- CockroachDB StyleGuide.md: shape precedent.
- Google Developer Style Guide: voice/tone-as-named-section
  precedent.
- Vale (`vale.sh`): the linter.
