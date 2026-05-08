# Doc voice — Pass A: build the gate

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the Vale-based prose-voice gate. Author `docs/poplar/STYLE.md`, ship the rule set (12 AITell + 5 ProseTell rules), wire `make check` to call Vale, retire `scripts/voice-check.sh`, sweep `docs/poplar/` clean, and record the policy in ADR-0174. Pass B drains the remaining trees.

**Architecture:** Vale (`vale.sh`, Go binary) replaces grep. Rules live in `.vale/styles/Poplar/`, organized by category. STYLE.md is the human-readable companion. `VALE_PATHS` starts narrow (`docs/poplar/` only) so `make check` stays green; Pass B widens it tree by tree.

**Tech Stack:** Vale, Make, bash, markdown.

**Spec:** `docs/superpowers/specs/2026-05-07-doc-voice-design.md`

**Pass split rationale:** Plan was 12 tasks with two subsystems (build the gate, drain the backlog). Pass A is architecture; Pass B is mechanical sweep. Reduces per-task review fatigue and keeps cross-task drift bounded.

---

## Files

**Create:**
- `docs/poplar/STYLE.md`
- `.vale.ini`
- `.vale/config/vocabularies/Poplar/accept.txt`
- `.vale/config/vocabularies/Poplar/reject.txt`
- `.vale/styles/Poplar/AITells/*.yml` (12 rules + fixtures)
- `.vale/styles/Poplar/ProseTells/*.yml` (5 rules + fixtures)
- `.vale/styles/Poplar/Mechanics/.gitkeep`
- `scripts/check-vale-fixtures.sh`
- `docs/poplar/decisions/0174-doc-voice-vale.md`

**Modify:**
- `Makefile` (replace `voice` target with `vale`, add `vale-test`; `VALE_PATHS = docs/poplar internal cmd`)
- `docs/poplar/invariants.md` (one bullet under Build & verification)

**Delete:**
- `scripts/voice-check.sh`

**Sweep:**
- `docs/poplar/**/*.md` (including `research/`, excluding `decisions/` and `archive/`)

---

## Task 1: Install Vale; commit foundation files

**Files:**
- Create: `.vale.ini`
- Create: `.vale/config/vocabularies/Poplar/accept.txt`
- Create: `.vale/config/vocabularies/Poplar/reject.txt`
- Create: `.vale/styles/Poplar/{AITells,ProseTells,Mechanics}/.gitkeep`

- [ ] **Step 1: Install Vale**

```bash
go install github.com/errata-ai/vale@latest
which vale && vale --version
```

Expected: `vale 3.x.x` printed.

- [ ] **Step 2: Write `.vale.ini`**

```ini
StylesPath = .vale/styles
MinAlertLevel = warning
Vocab = Poplar

[*.md]
BasedOnStyles = Poplar

[*.go]
BasedOnStyles = Poplar.AITells, Poplar.Mechanics
```

- [ ] **Step 3: Write `.vale/config/vocabularies/Poplar/accept.txt`**

```
Pike
Gerrand
lipgloss
bubbletea
glamour
muesli
charmbracelet
goldmark
chroma
modernc
sqlite
JMAP
IMAP
SMTP
godoc
gofmt
poplar
catkin
```

- [ ] **Step 4: Write empty `.vale/config/vocabularies/Poplar/reject.txt`**

Empty file (placeholder for project-specific denylist).

- [ ] **Step 5: Create style directories**

```bash
mkdir -p .vale/styles/Poplar/AITells .vale/styles/Poplar/ProseTells .vale/styles/Poplar/Mechanics
touch .vale/styles/Poplar/AITells/.gitkeep .vale/styles/Poplar/ProseTells/.gitkeep .vale/styles/Poplar/Mechanics/.gitkeep
```

- [ ] **Step 6: Smoke test Vale runs**

```bash
echo "test" > /tmp/vale-smoke.md
vale /tmp/vale-smoke.md
rm /tmp/vale-smoke.md
```

Expected: no errors, no findings. Exit 0.

- [ ] **Step 7: Commit**

```bash
git add .vale.ini .vale/
git commit -m "$(cat <<'EOF'
Add Vale config foundation

Empty Poplar style with three rule categories
(AITells, ProseTells, Mechanics). Vocabulary allowlist seeded
with project terms. Config scans .md with the full Poplar
style and .go with AITells + Mechanics.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Author docs/poplar/STYLE.md

**Files:**
- Create: `docs/poplar/STYLE.md`

- [ ] **Step 1: Write STYLE.md**

The full document. Four sections: voice and tone, genre playbooks, mechanics, word list and AI tells.

```markdown
# Poplar style

How poplar's prose reads. Applies to every Markdown file in the
repo, every comment in Go source, every ADR, every plan, every
spec. The mechanical gate is Vale (`make check`). This document
is the human-readable companion: when Vale flags a tell, find the
row in §4, fix the prose, move on.

## §1 Voice and tone

A knowledgeable Go programmer talking to peers.

State things. Do not apologize for stating them. The reader is a
working programmer. They want the rule, the example, the
rationale, in that order. They do not need warmups.

Concrete over abstract. Name the type, the file, the function.
"`mail.Backend.Connect` returns `ErrAuth` on a 401" beats "the
relevant component handles authentication failures."

Short sentences. Subordinate clauses earn their place. Two short
sentences read better than one long one with a comma splice in
the middle.

`We` appears in ADRs (decision narrative). Reference and tutorial
prose addresses the reader as `you` or no-person.

## §2 Genre playbooks

The repo's prose splits into four genres. Each has a different
natural voice. The shared discipline is the §4 word list. The
positive palette varies.

### Reference

**Files:** `docs/poplar/invariants.md`, `keybindings.md`,
`styling.md`, `system-map.md`, `bubbletea-conventions.md`
reference sections.

**Shape:** declarative, present tense, no first or second person.
Tables and lists carry the load. Each fact is a sentence;
sentences do not chain.

**Three positive moves.** Lead with the subject ("`mail.Backend`
is synchronous blocking."). Use parallel grammatical shape across
list items. Cite the ADR that codified the fact in parens at the
end of the bullet.

**Three pitfalls.** Narrating what the section will do ("This
document describes..."). Hedging facts that are decisions ("can
be considered..."). Mixing tenses across consecutive bullets.

### Tutorial

**Files:** `bubbletea-conventions.md` instructional sections,
`responsive-design.md`, future how-to docs.

**Shape:** instructional. Second person OK. Examples woven in.
Each rule has a why.

**Three positive moves.** Show the before-after for any rule.
State the rule, then the why, then the worked example. Use
imperative mood for the rule ("Cache renders in a `*<T>Cache`
pointer.").

**Three pitfalls.** Padding with transitions ("Now that we've
covered X, let's look at Y"). Restating the rule three times in
slightly different words. Burying the rule in a wall of context.

### Narrative

**Files:** `docs/poplar/decisions/*.md` (ADRs),
`docs/poplar/research/*.md`.

**Shape:** first-person plural, past tense for context, present
for the decision. Decision then rationale.

**Three positive moves.** Open with the decision in one sentence.
Quote the constraint that forced it. End with a one-line
implication for future passes.

**Three pitfalls.** Hedging the decision after stating it
("though this might change"). Burying the decision under the
rationale. Treating the ADR as a journal entry rather than a
record.

### Imperative

**Files:** `.claude/rules/*.md`, `.claude/skills/*/SKILL.md`,
`CLAUDE.md` rule sections, this document's §3 and §4.

**Shape:** commands. "Do X. Don't Y." No voice in the literary
sense.

**Three positive moves.** One rule per bullet. Active voice.
State the rule, not the rationale (rationale belongs in linked
ADRs).

**Three pitfalls.** Explanatory prose creeping in. Bullet trees
deeper than two levels. Soft commands ("you might want to...").

## §3 Mechanics

### Punctuation

- **Em dash (—)**: do not use. Replacements: period (sentence
  break), comma (tight aside), parens (explicit aside), colon
  (list or explanation). Em dashes are the loudest AI-prose tell
  in technical docs.
- **En dash (–)**: ranges only. `80×24`, `1990–2000`. Not as a
  sentence break.
- **Semicolon**: tables, lists, parallel constructs. Not as a
  clause-joiner. Break into two sentences. (A considered
  semicolon between two tightly-related clauses is fair game when
  prose reads better. Reviews flag overuse. ADR-0173.)
- **Colon**: introduces lists or definitions. One per paragraph
  at most.
- **Parens**: short asides. If the parenthetical runs longer than
  the surrounding sentence, it is a sentence.
- **Ellipsis (…)**: omitted content in quotation. Not for
  trailing thoughts.

### Structure

- **Headings**: sentence case, not Title Case. `## Connection
  state` not `## Connection State`.
- **Lists**: parallel grammatical shape. All bullets verbs, or
  all nouns, or all clauses. Mixing is a tell.
- **Code blocks**: language-tagged (`` ```go ``, `` ```bash ``,
  `` ```text ``). Inline code (backticks) for symbols
  (`Backend`, `mail.Classify`, file paths).

## §4 Word list and AI tells

When `make check` flags a Vale rule, find the row here. One row,
one fix.

The catalogue evolves. New tells log here and add a Vale rule in
the same commit.

### Padding adjectives

| Tell | Replace with |
| --- | --- |
| comprehensive | the specific scope ("covers X, Y, Z") or drop |
| robust | the specific guarantee ("survives crashes mid-write") or drop |
| seamless / seamlessly | drop |
| leverages / leverage | uses |
| utilizes | uses |
| facilitates | does |

### Hedge words

| Tell | Replace with |
| --- | --- |
| ensure / ensures | make sure / makes sure, or restate as the action |
| in order to | to |
| at this time | now (or drop) |
| simply | drop |
| basically | drop |

### Rhetorical signposting

| Tell | Replace with |
| --- | --- |
| It's worth noting that | drop and state the thing |
| It is important to note | drop and state the thing |
| Note that | drop |
| In this section we will | drop and start with the content |
| Indeed / Moreover / Furthermore | drop or use a period |

### AI-tic phrases

| Tell | Replace with |
| --- | --- |
| delve / delve into | examine, look at |
| navigate the complexities of | drop |
| unleash the power of | drop |
| at the heart of | drop |
| game-changer | drop |

### Code-comment-only tells

These fire on `.go` files and not on `.md`. Listed for
completeness; the Vale rules cite the same row.

| Tell | Rule | Catch |
| --- | --- | --- |
| T4 | "for now" hedge in a comment | use a real `TODO(owner)` or delete |
| T4 | bare or stub TODO | name an owner or a real decision |
| T10 | error string starts with "failed to" | bare noun or context prefix |
| T14 | `GetX` getter prefix | drop the prefix |
| T16 | role-suffix type name | name what it manages |
| T27 | apologetic comment | state real limitations or delete |
| T28 | over-explanation of Go idioms | comment on why, not what |
| T33 | em dash in a `//` comment | period |
| T35 | label colons | write prose |
| T39 | label-colon godoc opener | drop the label |
| T40 | NOTE/IMPORTANT prefix | prose for notes; `TODO(owner)` for tracked |
| T41 | SPDX header in source file | repo-level LICENSE is canonical (ADR-0169) |

### Punctuation tells

| Tell | Catch |
| --- | --- |
| em dash in prose | period, comma, parens, or colon |
| semicolon as clause-joiner in prose | break into two sentences |
| label-colon section opener | prose paragraph instead |

## Allowlist mechanics

When a flagged term has a legitimate use:

1. Add to `.vale/config/vocabularies/Poplar/accept.txt` if the
   term is project-vocabulary (proper noun, library name).
2. Wrap in a fenced code block. Vale skips by default.
3. Inline disable for narrow exceptions:
   ```
   <!-- vale Poplar.Ensure = NO -->
   ...the offending paragraph...
   <!-- vale Poplar.Ensure = YES -->
   ```

This document uses (3) to quote tells without recursive flagging.

## See also

- `~/.claude/docs/go-comment-voice.md`: positive palette for Go
  code comments. The catalogue cross-references this file.
- ADR-0141: original voice policy for Go source.
- ADR-0174: extension to public docs and Vale.
```

- [ ] **Step 2: Verify the file is well-formed**

```bash
test -f docs/poplar/STYLE.md && wc -l docs/poplar/STYLE.md
```

Expected: file exists, ~190 lines.

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/STYLE.md
git commit -m "$(cat <<'EOF'
Add docs/poplar/STYLE.md

Voice and tone, four genre playbooks (reference, tutorial,
narrative, imperative), mechanics, word list with AI tells.
The human-readable companion to the Vale ruleset.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Migrate voice-check.sh tells to Vale rules

Twelve rules (T4 splits into ForNow + StubTodo). Each is a YAML rule plus a `.test.md` fixture. Patterns mirror `scripts/voice-check.sh` exactly so coverage is preserved.

**Files:**
- Create: `.vale/styles/Poplar/AITells/{T4_ForNow,T4_StubTodo,T10_FailedTo,T14_Getter,T16_RoleSuffix,T27_Apologetic,T28_OverExplain,T33_EmDashComment,T35_LabelColons,T39_LabelColonGodoc,T40_NoteImportant,T41_SPDX}.yml`
- Create: matching `.test.md` fixture per rule

- [ ] **Step 1: Write T4_ForNow**

```yaml
# .vale/styles/Poplar/AITells/T4_ForNow.yml
extends: existence
message: "Tell #4. 'for now' hedge. Use a real TODO(owner) or delete. STYLE.md §4."
level: error
scope: comment
ignorecase: true
tokens:
  - 'for now'
```

Fixture `.vale/styles/Poplar/AITells/T4_ForNow.test.md`:

````markdown
<!-- bad -->
```go
// for now we skip the cache
```

<!-- good -->
```go
// TODO(glw907): replace with cached lookup once schema v8 lands.
```
````

- [ ] **Step 2: Write T4_StubTodo**

```yaml
# .vale/styles/Poplar/AITells/T4_StubTodo.yml
extends: existence
message: "Tell #4. Bare or stub TODO. Name an owner or a real decision, or delete. STYLE.md §4."
level: error
scope: comment
tokens:
  - '\bTODO\s*:?\s*(improve|fix this|cleanup|clean up|refactor|later)?\s*\.?\s*$'
```

Fixture:

````markdown
<!-- bad -->
```go
// TODO: cleanup
```

<!-- good -->
```go
// TODO(glw907): swap to bufio.Scanner once go-message lands a streaming API.
```
````

- [ ] **Step 3: Write T10_FailedTo**

```yaml
# .vale/styles/Poplar/AITells/T10_FailedTo.yml
extends: existence
message: "Tell #10. Error string starts with 'failed to'. Use a bare noun or context prefix. STYLE.md §4."
level: error
tokens:
  - '(errors\.New|fmt\.Errorf)\("failed to\b'
```

Fixture:

````markdown
<!-- bad -->
```go
return errors.New("failed to open mailbox")
```

<!-- good -->
```go
return fmt.Errorf("open mailbox: %w", err)
```
````

- [ ] **Step 4: Write T14_Getter**

```yaml
# .vale/styles/Poplar/AITells/T14_Getter.yml
extends: existence
message: "Tell #14. Getter prefixed with Get. Drop the prefix. STYLE.md §4."
level: error
tokens:
  - '^func \([^)]+\) Get[A-Z]\w*\('
```

Fixture:

````markdown
<!-- bad -->
```go
func (m *Model) GetCursor() int { return m.cursor }
```

<!-- good -->
```go
func (m *Model) Cursor() int { return m.cursor }
```
````

- [ ] **Step 5: Write T16_RoleSuffix**

```yaml
# .vale/styles/Poplar/AITells/T16_RoleSuffix.yml
extends: existence
message: "Tell #16. Role-suffix type name. Name what it manages/helps/serves. STYLE.md §4."
level: error
tokens:
  - '^type \w+(Manager|Helper|Util|Service) (struct|interface)\b'
```

Fixture:

````markdown
<!-- bad -->
```go
type CacheManager struct { /* ... */ }
```

<!-- good -->
```go
type Cache struct { /* ... */ }
```
````

- [ ] **Step 6: Write T27_Apologetic**

```yaml
# .vale/styles/Poplar/AITells/T27_Apologetic.yml
extends: existence
message: "Tell #27. Apologetic/speculative comment. State real limitations or delete. STYLE.md §4."
level: error
scope: comment
ignorecase: true
tokens:
  - '\b(may not handle|could be improved|might not work|perhaps we|maybe we)\b'
```

Fixture:

````markdown
<!-- bad -->
```go
// might not work for very large mailboxes
```

<!-- good -->
```go
// Caps mailbox load at 5000 messages. Larger folders paginate.
```
````

- [ ] **Step 7: Write T28_OverExplain**

```yaml
# .vale/styles/Poplar/AITells/T28_OverExplain.yml
extends: existence
message: "Tell #28. Explains standard Go mechanics. Comment on why this is structured this way, not what range/goroutine/close do. STYLE.md §4."
level: error
scope: comment
ignorecase: true
tokens:
  - '\b(use a goroutine to|close the channel when|iterate over (all|the|each)|loop through (the|all|each))\b'
```

Fixture:

````markdown
<!-- bad -->
```go
// iterate over each message and mark it seen
```

<!-- good -->
```go
// Mark seen optimistically. The cache reverts on auth failure.
```
````

- [ ] **Step 8: Write T33_EmDashComment**

```yaml
# .vale/styles/Poplar/AITells/T33_EmDashComment.yml
extends: existence
message: "Tell #33. Em dash in a Go comment. Use a period. Reserve em dashes for short comma-like asides only. STYLE.md §4."
level: error
scope: comment
tokens:
  - '—'
```

Fixture:

````markdown
<!-- bad -->
```go
// Cache miss falls through to the backend — see ADR-0084.
```

<!-- good -->
```go
// Cache miss falls through to the backend. See ADR-0084.
```
````

- [ ] **Step 9: Write T35_LabelColons**

```yaml
# .vale/styles/Poplar/AITells/T35_LabelColons.yml
extends: existence
message: "Tell #35. Documentation label in code comment. Write prose. STYLE.md §4."
level: error
scope: comment
tokens:
  - '^\s*(Preference|Fallback|Priority|Rationale|Caveat|Otherwise|Constraint|Invariant):'
```

Fixture:

````markdown
<!-- bad -->
```go
// Preference: bubbles/textinput for single-line entry.
// Fallback: hand-rolled when bubbles lacks the affordance.
```

<!-- good -->
```go
// Use bubbles/textinput for single-line entry. Hand-roll only
// when the affordance is missing.
```
````

- [ ] **Step 10: Write T39_LabelColonGodoc**

```yaml
# .vale/styles/Poplar/AITells/T39_LabelColonGodoc.yml
extends: existence
message: "Tell #39. Label-colon godoc opener. Idiomatic Go godoc is prose paragraphs. Drop the label and write the rule. STYLE.md §4."
level: error
scope: comment
tokens:
  - '^// [A-Z][a-zA-Z]+: '
```

Fixture:

````markdown
<!-- bad -->
```go
// Picker list: each row carries a wired flag.
```

<!-- good -->
```go
// Each row in the picker carries a wired flag.
```
````

- [ ] **Step 11: Write T40_NoteImportant**

```yaml
# .vale/styles/Poplar/AITells/T40_NoteImportant.yml
extends: existence
message: "Tell #40. NOTE/IMPORTANT/TODO prefix. Use TODO(owner): for tracked TODOs and prose for notes. STYLE.md §4."
level: error
scope: comment
tokens:
  - '^\s*(NOTE|IMPORTANT|TODO):'
```

Fixture:

````markdown
<!-- bad -->
```go
// NOTE: cache evicts oldest first.
```

<!-- good -->
```go
// Cache evicts oldest first.
```
````

- [ ] **Step 12: Write T41_SPDX**

```yaml
# .vale/styles/Poplar/AITells/T41_SPDX.yml
extends: existence
message: "Tell #41. SPDX header in source file. Repo-level LICENSE is canonical (ADR-0169). Remove the line. STYLE.md §4."
level: error
tokens:
  - '^// SPDX-License-Identifier:'
```

Fixture:

````markdown
<!-- bad -->
```go
// SPDX-License-Identifier: MIT
package mail
```

<!-- good -->
```go
package mail
```
````

- [ ] **Step 13: Run Vale on each fixture; verify the bad case fires**

```bash
for f in .vale/styles/Poplar/AITells/*.test.md; do
  echo "=== $f ==="
  vale "$f" || true
done
```

Expected: each fixture surfaces one finding on its `<!-- bad -->` block and zero on `<!-- good -->`.

- [ ] **Step 14: Commit**

```bash
git add .vale/styles/Poplar/AITells/
git commit -m "$(cat <<'EOF'
Migrate voice-check.sh tells to Vale rules

Twelve rules under .vale/styles/Poplar/AITells/ mirror the
existing scripts/voice-check.sh tells T4 (split into ForNow +
StubTodo), T10, T14, T16, T27, T28, T33, T35, T39, T40, T41.
Each rule ships with a .test.md fixture (bad case + good case).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add prose-only Vale rules

Five rules for prose-specific tells: em-dash, semicolon clause-joiner, padding adjectives, ensure, signposting.

**Files:**
- Create: `.vale/styles/Poplar/ProseTells/{EmDash,SemicolonClauseJoiner,PaddingAdjectives,Ensure,Signposting}.yml`
- Create: matching `.test.md` fixtures

- [ ] **Step 1: Write EmDash**

```yaml
# .vale/styles/Poplar/ProseTells/EmDash.yml
extends: existence
message: "Em dashes read as AI-generated. Replace with period, comma, parens, or colon. STYLE.md §3."
level: error
tokens:
  - '—'
```

Fixture:

```markdown
<!-- bad -->
The cache is per-account — one SQLite database per address.

<!-- good -->
The cache is per-account. One SQLite database per address.
```

- [ ] **Step 2: Write SemicolonClauseJoiner**

```yaml
# .vale/styles/Poplar/ProseTells/SemicolonClauseJoiner.yml
extends: existence
message: "Semicolon as clause-joiner. Break into two sentences. STYLE.md §3."
level: warning
tokens:
  - '[a-z]+;\s+[a-z]'
```

Fixture:

```markdown
<!-- bad -->
The cache is per-account; one database per address.

<!-- good -->
The cache is per-account. One database per address.
```

- [ ] **Step 3: Write PaddingAdjectives**

```yaml
# .vale/styles/Poplar/ProseTells/PaddingAdjectives.yml
extends: substitution
message: "Padding adjective. Use the specific scope, the specific guarantee, or drop. STYLE.md §4."
level: error
ignorecase: true
swap:
  comprehensive: '<specific scope>'
  robust: '<specific guarantee>'
  seamless: ''
  seamlessly: ''
  leverages: uses
  leverage: use
  utilizes: uses
  facilitates: does
```

Fixture:

```markdown
<!-- bad -->
The cache provides comprehensive offline support and robust error handling. Poplar leverages SQLite's WAL mode.

<!-- good -->
The cache works offline (read and write). Outbox conflicts are visible via the conflict overlay. Poplar uses SQLite's WAL mode.
```

- [ ] **Step 4: Write Ensure**

```yaml
# .vale/styles/Poplar/ProseTells/Ensure.yml
extends: substitution
message: "'Ensure' is AI-flavored. Use 'make sure', 'check', or restate as the action. STYLE.md §4."
level: error
ignorecase: true
swap:
  ensure: make sure
  ensures: makes sure
  ensured: made sure
  ensuring: making sure
```

Fixture:

```markdown
<!-- bad -->
Ensure the cache is open before queueing the op.

<!-- good -->
Open the cache before queueing the op. The drainer panics if it isn't.
```

- [ ] **Step 5: Write Signposting**

```yaml
# .vale/styles/Poplar/ProseTells/Signposting.yml
extends: existence
message: "Rhetorical signposting. Drop and state the thing directly. STYLE.md §4."
level: error
ignorecase: true
tokens:
  - "it's worth noting that"
  - "it is important to note"
  - "note that"
  - "in this section we will"
  - "let's (delve|explore|examine|dive)"
  - "indeed,"
  - "moreover,"
  - "furthermore,"
```

Fixture:

```markdown
<!-- bad -->
It's worth noting that the cache is per-account. Note that the schema is versioned. Furthermore, migrations run on Open.

<!-- good -->
The cache is per-account. The schema is versioned. Migrations run on Open.
```

- [ ] **Step 6: Run Vale on each fixture**

```bash
for f in .vale/styles/Poplar/ProseTells/*.test.md; do
  echo "=== $f ==="
  vale "$f" || true
done
```

Expected: each fixture surfaces findings on `<!-- bad -->` lines and none on `<!-- good -->` lines.

- [ ] **Step 7: Commit**

```bash
git add .vale/styles/Poplar/ProseTells/
git commit -m "$(cat <<'EOF'
Add prose-only Vale rules

Five rules under .vale/styles/Poplar/ProseTells/: em-dash,
semicolon clause-joiner (warning), padding adjectives,
'ensure', rhetorical signposting. Each ships with a fixture.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Wire Vale into make check; retire voice-check.sh

VALE_PATHS starts narrow. The Pass A sweep cleans `docs/poplar/`. Pass B widens.

**Files:**
- Modify: `Makefile`
- Create: `scripts/check-vale-fixtures.sh`
- Delete: `scripts/voice-check.sh`

- [ ] **Step 1: Write `scripts/check-vale-fixtures.sh`**

```bash
#!/usr/bin/env bash
# check-vale-fixtures: assert each Poplar rule's .test.md fixture
# surfaces ≥1 finding when scanned by Vale.

set -euo pipefail

ROOT=${1:-.vale/styles/Poplar}
fail=0
total=0

for fixture in "$ROOT"/*/*.test.md; do
    total=$((total + 1))
    hits=$(vale --output=line "$fixture" 2>/dev/null | wc -l)
    if [ "$hits" -lt 1 ]; then
        echo "FAIL: $fixture flagged 0 findings (rule may be broken)"
        fail=$((fail + 1))
    fi
done

echo "checked $total fixtures, $fail failures"
exit "$fail"
```

```bash
chmod +x scripts/check-vale-fixtures.sh
```

- [ ] **Step 2: Run the fixture check**

```bash
./scripts/check-vale-fixtures.sh
```

Expected: `checked 17 fixtures, 0 failures`.

- [ ] **Step 3: Modify Makefile**

Replace the `voice` target with `vale` + `vale-test`. VALE_PATHS scoped narrow for Pass A; Pass B widens it.

Old:
```make
voice:
	@./scripts/voice-check.sh

check: fmt-check vet voice test
```

New:
```make
VALE := vale
VALE_PATHS := docs/poplar internal cmd

vale:
	@command -v $(VALE) >/dev/null || { \
	  echo "vale not installed. go install github.com/errata-ai/vale@latest"; \
	  exit 1; \
	}
	$(VALE) --minAlertLevel=error $(VALE_PATHS)

vale-test:
	@./scripts/check-vale-fixtures.sh

check: fmt-check vet vale test
```

Update `.PHONY`. Old:
```make
.PHONY: build test test-imap vet fmt-check voice lint audit install check clean
```

New:
```make
.PHONY: build test test-imap vet fmt-check vale vale-test lint audit install check clean
```

- [ ] **Step 4: Delete voice-check.sh**

```bash
git rm scripts/voice-check.sh
```

- [ ] **Step 5: Run `make vale`**

```bash
make vale
```

Expected: Vale runs against `VALE_PATHS`. Findings expected (Pass A's sweep in Task 6 cleans them up). Note the count.

- [ ] **Step 6: Run `make vale-test`**

```bash
make vale-test
```

Expected: `checked 17 fixtures, 0 failures`.

- [ ] **Step 7: Commit**

```bash
git add Makefile scripts/check-vale-fixtures.sh
git commit -m "$(cat <<'EOF'
Wire Vale into make check; retire voice-check.sh

Replace the voice target with vale + vale-test. VALE_PATHS
narrow to docs/poplar/, internal/, cmd/ for Pass A. Pass B
widens to .claude/, decisions/, and superpowers/ tree by tree.
voice-check.sh deleted; tells migrated to Vale rules in the
previous commits.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Sweep docs/poplar/ for error-severity findings

**Files:** every `.md` under `docs/poplar/` except `decisions/` (Pass B), `archive/` (immutable).

- [ ] **Step 1: Run Vale and capture findings**

```bash
vale --minAlertLevel=error --output=line docs/poplar/*.md docs/poplar/research/ > /tmp/vale-docs-poplar.txt 2>&1 || true
cat /tmp/vale-docs-poplar.txt
```

Triage rules:

- **Em dash**: replace with period, comma, parens, or colon (case-by-case).
- **Padding adjective**: replace per the swap or drop.
- **Ensure**: rephrase as the action.
- **Signposting**: drop the signpost; the next clause stands alone.
- **Label-colon godoc opener** in prose: rephrase as a sentence.

- [ ] **Step 2: Fix findings file by file**

Open each flagged file. Apply edits. Re-run Vale on that file to confirm clean before moving on.

```bash
# Per-file verification:
vale --minAlertLevel=error <file>
```

- [ ] **Step 3: Verify the whole tree is clean**

```bash
vale --minAlertLevel=error docs/poplar/*.md docs/poplar/research/
```

Expected: no findings. Exit 0.

- [ ] **Step 4: Run full make check**

```bash
make check
```

Expected: green. fmt-check, vet, vale, test all pass.

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/
git commit -m "$(cat <<'EOF'
Sweep docs/poplar/ for voice errors

Em dashes replaced. Padding adjectives swapped or dropped.
Signposting removed. Findings reduced to zero at error level
across docs/poplar/ (excluding decisions/ and archive/).
Pass A's gate is now green end-to-end.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: ADR-0174 + invariants update

**Files:**
- Create: `docs/poplar/decisions/0174-doc-voice-vale.md`
- Modify: `docs/poplar/invariants.md` (one bullet under Build & verification)
- Modify: `docs/poplar/decisions/INDEX.md` if present

- [ ] **Step 1: Write ADR-0174**

```markdown
# ADR-0174: Doc voice extends to public docs; Vale replaces voice-check.sh

Date: 2026-05-07
Status: Accepted

## Context

ADR-0141 codified voice policy for Go source: comments and
godoc match the poplar palette (stdlib-formal, Pike-aphoristic
errors, Gerrand-welcoming package docs). The catalogue of AI
tells lived inline in `~/.claude/docs/go-comment-voice.md §7`
and was enforced by `scripts/voice-check.sh` (grep-tier
patterns) at `make check`.

Two gaps surfaced in pre-beta passes:

1. The repo will be public at v1.0. Prose under `docs/poplar/`
   ships with the binary. AI-prose tells in invariants and ADRs
   disqualify the project from reading like a thoughtful Go
   programmer wrote it.
2. Claude reads `CLAUDE.md`, the auto-loaded rules, the skills
   it invokes, and the ADRs/research it cites on every turn.
   AI-flavored input documents prime AI-flavored drafts.
   Cleaning the inputs is more efficient than catching the
   outputs at review time.

The 2026-05-07 prior-art research surveyed Go, Charm,
CockroachDB, Kubernetes, Rust, SQLite, GitLab, Google, Microsoft,
and Grafana for documentation style guides and mechanical
enforcement. Findings:

- Vale (vale.sh) is the dominant prose linter in serious
  docs-first OSS (CockroachDB, Grafana, Datadog, Meilisearch).
  Go-native, regex YAML rules, format-aware (.md and .go),
  composable styles.
- No project surveyed targets AI tells with a Vale ruleset.
  The field is open.
- The CockroachDB shape (`docs/StyleGuide.md` + Vale custom
  style) is the closest fit for poplar.
- Google's Developer Style Guide treats voice/tone as a named
  section separate from mechanics. The cleanest split in the
  survey.

## Decision

1. Voice policy extends from Go source to all repo prose:
   `docs/poplar/`, `CLAUDE.md`, `.claude/rules/`,
   `.claude/skills/`, `.claude/docs/`, active
   `docs/superpowers/plans/` and `specs/`. Archived plans stay
   immutable.
2. The catalogue moves to one source of truth:
   `docs/poplar/STYLE.md §4` (human-readable) and
   `.vale/styles/Poplar/AITells/` + `ProseTells/` (mechanical
   gate). `~/.claude/docs/go-comment-voice.md` retires its
   inline catalogue and cross-references STYLE.md (Pass B).
3. STYLE.md adds four genre playbooks (reference, tutorial,
   narrative, imperative). Each genre has its own positive
   palette. The negative space (the catalogue) is shared.
4. Vale replaces `scripts/voice-check.sh`. The twelve Go-source
   tells migrate to Vale rules. Five prose-only rules ship
   alongside (em-dash, semicolon clause-joiner, padding
   adjectives, ensure, signposting). `voice-check.sh` is
   deleted in the same commit that wires `make vale` into
   `make check`.
5. The `/simplify` voice review agent treats Vale findings as
   the mechanical floor and STYLE.md §1–§2 as the squishy layer
   it assesses on its own (Pass B).

## Pass split

This work landed across two passes. Pass A (this ADR) builds
the gate: STYLE.md, the 17 Vale rules, the Makefile wiring,
voice-check.sh retirement, and the `docs/poplar/` sweep that
proves the gate works green. Pass B widens VALE_PATHS to the
remaining trees (`.claude/`, `decisions/`, `superpowers/`),
trims the global catalogue, and updates the `/simplify` brief.

## Consequences

- One mechanical tool (`vale`) replaces bespoke grep. Go and
  Markdown share the same gate. Single source of truth for
  tells.
- Public docs ship under voice discipline at v1.0.
- Contributors install Vale alongside the toolchain. CI
  installs it in the workflow. One-time friction, with an
  explicit install message in the Makefile.
- The catalogue grows over time. New tells add a row in
  STYLE.md §4 and a Vale rule in the same commit.
- Poplar is, per the prior-art survey, the first Go OSS project
  with a Vale ruleset that targets AI prose tells. Small
  novelty, worth flagging.

## See also

- ADR-0141: original voice policy for Go source.
- ADR-0173: T34 (semicolon clause-joiner) demoted to voice-lens
  only in Go scope. Reinstated as a warning-level Vale rule for
  prose.
- `docs/poplar/STYLE.md`: the human-readable companion.
- `docs/superpowers/specs/2026-05-07-doc-voice-design.md`: the
  design spec.
```

- [ ] **Step 2: Update docs/poplar/invariants.md**

Find the "Build & verification" section. Locate the bullet that mentions `scripts/voice-check.sh`. Replace it with:

> The voice step is Vale (`vale.sh`). `make check` runs `vale`
> against the prose-bearing tree
> (`VALE_PATHS = docs/poplar internal cmd` in Pass A; widens
> through `.claude/`, `decisions/`, `superpowers/` in Pass B).
> Rules live in `.vale/styles/Poplar/`. The human-readable
> companion is `docs/poplar/STYLE.md`. New tells log to
> STYLE.md §4 and add a Vale rule in the same commit.
> ADR-0174.

If invariants.md enumerates specific T-numbers (T4, T10, T14, etc.), replace that enumeration with: "Twelve Go-source rules and five prose-only rules ship today. See `.vale/styles/Poplar/`."

- [ ] **Step 3: Update INDEX.md if present**

```bash
test -f docs/poplar/decisions/INDEX.md && grep -n 'ADR-0173' docs/poplar/decisions/INDEX.md
```

If INDEX.md exists, add an entry for ADR-0174 in the appropriate themed section (group with 0141 and 0173 under voice/prose).

- [ ] **Step 4: Verify Vale on the new ADR**

```bash
vale --minAlertLevel=error docs/poplar/decisions/0174-doc-voice-vale.md
```

Expected: no findings. (ADR-0174 lives outside Pass A's VALE_PATHS, so this is a manual check; Pass B sweeps decisions/.)

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/decisions/0174-doc-voice-vale.md docs/poplar/invariants.md
test -f docs/poplar/decisions/INDEX.md && git add docs/poplar/decisions/INDEX.md
git commit -m "$(cat <<'EOF'
ADR-0174: doc voice extension and Vale migration

Records the policy extension from Go source to all repo prose,
the choice of Vale over bespoke grep, and the retirement of
voice-check.sh. Updates invariants.md to point at Vale and
docs/poplar/STYLE.md.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Pass-end consolidation

Invoke the `poplar-pass` skill ritual.

- [ ] **Step 1: Run the full check gate**

```bash
make check
```

Expected: `fmt-check`, `vet`, `vale`, `test` all pass. Exit 0.

- [ ] **Step 2: Run vale-test**

```bash
make vale-test
```

Expected: `checked 17 fixtures, 0 failures`.

- [ ] **Step 3: Update STATUS.md**

Open `docs/poplar/STATUS.md`. Update the "current pass" entry. Queue Pass B as the next starter prompt: "Doc voice — Pass B: drain the backlog."

- [ ] **Step 4: Push**

```bash
git push
```

- [ ] **Step 5: Install**

```bash
make install
```

Expected: `poplar` binary installed to `~/.local/bin/poplar`.

- [ ] **Step 6: Sanity check**

```bash
which poplar && poplar --version
```

Expected: path printed, version printed.

- [ ] **Step 7: Pass-end commit**

```bash
git add docs/poplar/STATUS.md
git commit -m "$(cat <<'EOF'
Pass A complete: doc voice gate live

Vale ruleset (12 AITells + 5 ProseTells) ships. STYLE.md
authored. make check wired. voice-check.sh retired.
docs/poplar/ swept clean. ADR-0174 records the policy.
Pass B widens VALE_PATHS and drains .claude/, decisions/,
superpowers/.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
git push
```

---

## Self-review checks

- [ ] `make check` exits 0.
- [ ] `make vale-test` exits 0.
- [ ] `vale --minAlertLevel=error` finds nothing across `VALE_PATHS`.
- [ ] `scripts/voice-check.sh` does not exist.
- [ ] `docs/poplar/STYLE.md` exists.
- [ ] `docs/poplar/decisions/0174-doc-voice-vale.md` exists.
- [ ] `docs/poplar/invariants.md` mentions Vale.
- [ ] `STATUS.md` queues Pass B as the next pass.

If any check fails, return to the relevant task.
