# Catkin annotations + spellcheck — Pass 9d implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a generic range-based annotation pipeline to Catkin and ship spellcheck as the first consumer — squiggles via embedded SymSpell, `Ctrl+;` popover with up to five suggestions plus add/ignore actions.

**Architecture:** `Annotator` interface produces `[]Annotation` over the raw source. A debounced 350 ms idle tick + generation counter recomputes the `AnnotationSet` in a `tea.Cmd`; stale results are dropped. `Render` overlays decoration styles per range. The popover is a Catkin-owned overlay drawn on top of `Render`'s output via row-by-row ANSI-aware substitution.

**Tech Stack:** Go 1.26, bubbletea, bubbles, lipgloss, charmbracelet/x/ansi, alecthomas/chroma/v2 (already vendored). No new third-party deps — SymSpell is hand-rolled. Wordlists embed via `//go:embed`.

**Spec:** `docs/superpowers/specs/2026-05-05-catkin-annotations-design.md`.

---

## File map

**Created:**

- `internal/catkin/annotate.go` — `Range`, `AnnotationKind`, `Annotation`, `MisspellingPayload`, `Annotator`, `AnnotationSet`. Generation-counter tick + `annotateRequestMsg` / `annotationsReadyMsg`.
- `internal/catkin/annotate_test.go` — pipeline contract tests.
- `internal/catkin/spellcheck.go` — `Speller` (hand-rolled SymSpell), `LoadUserWordlist`, `NewSpeller`, `spellcheckAnnotator`, code/inline/URL skip masks, all-caps skip.
- `internal/catkin/spellcheck_test.go` — engine + annotator tests.
- `internal/catkin/spellcheck/en_US.txt` — frequency-sorted top-50k (committed; built by `scripts/build-wordlist.sh`).
- `internal/catkin/spellcheck/project.txt` — hand-curated poplar terms.
- `internal/catkin/popover.go` — popover state, render, key bindings, apply/add/ignore.
- `internal/catkin/popover_test.go` — popover behavior tests.
- `scripts/build-wordlist.sh` — one-shot script that produces `en_US.txt` from upstream sources. Committed for reproducibility, not run during build.

**Modified:**

- `internal/catkin/style.go` — add `Squiggle`, `Popover`, `PopoverSelected` to `Styles`.
- `internal/catkin/render.go` — `Render` accepts an `*AnnotationSet` (nil = pass-through) and applies annotation styles per-cell on each visible row before the cursor block.
- `internal/catkin/catkin.go` — `Model` carries `srcGen`, `annoGen`, `annotations *AnnotationSet`, `annotators []Annotator`, `popover popoverState`, `ignored map[string]struct{}`. `Update` schedules annotate-tick after edits, dispatches popover keys, gates find vs popover. `View` threads `m.annotations` into `Render` and overlays the popover.
- `internal/catkin/catkin_test.go` — integration test for `RegisterAnnotator` + popover round-trip.
- `docs/poplar/invariants.md` — Catkin section gains the annotation + spellcheck binding facts (done at pass end, not in-task).
- `docs/poplar/STATUS.md` — pass-end ritual.

**Test fixtures:**

- `internal/catkin/spellcheck/testdata/small_words.txt` — small fixture wordlist for engine tests.

---

## Task 1: Annotation primitive types and AnnotationSet

**Files:**

- Create: `internal/catkin/annotate.go`
- Test: `internal/catkin/annotate_test.go`

- [ ] **Step 1: Write failing tests for the primitive types and AnnotationSet shape**

```go
// internal/catkin/annotate_test.go
package catkin

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRangeContains(t *testing.T) {
	r := Range{Start: 3, End: 7}
	cases := []struct {
		off  int
		want bool
	}{
		{2, false},
		{3, true},
		{6, true},
		{7, false}, // half-open
	}
	for _, c := range cases {
		if got := r.Contains(c.off); got != c.want {
			t.Errorf("Range{3,7}.Contains(%d) = %v, want %v", c.off, got, c.want)
		}
	}
}

func TestAnnotationSetByRow(t *testing.T) {
	// Source: "abc\ndef\nghi" — newlines at offsets 3 and 7.
	// Annotations on row 0 (Start=0), row 1 (Start=4), row 2 (Start=8).
	src := "abc\ndef\nghi"
	anns := []Annotation{
		{Range: Range{0, 3}, Kind: KindMisspelling},
		{Range: Range{4, 7}, Kind: KindMisspelling},
		{Range: Range{8, 11}, Kind: KindMisspelling},
	}
	set := newAnnotationSet(src, anns)
	if got := set.firstOnRow(0); got != 0 {
		t.Errorf("firstOnRow(0) = %d, want 0", got)
	}
	if got := set.firstOnRow(1); got != 1 {
		t.Errorf("firstOnRow(1) = %d, want 1", got)
	}
	if got := set.firstOnRow(2); got != 2 {
		t.Errorf("firstOnRow(2) = %d, want 2", got)
	}
	if got := set.firstOnRow(99); got != -1 {
		t.Errorf("firstOnRow(99) = %d, want -1", got)
	}
}

func TestAnnotationCarriesStyleAndPayload(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true)
	a := Annotation{
		Range:   Range{0, 3},
		Kind:    KindMisspelling,
		Style:   style,
		Payload: MisspellingPayload{Word: "abc", Suggestions: []string{"abd"}},
	}
	if a.Style != style {
		t.Errorf("Style not preserved")
	}
	mp, ok := a.Payload.(MisspellingPayload)
	if !ok || mp.Word != "abc" {
		t.Errorf("Payload = %#v, want MisspellingPayload{Word:\"abc\"}", a.Payload)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run TestRange -v`
Expected: FAIL with "undefined: Range" (and similar for other tests).

- [ ] **Step 3: Implement the types and AnnotationSet**

```go
// internal/catkin/annotate.go
package catkin

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Range is a half-open byte-offset range over the raw source.
// Stored as offsets — not row/col — so annotators don't re-derive
// from a moving cursor; rendering maps offsets to row/col once.
type Range struct{ Start, End int }

// Contains reports whether off lies in [Start, End).
func (r Range) Contains(off int) bool { return off >= r.Start && off < r.End }

// AnnotationKind identifies the producer category. Reserved
// future kinds (grammar, lint) sit beside KindMisspelling so
// composition rules can key off the kind.
type AnnotationKind int

const (
	KindMisspelling AnnotationKind = iota
)

// Annotation is one decoration over a source range.
type Annotation struct {
	Range   Range
	Kind    AnnotationKind
	Style   lipgloss.Style
	Payload any
}

// MisspellingPayload is the typed payload for KindMisspelling.
type MisspellingPayload struct {
	Word        string
	Suggestions []string // up to 5, frequency-ordered
}

// Annotator produces annotations over the full source. Implementations
// must be pure: no I/O, no goroutine kickoff. Heavy work runs on
// the idle tick path.
type Annotator interface {
	Name() string
	Annotate(src string) []Annotation
}

// AnnotationSet is the per-frame artifact rendering consults. The
// All slice is sorted by Range.Start.
type AnnotationSet struct {
	All   []Annotation
	byRow []int // first index of an annotation starting on row r; -1 if none
}

func newAnnotationSet(src string, anns []Annotation) *AnnotationSet {
	rowStarts := []int{0}
	for i, r := range src {
		_ = r
		if src[i] == '\n' {
			rowStarts = append(rowStarts, i+1)
		}
	}
	byRow := make([]int, len(rowStarts))
	for i := range byRow {
		byRow[i] = -1
	}
	row := 0
	for ai, a := range anns {
		for row+1 < len(rowStarts) && a.Range.Start >= rowStarts[row+1] {
			row++
		}
		if row < len(byRow) && byRow[row] == -1 {
			byRow[row] = ai
		}
	}
	return &AnnotationSet{All: anns, byRow: byRow}
}

func (s *AnnotationSet) firstOnRow(row int) int {
	if s == nil || row < 0 || row >= len(s.byRow) {
		return -1
	}
	return s.byRow[row]
}

// rangesOnRow returns annotations that intersect row r, where
// row offsets are derived from src (the same src that built the
// set). Used by the renderer.
func (s *AnnotationSet) rangesOnRow(src string, row int) []Annotation {
	if s == nil {
		return nil
	}
	rowStarts := []int{0}
	for i := range src {
		if src[i] == '\n' {
			rowStarts = append(rowStarts, i+1)
		}
	}
	if row < 0 || row >= len(rowStarts) {
		return nil
	}
	rowStart := rowStarts[row]
	rowEnd := len(src)
	if row+1 < len(rowStarts) {
		rowEnd = rowStarts[row+1] - 1 // exclude '\n'
	}
	var out []Annotation
	for _, a := range s.All {
		if a.Range.End <= rowStart || a.Range.Start >= rowEnd {
			continue
		}
		out = append(out, a)
	}
	return out
}

// helper kept here to avoid importing strings just for the test below.
var _ = strings.Index
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/catkin/ -run "TestRange|TestAnnotation" -v`
Expected: PASS for `TestRangeContains`, `TestAnnotationSetByRow`, `TestAnnotationCarriesStyleAndPayload`.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/annotate.go internal/catkin/annotate_test.go
git commit -m "Pass 9d: Catkin annotation primitive types + AnnotationSet"
```

---

## Task 2: Annotator registry and idle scheduling on Model

**Files:**

- Modify: `internal/catkin/catkin.go`
- Modify: `internal/catkin/annotate.go` (add the Msg types and helpers)
- Test: `internal/catkin/annotate_test.go`

- [ ] **Step 1: Write failing tests for registration and stale-drop**

Append to `internal/catkin/annotate_test.go`:

```go
type fakeAnnotator struct {
	name  string
	calls int
	out   []Annotation
}

func (f *fakeAnnotator) Name() string { return f.name }
func (f *fakeAnnotator) Annotate(src string) []Annotation {
	f.calls++
	return f.out
}

func TestRegisterAnnotator(t *testing.T) {
	m := New()
	a := &fakeAnnotator{name: "fake", out: []Annotation{{Range: Range{0, 3}, Kind: KindMisspelling}}}
	m.RegisterAnnotator(a)
	set := runAnnotators(m.annotators, "abc def")
	if len(set) != 1 || set[0].Range.Start != 0 {
		t.Fatalf("runAnnotators output = %#v, want one annotation at 0", set)
	}
	if a.calls != 1 {
		t.Errorf("Annotate calls = %d, want 1", a.calls)
	}
}

func TestAnnotateStaleDrop(t *testing.T) {
	m := New()
	m.srcGen = 5
	// Ready msg from generation 4 should be ignored.
	m, _ = m.Update(annotationsReadyMsg{gen: 4, set: &AnnotationSet{}})
	if m.annotations != nil {
		t.Errorf("stale annotationsReadyMsg should not install a set")
	}
	// Matching gen installs.
	want := &AnnotationSet{}
	m, _ = m.Update(annotationsReadyMsg{gen: 5, set: want})
	if m.annotations != want {
		t.Errorf("matching gen should install the set")
	}
}

func TestAnnotateRequestStaleDrop(t *testing.T) {
	m := New()
	a := &fakeAnnotator{name: "fake"}
	m.RegisterAnnotator(a)
	m.srcGen = 7
	// Request from gen 6 should not run annotators.
	m, _ = m.Update(annotateRequestMsg{gen: 6})
	if a.calls != 0 {
		t.Errorf("stale request should not invoke annotators; calls = %d", a.calls)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run "TestRegisterAnnotator|TestAnnotateStale|TestAnnotateRequestStale" -v`
Expected: FAIL — `RegisterAnnotator`, `srcGen`, `runAnnotators`, `annotationsReadyMsg`, `annotateRequestMsg` do not yet exist.

- [ ] **Step 3: Add Msg types, runAnnotators, and the schedule helper to annotate.go**

Append to `internal/catkin/annotate.go`:

```go
import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// annotateDebounce is the idle delay after the last edit before
// annotators run. 350 ms keeps typing cheap and matches common
// IDE inline-lint timing.
const annotateDebounce = 350 * time.Millisecond

// annotateRequestMsg fires after the debounce. If gen still
// matches Model.srcGen, annotators run; otherwise the request is
// stale and dropped.
type annotateRequestMsg struct{ gen uint64 }

// annotationsReadyMsg carries a freshly-computed set. Model
// installs it iff gen still matches Model.srcGen.
type annotationsReadyMsg struct {
	gen uint64
	set *AnnotationSet
}

// scheduleAnnotateCmd issues a tick that fires
// annotateRequestMsg{gen} after annotateDebounce.
func scheduleAnnotateCmd(gen uint64) tea.Cmd {
	return tea.Tick(annotateDebounce, func(time.Time) tea.Msg {
		return annotateRequestMsg{gen: gen}
	})
}

// runAnnotatorsCmd runs every annotator over src and returns an
// annotationsReadyMsg{gen, set}. Pure compute; safe inside a Cmd.
func runAnnotatorsCmd(gen uint64, src string, annotators []Annotator) tea.Cmd {
	return func() tea.Msg {
		anns := runAnnotators(annotators, src)
		return annotationsReadyMsg{gen: gen, set: newAnnotationSet(src, anns)}
	}
}

// runAnnotators invokes each registered annotator and merges
// their output. Result is sorted by Range.Start.
func runAnnotators(annotators []Annotator, src string) []Annotation {
	var out []Annotation
	for _, a := range annotators {
		out = append(out, a.Annotate(src)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Range.Start < out[j].Range.Start
	})
	return out
}
```

- [ ] **Step 4: Wire the registry, generation counter, and Update branches into Model**

Edit `internal/catkin/catkin.go`:

In the `Model` struct, add fields:

```go
type Model struct {
	buf         Buffer
	width       int
	height      int
	viewportTop int
	styles      Styles
	undo        undoRing
	mode        DisplayMode
	find        findState

	annotators  []Annotator
	annotations *AnnotationSet
	srcGen      uint64
	annoGen     uint64
}
```

Add the public registration method (place after `New`):

```go
// RegisterAnnotator adds a to the Model's annotator registry.
// Annotators run in registration order; their output merges and
// sorts by Range.Start.
func (m *Model) RegisterAnnotator(a Annotator) {
	m.annotators = append(m.annotators, a)
}
```

In `Update`, add a top-level switch on the new Msg types **before** the `tea.KeyMsg` branch:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch tm := msg.(type) {
	case annotateRequestMsg:
		if tm.gen != m.srcGen {
			return m, nil
		}
		return m, runAnnotatorsCmd(m.srcGen, m.buf.Value(), m.annotators)
	case annotationsReadyMsg:
		if tm.gen == m.srcGen {
			m.annotations = tm.set
			m.annoGen = tm.gen
		}
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		// ...existing key handling...
```

Replace `afterEdit` so every source-mutating edit bumps `srcGen` and schedules an annotate tick:

```go
func (m Model) afterEdit(b Buffer, cmd tea.Cmd) (Model, tea.Cmd) {
	prev := m.buf.Value()
	m.buf = b
	m.recordSnap()
	if m.buf.Value() != prev && len(m.annotators) > 0 {
		m.srcGen++
		cmd = tea.Batch(cmd, scheduleAnnotateCmd(m.srcGen))
	}
	return applyScrollOff(m), cmd
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/catkin/ -run "TestRegisterAnnotator|TestAnnotateStale|TestAnnotateRequestStale" -v`
Expected: PASS.

Run: `go test ./internal/catkin/ -v`
Expected: all existing tests still PASS (no regressions on existing behavior).

- [ ] **Step 6: Commit**

```bash
git add internal/catkin/annotate.go internal/catkin/annotate_test.go internal/catkin/catkin.go
git commit -m "Pass 9d: Annotator registry + debounced-idle scheduling"
```

---

## Task 3: Build the en_US wordlist and embed it

**Files:**

- Create: `scripts/build-wordlist.sh`
- Create: `internal/catkin/spellcheck/en_US.txt` (committed output)
- Create: `internal/catkin/spellcheck/project.txt`
- Create: `internal/catkin/spellcheck/testdata/small_words.txt`

- [ ] **Step 1: Write the wordlist build script**

```bash
# scripts/build-wordlist.sh — one-shot. Output is committed; this
# script exists for reproducibility, not for the build pipeline.
#
# Sources:
#   - github.com/first20hours/google-10000-english (top 10k by frequency).
#   - github.com/dwyl/english-words/words_alpha.txt (~370k alphabetical fill).
#
# Output is frequency-sorted: ranks 1-10000 from the google list,
# remainder appended alphabetically (treated as floor frequency by
# the SymSpell engine).

set -euo pipefail

OUT="$(dirname "$0")/../internal/catkin/spellcheck/en_US.txt"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-no-swears.txt > "$TMP/google.txt"
curl -fsSL https://raw.githubusercontent.com/dwyl/english-words/master/words_alpha.txt > "$TMP/alpha.txt"

# Lowercase + strip non-ASCII-alpha; google list keeps order.
awk '{print tolower($0)}' "$TMP/google.txt" | grep -E '^[a-z]+$' | awk '!seen[$0]++' > "$TMP/freq.txt"

# Alphabetical fill, excluding entries already in freq list.
awk '{print tolower($0)}' "$TMP/alpha.txt" | grep -E '^[a-z]+$' \
	| awk 'NR==FNR{seen[$0]=1; next} !seen[$0]' "$TMP/freq.txt" - \
	| sort -u > "$TMP/fill.txt"

# Cap output at ~50k to bound binary size. 10k freq + first 40k alpha.
head -n 40000 "$TMP/fill.txt" > "$TMP/fill_capped.txt"
cat "$TMP/freq.txt" "$TMP/fill_capped.txt" > "$OUT"

echo "wrote $OUT ($(wc -l < "$OUT") lines)"
```

- [ ] **Step 2: Run the script and commit the output**

```bash
chmod +x scripts/build-wordlist.sh
./scripts/build-wordlist.sh
# Verify line count is in the 45k–55k range.
wc -l internal/catkin/spellcheck/en_US.txt
```

Expected: ~50,000 lines.

- [ ] **Step 3: Author the project.txt allowlist**

```
# internal/catkin/spellcheck/project.txt
# Poplar-specific terms beyond standard en_US. One word per line.
# Lines starting with '#' are comments. Casing here is canonical;
# matching is case-insensitive. Treated as max-frequency by the
# SymSpell engine so project terms beat similar dictionary words
# in suggestions.
catkin
poplar
bubbletea
lipgloss
glamour
chroma
muesli
charm
charmbracelet
emersion
fastmail
jmap
imap
smtp
dav
carddav
caldav
xoauth
oauth
xdg
sqlite
modernc
viewport
textarea
keymap
tmux
nerdfont
nerd
spua
ansi
utf
mime
mbox
maildir
adr
crd
goimports
gofmt
golangci
godoc
struct
const
slice
goroutine
nilable
runtime
toml
markdown
codepoint
hexdigit
linter
lipgloss
memcache
oncall
unmarshal
roundtrip
```

- [ ] **Step 4: Author the test fixture**

```
# internal/catkin/spellcheck/testdata/small_words.txt
the
of
and
to
a
in
is
you
that
it
he
was
for
on
are
as
with
his
they
at
be
this
have
from
or
one
had
by
word
but
not
what
all
were
when
we
there
can
an
your
which
their
said
if
do
will
each
about
how
up
out
them
then
she
many
some
so
these
would
other
into
has
more
her
two
like
him
see
time
could
no
make
than
first
been
its
who
now
people
my
made
over
did
down
only
way
find
use
may
water
long
little
very
after
words
called
just
where
most
know
get
through
back
much
before
go
good
new
write
our
me
man
too
any
day
same
right
look
think
also
around
another
came
come
work
three
must
because
does
part
even
place
well
such
here
take
why
help
put
different
away
again
off
went
old
number
great
tell
men
say
small
every
found
still
between
name
should
home
big
give
air
line
set
own
under
read
last
never
us
left
end
along
while
might
next
sound
below
saw
something
thought
both
few
those
always
show
large
often
together
asked
house
don
world
going
want
school
important
until
form
food
keep
children
feet
land
side
without
boy
once
animal
life
enough
took
sometimes
four
head
above
kind
began
almost
live
page
got
earth
need
far
hand
high
year
mother
light
country
father
let
night
picture
being
study
second
soon
story
since
white
ever
paper
hard
near
sentence
better
best
across
during
today
however
sure
knew
try
told
young
sun
thing
whole
hear
example
heard
several
change
answer
room
sea
against
top
turned
learn
point
city
play
toward
five
himself
usually
money
seen
didn
car
morning
i'm
body
upon
family
later
turn
move
face
door
cut
done
group
true
half
red
fish
plants
living
black
eat
short
united
run
book
gave
order
open
ground
cold
really
table
remember
tree
course
front
american
space
inside
ago
sad
early
i'll
learned
brought
close
nothing
though
idea
before
lived
become
fly
stop
without
second
late
miss
idea
enough
eat
face
watch
far
indian
real
almost
let
above
girl
sometimes
mountain
cut
young
talk
soon
list
song
being
leave
family
it's
```

- [ ] **Step 5: Commit**

```bash
git add scripts/build-wordlist.sh internal/catkin/spellcheck/en_US.txt internal/catkin/spellcheck/project.txt internal/catkin/spellcheck/testdata/small_words.txt
git commit -m "Pass 9d: Bundled wordlists for spellcheck (en_US, project, fixture)"
```

---

## Task 4: Speller engine — Check

**Files:**

- Create: `internal/catkin/spellcheck.go`
- Test: `internal/catkin/spellcheck_test.go`

- [ ] **Step 1: Write failing tests for `Speller.Check`**

```go
// internal/catkin/spellcheck_test.go
package catkin

import (
	"strings"
	"testing"
)

const fixtureExtra = "" // overridden in user-overlay test

func newFixtureSpeller(t *testing.T, extra []string) *Speller {
	t.Helper()
	s, err := newSpellerFromReader(strings.NewReader(fixtureWordlist), nil, extra)
	if err != nil {
		t.Fatalf("newSpellerFromReader: %v", err)
	}
	return s
}

const fixtureWordlist = `the
quick
brown
fox
jumps
over
lazy
dog
tradeoff
markdown
`

func TestSpellerCheckKnown(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	cases := []struct {
		w    string
		want bool
	}{
		{"the", true},
		{"The", true},     // case-insensitive
		{"BROWN", true},   // all caps known word
		{"tradeof", false},
		{"markdwn", false},
		{"", false},       // empty is not a word
	}
	for _, c := range cases {
		if got := s.Check(c.w); got != c.want {
			t.Errorf("Check(%q) = %v, want %v", c.w, got, c.want)
		}
	}
}

func TestSpellerExtraWords(t *testing.T) {
	s := newFixtureSpeller(t, []string{"frobnicate", "quux"})
	if !s.Check("frobnicate") {
		t.Errorf("extra word frobnicate should pass Check")
	}
	if !s.Check("Quux") {
		t.Errorf("extra word Quux should pass Check (case-insensitive)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run TestSpeller -v`
Expected: FAIL with "undefined: Speller" (and similar).

- [ ] **Step 3: Implement Speller.Check + the loaders**

```go
// internal/catkin/spellcheck.go
package catkin

import (
	"bufio"
	"embed"
	"io"
	"strings"
)

//go:embed spellcheck/en_US.txt spellcheck/project.txt
var spellcheckFS embed.FS

// Speller is a lightweight in-memory spellchecker. Construct via
// NewSpeller. The zero value is unsafe — Check on a nil receiver
// returns true (everything passes) so callers without a Speller
// degrade to no-op rather than spurious errors.
type Speller struct {
	known map[string]uint32 // lowercased word → frequency rank (lower = more frequent)
	// SymSpell deletion-distance index, populated in a follow-up
	// task. Speller.Suggest uses it; Check does not.
	delIdx map[string][]string
}

// NewSpeller loads en_US + project from the embedded filesystem
// and unions extra (user wordlist) on top. Extra entries take
// precedence in case-folding and gain max-frequency rank so they
// outrank similar dictionary words in suggestions.
func NewSpeller(extra []string) (*Speller, error) {
	en, err := spellcheckFS.Open("spellcheck/en_US.txt")
	if err != nil {
		return nil, err
	}
	defer en.Close()
	proj, err := spellcheckFS.Open("spellcheck/project.txt")
	if err != nil {
		return nil, err
	}
	defer proj.Close()
	return newSpellerFromReader(en, proj, extra)
}

// newSpellerFromReader is the test-friendly constructor. project
// may be nil.
func newSpellerFromReader(en, project io.Reader, extra []string) (*Speller, error) {
	known := make(map[string]uint32, 50000)
	if err := loadInto(known, en, false); err != nil {
		return nil, err
	}
	if project != nil {
		if err := loadInto(known, project, true); err != nil {
			return nil, err
		}
	}
	for _, w := range extra {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		known[w] = 1 // max frequency for user terms
	}
	return &Speller{known: known}, nil
}

// loadInto reads one-word-per-line wordlists. Comments (#) and
// blank lines are skipped. If maxFreq is true, every entry is
// inserted at rank 1 (project allowlist behavior).
func loadInto(dst map[string]uint32, r io.Reader, maxFreq bool) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	var rank uint32
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w := strings.ToLower(line)
		rank++
		if _, ok := dst[w]; ok {
			continue
		}
		if maxFreq {
			dst[w] = 1
		} else {
			dst[w] = rank
		}
	}
	return sc.Err()
}

// Check reports whether word is in the dictionary. Comparison is
// case-insensitive. Empty strings return false.
func (s *Speller) Check(word string) bool {
	if s == nil {
		return true
	}
	if word == "" {
		return false
	}
	_, ok := s.known[strings.ToLower(word)]
	return ok
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/catkin/ -run TestSpeller -v`
Expected: PASS for `TestSpellerCheckKnown`, `TestSpellerExtraWords`.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/spellcheck.go internal/catkin/spellcheck_test.go
git commit -m "Pass 9d: Speller.Check over embedded en_US + project + extra"
```

---

## Task 5: Speller engine — Suggest (SymSpell deletion-distance)

**Files:**

- Modify: `internal/catkin/spellcheck.go`
- Test: `internal/catkin/spellcheck_test.go`

- [ ] **Step 1: Write failing tests for Suggest**

Append to `spellcheck_test.go`:

```go
func TestSpellerSuggest(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	got := s.Suggest("tradeof", 5)
	if len(got) == 0 || got[0] != "tradeoff" {
		t.Errorf("Suggest(tradeof) = %v, want first suggestion \"tradeoff\"", got)
	}
}

func TestSpellerSuggestRespectsLimit(t *testing.T) {
	s := newFixtureSpeller(t, nil)
	got := s.Suggest("brwn", 3) // close to "brown"
	if len(got) > 3 {
		t.Errorf("Suggest returned %d, want ≤ 3", len(got))
	}
}

func TestSpellerSuggestFrequencyOrder(t *testing.T) {
	// Both "the" and "tea" are within edit distance 2 of "tee".
	// "the" has lower rank (more frequent) so should sort first.
	s, err := newSpellerFromReader(strings.NewReader("the\ntea\n"), nil, nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	got := s.Suggest("tee", 5)
	if len(got) < 2 || got[0] != "the" {
		t.Errorf("Suggest(tee) = %v, want \"the\" first by frequency", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run "TestSpellerSuggest" -v`
Expected: FAIL — `Suggest` not implemented.

- [ ] **Step 3: Implement SymSpell deletion-distance index + Suggest**

Append to `spellcheck.go`:

```go
import (
	"sort"
)

// maxEditDistance bounds the SymSpell delete-prefix expansion.
// SymSpell with d=2 covers >95% of single-word typos in
// English-language inline-spellcheck workloads while keeping the
// index size modest (~10–20 MB resident for top-50k).
const maxEditDistance = 2

// buildIndex populates s.delIdx. Each known word generates the
// set of deletes within maxEditDistance, and each delete maps
// back to the originating word. Suggest then deletes the input
// candidate, looks up the resulting key, and verifies real
// edit distance ≤ maxEditDistance against each candidate.
func (s *Speller) buildIndex() {
	s.delIdx = make(map[string][]string, len(s.known)*4)
	for w := range s.known {
		for d := range deletes(w, maxEditDistance) {
			s.delIdx[d] = append(s.delIdx[d], w)
		}
	}
}

// deletes returns the set of distinct strings produced by deleting
// up to dist characters from w. The result includes w itself
// (zero deletions).
func deletes(w string, dist int) map[string]struct{} {
	out := map[string]struct{}{w: {}}
	if dist <= 0 || len(w) == 0 {
		return out
	}
	for i := 0; i < len(w); i++ {
		shorter := w[:i] + w[i+1:]
		if _, seen := out[shorter]; seen {
			continue
		}
		out[shorter] = struct{}{}
		for d := range deletes(shorter, dist-1) {
			out[d] = struct{}{}
		}
	}
	return out
}

// editDistance is plain Levenshtein, capped at limit. Returns
// limit+1 if the true distance exceeds limit.
func editDistance(a, b string, limit int) int {
	la, lb := len(a), len(b)
	if abs(la-lb) > limit {
		return limit + 1
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if curr[j] < rowMin {
				rowMin = curr[j]
			}
		}
		if rowMin > limit {
			return limit + 1
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// Suggest returns up to n correction candidates for word, ordered
// by (edit distance ascending, frequency rank ascending).
func (s *Speller) Suggest(word string, n int) []string {
	if s == nil || word == "" || n <= 0 {
		return nil
	}
	if s.delIdx == nil {
		s.buildIndex()
	}
	w := strings.ToLower(word)

	type cand struct {
		word string
		dist int
		rank uint32
	}
	seen := map[string]struct{}{}
	var cands []cand
	for d := range deletes(w, maxEditDistance) {
		for _, candidate := range s.delIdx[d] {
			if _, dup := seen[candidate]; dup {
				continue
			}
			seen[candidate] = struct{}{}
			ed := editDistance(w, candidate, maxEditDistance)
			if ed > maxEditDistance {
				continue
			}
			cands = append(cands, cand{word: candidate, dist: ed, rank: s.known[candidate]})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].rank < cands[j].rank
	})
	if len(cands) > n {
		cands = cands[:n]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.word
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/catkin/ -run "TestSpellerSuggest" -v`
Expected: PASS for all three Suggest tests.

Run: `go test ./internal/catkin/ -v`
Expected: full package green.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/spellcheck.go internal/catkin/spellcheck_test.go
git commit -m "Pass 9d: Speller.Suggest via SymSpell deletion-distance"
```

---

## Task 6: LoadUserWordlist helper

**Files:**

- Modify: `internal/catkin/spellcheck.go`
- Test: `internal/catkin/spellcheck_test.go`

- [ ] **Step 1: Write failing tests**

Append to `spellcheck_test.go`:

```go
func TestLoadUserWordlistMissing(t *testing.T) {
	got, err := LoadUserWordlist(t.TempDir() + "/does-not-exist.txt")
	if err != nil {
		t.Errorf("missing file should not be an error; got %v", err)
	}
	if got != nil {
		t.Errorf("missing file should return nil; got %v", got)
	}
}

func TestLoadUserWordlistParses(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/wordlist.txt"
	body := "# comment\nfrobnicate\n\nQuux\n   spaced   \n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadUserWordlist(path)
	if err != nil {
		t.Fatalf("LoadUserWordlist: %v", err)
	}
	want := []string{"frobnicate", "Quux", "spaced"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadUserWordlist = %v, want %v", got, want)
	}
}
```

Add the new imports at the top of `spellcheck_test.go`:

```go
import (
	"os"
	"reflect"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run "TestLoadUserWordlist" -v`
Expected: FAIL with "undefined: LoadUserWordlist".

- [ ] **Step 3: Implement**

Append to `spellcheck.go`:

```go
import (
	"errors"
	"os"
)

// LoadUserWordlist reads one-word-per-line entries from path.
// Comments ('#') and blank lines are skipped. Whitespace is
// trimmed. A missing file is not an error and returns (nil, nil).
// Casing is preserved — the Speller lowercases at lookup time.
func LoadUserWordlist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/catkin/ -run "TestLoadUserWordlist" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/spellcheck.go internal/catkin/spellcheck_test.go
git commit -m "Pass 9d: LoadUserWordlist helper"
```

---

## Task 7: Spellcheck annotator with skip masks

**Files:**

- Modify: `internal/catkin/spellcheck.go`
- Test: `internal/catkin/spellcheck_test.go`

- [ ] **Step 1: Write failing tests**

Append to `spellcheck_test.go`:

```go
func TestSpellcheckAnnotator(t *testing.T) {
	speller := newFixtureSpeller(t, nil)
	a := NewSpellcheckAnnotator(speller)

	cases := []struct {
		name      string
		src       string
		wantRanges []Range
	}{
		{
			name:       "single misspelling",
			src:        "the tradeof is real",
			wantRanges: []Range{{4, 11}},
		},
		{
			name:       "no misspellings",
			src:        "the quick brown fox",
			wantRanges: nil,
		},
		{
			name:       "skip inside fenced code",
			src:        "the\n```go\nquxxxxxx\n```\nover",
			wantRanges: nil,
		},
		{
			name:       "skip inside inline code",
			src:        "the `quxxxxxx` over",
			wantRanges: nil,
		},
		{
			name:       "skip inside link URL",
			src:        "the [quick](http://exampole.com/foo) over",
			wantRanges: nil,
		},
		{
			name:       "skip all-caps acronym <=4 runes",
			src:        "JMAP and HTTP and IMAP",
			wantRanges: nil,
		},
		{
			name:       "all-caps >4 still checked",
			src:        "QUXXXXX",
			wantRanges: []Range{{0, 7}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := a.Annotate(c.src)
			if len(got) != len(c.wantRanges) {
				t.Fatalf("Annotate(%q) returned %d ranges, want %d: %v", c.src, len(got), len(c.wantRanges), got)
			}
			for i, want := range c.wantRanges {
				if got[i].Range != want {
					t.Errorf("range %d = %v, want %v", i, got[i].Range, want)
				}
				if got[i].Kind != KindMisspelling {
					t.Errorf("range %d kind = %v, want KindMisspelling", i, got[i].Kind)
				}
			}
		})
	}
}

func TestSpellcheckAnnotatorPayload(t *testing.T) {
	speller := newFixtureSpeller(t, nil)
	a := NewSpellcheckAnnotator(speller)
	got := a.Annotate("tradeof here")
	if len(got) != 1 {
		t.Fatalf("Annotate returned %d, want 1", len(got))
	}
	mp, ok := got[0].Payload.(MisspellingPayload)
	if !ok {
		t.Fatalf("Payload type = %T, want MisspellingPayload", got[0].Payload)
	}
	if mp.Word != "tradeof" {
		t.Errorf("Word = %q, want \"tradeof\"", mp.Word)
	}
	if len(mp.Suggestions) == 0 || mp.Suggestions[0] != "tradeoff" {
		t.Errorf("Suggestions = %v, want first \"tradeoff\"", mp.Suggestions)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run TestSpellcheckAnnotator -v`
Expected: FAIL with "undefined: NewSpellcheckAnnotator".

- [ ] **Step 3: Implement**

Append to `spellcheck.go`:

```go
import (
	"unicode"
)

// NewSpellcheckAnnotator wires speller into the Annotator
// interface. Construct once at host setup.
func NewSpellcheckAnnotator(speller *Speller) Annotator {
	return &spellcheckAnnotator{
		speller: speller,
		ignored: map[string]struct{}{},
	}
}

type spellcheckAnnotator struct {
	speller *Speller
	ignored map[string]struct{} // session-only adds
}

func (s *spellcheckAnnotator) Name() string { return "spellcheck" }

func (s *spellcheckAnnotator) Annotate(src string) []Annotation {
	if s.speller == nil {
		return nil
	}
	mask := buildSkipMask(src)
	var out []Annotation
	i := 0
	for i < len(src) {
		// Walk to the next word-rune.
		r, size := utf8RuneAt(src, i)
		if !isWordRune(r) {
			i += size
			continue
		}
		start := i
		for i < len(src) {
			r, size = utf8RuneAt(src, i)
			if !isWordRune(r) {
				break
			}
			i += size
		}
		end := i
		if mask.covers(start, end) {
			continue
		}
		word := src[start:end]
		if isAllUpperShort(word) {
			continue
		}
		if _, ig := s.ignored[strings.ToLower(word)]; ig {
			continue
		}
		if s.speller.Check(word) {
			continue
		}
		out = append(out, Annotation{
			Range:   Range{Start: start, End: end},
			Kind:    KindMisspelling,
			Payload: MisspellingPayload{
				Word:        word,
				Suggestions: s.speller.Suggest(word, 5),
			},
		})
	}
	return out
}

// IgnoreInSession adds word to the in-memory ignore set. Subsequent
// Annotate calls treat it as known until the annotator is rebuilt.
// Lowercased before storage; matching is case-insensitive.
func (s *spellcheckAnnotator) IgnoreInSession(word string) {
	s.ignored[strings.ToLower(word)] = struct{}{}
}

// utf8RuneAt is a small helper to avoid importing unicode/utf8 at
// every call site. Returns (utf8.RuneError, 1) for invalid bytes;
// callers treat that as a non-word-rune which is what we want.
func utf8RuneAt(s string, i int) (rune, int) {
	for off, r := range s[i:] {
		_ = off
		return r, len(string(r))
	}
	return 0, 0
}

// isAllUpperShort reports whether word is all-uppercase and ≤4
// runes — a heuristic that skips common acronyms (HTTP, JMAP)
// without a dedicated allowlist.
func isAllUpperShort(word string) bool {
	n := 0
	for _, r := range word {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
		n++
		if n > 4 {
			return false
		}
	}
	return n > 0
}

// skipMask is a sorted, non-overlapping set of byte ranges that
// the spellchecker must not flag (code blocks, code spans, link
// URLs).
type skipMask struct {
	ranges []Range
}

// covers reports whether [start, end) is fully inside any masked range.
func (m skipMask) covers(start, end int) bool {
	for _, r := range m.ranges {
		if start >= r.Start && end <= r.End {
			return true
		}
	}
	return false
}

// buildSkipMask scans src for fenced blocks, inline code spans,
// and link URLs, returning the union of their byte ranges.
func buildSkipMask(src string) skipMask {
	var ranges []Range
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)

	// Fenced blocks: union the byte ranges of all inside-fence lines.
	off := 0
	for i, l := range lines {
		lineStart := off
		lineEnd := off + len(l)
		if ctxs[i].InsideFence {
			ranges = append(ranges, Range{Start: lineStart, End: lineEnd})
		}
		off = lineEnd + 1 // +1 for the '\n'
	}

	// Inline code + link URL via walkSpans, with a per-line offset.
	off = 0
	for _, l := range lines {
		lineStart := off
		walkSpans(l, func(kind spanKind, text string, sub []string) {
			switch kind {
			case spanCode:
				idx := strings.Index(l[lineStart-lineStart:], text) // local find
				_ = idx
				// We can't trust sub-offsets from walkSpans — recompute
				// against the raw line.
				if start := strings.Index(l, text); start >= 0 {
					ranges = append(ranges, Range{Start: lineStart + start, End: lineStart + start + len(text)})
				}
			case spanLink:
				if len(sub) >= 2 {
					urlText := sub[1]
					if start := strings.Index(l, urlText); start >= 0 {
						ranges = append(ranges, Range{Start: lineStart + start, End: lineStart + start + len(urlText)})
					}
				}
			}
		})
		off = lineStart + len(l) + 1
	}

	return skipMask{ranges: ranges}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/catkin/ -run TestSpellcheckAnnotator -v`
Expected: PASS for all subtests of `TestSpellcheckAnnotator` and `TestSpellcheckAnnotatorPayload`.

> **Note for the implementer:** if the inline-code or link-URL skip mask logic above proves brittle on real source (the "find the text inside the line" approach is approximate), refactor `walkSpans` in `style.go` to expose absolute offsets via a new internal helper rather than reverse-engineering them here. Keep the public API stable.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/spellcheck.go internal/catkin/spellcheck_test.go
git commit -m "Pass 9d: spellcheck annotator + code/inline/URL skip masks"
```

---

## Task 8: Squiggle render — Styles.Squiggle and per-cell decoration

**Files:**

- Modify: `internal/catkin/style.go`
- Modify: `internal/catkin/render.go`
- Modify: `internal/catkin/catkin.go` (thread `m.annotations` into `Render`)
- Test: `internal/catkin/render_test.go`

- [ ] **Step 1: Add Styles.Squiggle**

Edit `internal/catkin/style.go`, in the `Styles` struct definition (around line 26):

```go
type Styles struct {
	Heading    [6]lipgloss.Style
	Quote      lipgloss.Style
	DeepQuote  lipgloss.Style
	Bold       lipgloss.Style
	Italic     lipgloss.Style
	BoldItalic lipgloss.Style
	CodeInline lipgloss.Style
	CodeBlock  lipgloss.Style
	Link       lipgloss.Style
	URL        lipgloss.Style
	ListMarker     lipgloss.Style
	TaskBox        lipgloss.Style
	MatchHighlight lipgloss.Style
	Dim            lipgloss.Style
	Squiggle       lipgloss.Style // misspelling decoration
	Popover        lipgloss.Style // suggestion popover frame
	PopoverSelected lipgloss.Style // highlighted suggestion row
}
```

- [ ] **Step 2: Write a failing render test**

Append to `internal/catkin/render_test.go`:

```go
func TestRenderAppliesSquiggle(t *testing.T) {
	src := "the tradeof is real"
	anns := []Annotation{
		{
			Range: Range{4, 11},
			Kind:  KindMisspelling,
			Style: lipgloss.NewStyle().Underline(true),
		},
	}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 0, Styles{}, ModeNormal, set)
	if !strings.Contains(out, "\x1b[4m") { // underline SGR
		t.Errorf("rendered output missing underline SGR; got:\n%q", out)
	}
}
```

(If the existing test file uses a different name for the normal mode constant, substitute it — check `mode.go` for the canonical identifier.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/catkin/ -run TestRenderAppliesSquiggle -v`
Expected: FAIL with "undefined: RenderAnnotated".

- [ ] **Step 4: Add RenderAnnotated and overlay logic**

Edit `internal/catkin/render.go`. Keep `Render` as a thin shim for back-compat:

```go
// Render is the back-compat entry point. Equivalent to
// RenderAnnotated with a nil AnnotationSet.
func Render(src string, width, height, top, cursor int, styles Styles, mode DisplayMode) string {
	return RenderAnnotated(src, width, height, top, cursor, styles, mode, nil)
}

// RenderAnnotated produces Catkin's view content with optional
// annotations overlaid before the cursor block. ann may be nil.
func RenderAnnotated(src string, width, height, top, cursor int, styles Styles, mode DisplayMode, ann *AnnotationSet) string {
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, cursor)
	fenceLines := renderFences(lines, ctxs, styles, top, top+height)
	focusFirst, focusLast := -1, -1
	if mode.focus() {
		focusFirst, focusLast = activeParagraphRange(ctxs, cursorRow)
	}

	// rowOffsets[i] is the byte offset of the start of line i in src.
	rowOffsets := computeRowOffsets(src)

	var visual []string
	for i := top; i < len(lines) && len(visual) < height; i++ {
		raw := lines[i]
		var matchCol = -1
		var matchCh rune
		if i == cursorRow {
			if mc, ok := bracketMatchAt(raw, cursorCol); ok && mc != cursorCol {
				matchRunes := []rune(raw)
				if mc < len(matchRunes) {
					matchCh = matchRunes[mc]
					matchCol = lipgloss.Width(string(matchRunes[:mc]))
				}
			}
			raw = insertCursorBlock(raw, cursorCol)
		}
		styled := styleLine(raw, ctxs[i], styles, fenceLines, i, i == cursorRow)
		if ann != nil {
			styled = applyAnnotationsToLine(styled, raw, lines[i], rowOffsets[i], ann.rangesOnRow(src, i), styles)
		}
		if matchCol >= 0 {
			styled = overlayMatch(styled, matchCol, matchCh, styles.MatchHighlight)
		}
		if mode.focus() && (i < focusFirst || i > focusLast) {
			styled = styles.Dim.Render(ansi.Strip(styled))
		}
		for _, w := range softWrap(styled, width) {
			if len(visual) >= height {
				break
			}
			visual = append(visual, w)
		}
	}
	for len(visual) < height {
		visual = append(visual, "")
	}
	return strings.Join(visual, "\n")
}

func computeRowOffsets(src string) []int {
	out := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}

// applyAnnotationsToLine wraps the byte ranges from anns (relative
// to src) in the corresponding Annotation.Style, layered on top of
// the already-styled line.
//
// Implementation detail: we strip ANSI from styled, splice the
// annotation styles around the relevant byte ranges in the plain
// version, then re-apply the prior styling via a per-cell merge.
// For Pass 9d only one annotator is registered (spellcheck) so the
// straightforward path is acceptable: build a styled string by
// concatenating substrings, applying annotation Style.Render to
// the inside ranges.
func applyAnnotationsToLine(styled, raw, plain string, lineOffset int, anns []Annotation, st Styles) string {
	if len(anns) == 0 {
		return styled
	}
	// Rebuild the line plain-then-styled with annotation styles
	// layered. Because styleLine already produced ANSI-styled
	// output, and lipgloss styles compose left-to-right within a
	// line, the simplest approach is to re-style from raw with
	// both the original markdown styling AND the annotation
	// styling. For 9d we accept the loss-of-fidelity for the
	// markdown layer on annotated cells (squiggle wins) and
	// re-render using styleLine on slices.
	//
	// Strategy: split raw into [pre, target, post] for each
	// annotation. The target is styled with Annotation.Style; the
	// pre/post halves keep the existing styled output via
	// substring extraction over styled. Since multiple
	// annotations on one line are rare for spellcheck, iterate.
	out := styled
	for _, a := range anns {
		startInLine := a.Range.Start - lineOffset
		endInLine := a.Range.End - lineOffset
		if startInLine < 0 {
			startInLine = 0
		}
		if endInLine > len(plain) {
			endInLine = len(plain)
		}
		if startInLine >= endInLine {
			continue
		}
		// Compute the column (display-width) position of [startInLine, endInLine)
		// against plain; replace that slice in the styled string by
		// re-styling the original raw substring.
		preCol := lipgloss.Width(plain[:startInLine])
		target := raw[startInLine:endInLine]
		styledTarget := a.Style.Render(target)
		out = ansiSpliceAtCol(out, preCol, lipgloss.Width(target), styledTarget)
	}
	return out
}

// ansiSpliceAtCol replaces a fixed-width column range of an
// ANSI-styled string with replacement. Implementation uses
// charmbracelet/x/ansi.Truncate to slice around the target. If
// the styled string is shorter than expected (trailing whitespace
// trimmed), the call falls back to padding with raw spaces.
func ansiSpliceAtCol(styled string, col, width int, replacement string) string {
	left := ansi.Truncate(styled, col, "")
	rest := ansi.TruncateLeft(styled, col, "")
	right := ansi.TruncateLeft(rest, width, "")
	return left + replacement + right
}
```

- [ ] **Step 5: Thread the annotation set through Model.View**

Edit `internal/catkin/catkin.go`'s `View` to use `RenderAnnotated`:

```go
func (m Model) View() string {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	body := RenderAnnotated(src, m.width, m.height-m.find.footerRows(), m.viewportTop, cur, m.styles, m.mode, m.annotations)
	if !m.find.active() {
		return body
	}
	return body + "\n" + m.find.renderFindFooter(m.width)
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/catkin/ -run TestRenderAppliesSquiggle -v`
Expected: PASS.

Run: `go test ./internal/catkin/ -v`
Expected: full package green.

- [ ] **Step 7: Commit**

```bash
git add internal/catkin/style.go internal/catkin/render.go internal/catkin/catkin.go internal/catkin/render_test.go
git commit -m "Pass 9d: Squiggle decoration via RenderAnnotated"
```

---

## Task 9: Popover state, key bindings, lifecycle

**Files:**

- Create: `internal/catkin/popover.go`
- Test: `internal/catkin/popover_test.go`
- Modify: `internal/catkin/catkin.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/catkin/popover_test.go
package catkin

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newModelWithMisspelling(t *testing.T) (Model, Range) {
	t.Helper()
	speller := newFixtureSpeller(t, nil)
	m := New()
	m.SetSize(80, 10)
	m.SetValue("the tradeof is real")
	m.RegisterAnnotator(NewSpellcheckAnnotator(speller))
	// Run annotators inline, bypassing the tick.
	anns := runAnnotators(m.annotators, m.buf.Value())
	m.annotations = newAnnotationSet(m.buf.Value(), anns)
	if len(anns) != 1 {
		t.Fatalf("expected one annotation, got %d", len(anns))
	}
	return m, anns[0].Range
}

func TestPopoverOpensOnRange(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start) // cursor on misspelling
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	if !m.popover.open {
		t.Errorf("Ctrl+; on misspelling should open popover")
	}
	if m.popover.word != "tradeof" {
		t.Errorf("popover.word = %q, want \"tradeof\"", m.popover.word)
	}
}

func TestPopoverCloseOnEsc(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.popover.open {
		t.Errorf("Esc should close popover")
	}
}

func TestPopoverApplyReplacesRange(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // apply first suggestion
	if got := m.buf.Value(); got != "the tradeoff is real" {
		t.Errorf("after apply: %q, want \"the tradeoff is real\"", got)
	}
	if m.popover.open {
		t.Errorf("apply should close popover")
	}
}

func TestPopoverDoesNotOpenOffRange(t *testing.T) {
	m, _ := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(0) // cursor on "the", not on misspelling
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	if m.popover.open {
		t.Errorf("Ctrl+; off a misspelling should not open popover")
	}
}

func TestPopoverDigitJumpApply(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	if len(m.popover.suggestions) < 1 {
		t.Skipf("no suggestions; cannot exercise digit jump")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.popover.open {
		t.Errorf("digit jump should close popover")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run TestPopover -v`
Expected: FAIL — `m.popover` field, key handler, and types undefined.

- [ ] **Step 3: Implement popover state and Update integration**

```go
// internal/catkin/popover.go
package catkin

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// popoverMaxSuggestions caps the suggestion list. 5 matches the
// OS-native context-menu convention (macOS, Firefox, Chrome) and
// stays inside SymSpell's high-quality top-of-list ranking.
const popoverMaxSuggestions = 5

// popoverState is Catkin's misspelling-suggestion overlay. The
// zero value is closed.
type popoverState struct {
	open        bool
	wordRange   Range
	word        string
	suggestions []string
	cursor      int // selected suggestion index
}

var popoverKeys = struct {
	Open, Close                   key.Binding
	Next, Prev, Apply             key.Binding
	Add, Ignore                   key.Binding
}{
	Open:   key.NewBinding(key.WithKeys("ctrl+;")),
	Close:  key.NewBinding(key.WithKeys("esc")),
	Next:   key.NewBinding(key.WithKeys("down", "ctrl+n")),
	Prev:   key.NewBinding(key.WithKeys("up", "ctrl+p")),
	Apply:  key.NewBinding(key.WithKeys("enter")),
	Add:    key.NewBinding(key.WithKeys("a")),
	Ignore: key.NewBinding(key.WithKeys("i")),
}

// findMisspellingAt returns the annotation whose range covers off,
// or nil if none does.
func (m Model) findMisspellingAt(off int) *Annotation {
	if m.annotations == nil {
		return nil
	}
	for i := range m.annotations.All {
		a := &m.annotations.All[i]
		if a.Kind != KindMisspelling {
			continue
		}
		if a.Range.Contains(off) {
			return a
		}
	}
	return nil
}

// openPopover seeds popoverState from an annotation under the
// cursor and closes any active find shelf.
func (m Model) openPopover(a *Annotation) Model {
	mp, ok := a.Payload.(MisspellingPayload)
	if !ok {
		return m
	}
	if m.find.active() {
		m.find = findState{}
	}
	m.popover = popoverState{
		open:        true,
		wordRange:   a.Range,
		word:        mp.Word,
		suggestions: trimTo(mp.Suggestions, popoverMaxSuggestions),
		cursor:      0,
	}
	return m
}

// closePopover resets the overlay to its zero state.
func (m Model) closePopover() Model {
	m.popover = popoverState{}
	return m
}

// handlePopoverKey dispatches a KeyMsg while the popover is open.
// Returns (handled, model, cmd). When handled is false, the
// caller falls through to normal Update handling.
func (m Model) handlePopoverKey(k tea.KeyMsg) (bool, Model, tea.Cmd) {
	switch {
	case key.Matches(k, popoverKeys.Close):
		return true, m.closePopover(), nil
	case key.Matches(k, popoverKeys.Next):
		if len(m.popover.suggestions) > 0 {
			m.popover.cursor = (m.popover.cursor + 1) % len(m.popover.suggestions)
		}
		return true, m, nil
	case key.Matches(k, popoverKeys.Prev):
		if n := len(m.popover.suggestions); n > 0 {
			m.popover.cursor = (m.popover.cursor - 1 + n) % n
		}
		return true, m, nil
	case key.Matches(k, popoverKeys.Apply):
		return true, m.applySelectedSuggestion(), nil
	case key.Matches(k, popoverKeys.Ignore):
		return true, m.ignoreCurrentWord(), nil
	case key.Matches(k, popoverKeys.Add):
		// Add-to-wordlist is implemented in Task 10.
		return true, m.closePopover(), nil
	}
	// Digit jump-and-apply.
	if len(k.Runes) == 1 && k.Runes[0] >= '1' && k.Runes[0] <= '9' {
		idx := int(k.Runes[0] - '1')
		if idx < len(m.popover.suggestions) {
			m.popover.cursor = idx
			return true, m.applySelectedSuggestion(), nil
		}
		return true, m, nil
	}
	return false, m, nil
}

func (m Model) applySelectedSuggestion() Model {
	if !m.popover.open || len(m.popover.suggestions) == 0 {
		return m.closePopover()
	}
	repl := m.popover.suggestions[m.popover.cursor]
	src := m.buf.Value()
	r := m.popover.wordRange
	if r.Start < 0 || r.End > len(src) || r.Start >= r.End {
		return m.closePopover()
	}
	newSrc := src[:r.Start] + repl + src[r.End:]
	m.buf.SetValue(newSrc)
	// Position cursor at end of the replacement (rune-counted).
	prefixRunes := len([]rune(src[:r.Start]))
	replRunes := len([]rune(repl))
	m.buf.SetRuneOffset(prefixRunes + replRunes)
	m.recordSnap()
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
		return m // caller batches the schedule cmd; see Update integration
	}
	return m
}

func (m Model) ignoreCurrentWord() Model {
	for _, a := range m.annotators {
		if sa, ok := a.(*spellcheckAnnotator); ok {
			sa.IgnoreInSession(m.popover.word)
		}
	}
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
	}
	return m
}

func trimTo(xs []string, n int) []string {
	if len(xs) <= n {
		out := make([]string, len(xs))
		copy(out, xs)
		return out
	}
	out := make([]string, n)
	copy(out, xs[:n])
	return out
}

// keep imports honest for future expansion
var _ = strings.Builder{}
```

- [ ] **Step 4: Wire popover into Model.Update**

Edit `internal/catkin/catkin.go`. In the `Model` struct, add the `popover` field:

```go
type Model struct {
	// ...existing fields...
	annotators  []Annotator
	annotations *AnnotationSet
	srcGen      uint64
	annoGen     uint64
	popover     popoverState
}
```

In `Update`, after the annotation Msg switch and before the existing key handling, add the popover branch:

```go
if k, ok := msg.(tea.KeyMsg); ok {
	if m.popover.open {
		if handled, mm, cmd := m.handlePopoverKey(k); handled {
			cmd = m.maybeScheduleAnnotateAfterMutation(mm, cmd)
			return mm, cmd
		}
	}
	// Open popover if cursor is on a misspelling and Ctrl+; pressed.
	if !m.popover.open && key.Matches(k, popoverKeys.Open) {
		if a := m.findMisspellingAt(byteOffsetForRune(m.buf.Value(), m.buf.RuneOffset())); a != nil {
			return m.openPopover(a), nil
		}
	}
	// ...existing key handling continues...
}
```

Add the helpers:

```go
// byteOffsetForRune translates a rune offset to a byte offset
// against src.
func byteOffsetForRune(src string, runeOff int) int {
	if runeOff <= 0 {
		return 0
	}
	count := 0
	for i := range src {
		if count == runeOff {
			return i
		}
		count++
	}
	return len(src)
}

// maybeScheduleAnnotateAfterMutation issues an annotate-tick if the
// model's srcGen advanced (apply / ignore both bump it). Returns
// the merged Cmd.
func (m Model) maybeScheduleAnnotateAfterMutation(mm Model, cmd tea.Cmd) tea.Cmd {
	if mm.srcGen != m.srcGen && len(mm.annotators) > 0 {
		return tea.Batch(cmd, scheduleAnnotateCmd(mm.srcGen))
	}
	return cmd
}
```

Add the import for `key`:

```go
import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/catkin/ -run TestPopover -v`
Expected: PASS for all popover tests.

Run: `go test ./internal/catkin/ -v`
Expected: full package green. If `TestPopoverOpensOnRange` fails because `tea.KeyCtrlSemicolon` is not a defined constant in your bubbletea version, change the test to construct the key as:

```go
tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}, Alt: false}
```

and adjust `popoverKeys.Open` to `key.NewBinding(key.WithKeys("ctrl+;"))` — bubbletea matches `ctrl+;` against `KeyMsg.String()` regardless of the canonical Type.

- [ ] **Step 6: Commit**

```bash
git add internal/catkin/popover.go internal/catkin/popover_test.go internal/catkin/catkin.go
git commit -m "Pass 9d: popover state machine + key dispatch"
```

---

## Task 10: AddToWordlist persistence

**Files:**

- Modify: `internal/catkin/popover.go`
- Modify: `internal/catkin/catkin.go` (carry the user-wordlist path)
- Test: `internal/catkin/popover_test.go`

- [ ] **Step 1: Write failing test**

Append to `popover_test.go`:

```go
func TestPopoverAddToWordlist(t *testing.T) {
	dir := t.TempDir()
	listPath := dir + "/wordlist.txt"

	m, r := newModelWithMisspelling(t)
	m.SetUserWordlistPath(listPath)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	got, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatalf("read wordlist: %v", err)
	}
	if !strings.Contains(string(got), "tradeof") {
		t.Errorf("wordlist missing \"tradeof\":\n%s", got)
	}
	if m.popover.open {
		t.Errorf("AddToWordlist should close popover")
	}
}
```

Add the imports `"os"` and `"strings"` to `popover_test.go` if missing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/catkin/ -run TestPopoverAddToWordlist -v`
Expected: FAIL — `SetUserWordlistPath` undefined or no-op.

- [ ] **Step 3: Implement persistence**

In `catkin.go`:

```go
type Model struct {
	// ...existing fields...
	userWordlistPath string
}

// SetUserWordlistPath sets the file the popover's add-action
// appends to. Empty path disables persistence (add becomes a no-op
// session-local addition like ignore).
func (m *Model) SetUserWordlistPath(path string) {
	m.userWordlistPath = path
}
```

In `popover.go`, replace the `popoverKeys.Add` branch in `handlePopoverKey`:

```go
case key.Matches(k, popoverKeys.Add):
	return true, m.addCurrentWordToWordlist(), nil
```

And add the implementation:

```go
import (
	"fmt"
	"os"
)

func (m Model) addCurrentWordToWordlist() Model {
	word := m.popover.word
	if m.userWordlistPath != "" {
		appendUserWord(m.userWordlistPath, word)
	}
	// Update each speller-backed annotator in-place so the next
	// Annotate run treats the word as known.
	for _, a := range m.annotators {
		if sa, ok := a.(*spellcheckAnnotator); ok && sa.speller != nil {
			sa.speller.known[strings.ToLower(word)] = 1
		}
	}
	m = m.closePopover()
	if len(m.annotators) > 0 {
		m.srcGen++
	}
	return m
}

// appendUserWord opens path in append mode (creating it 0o600 if
// missing) and writes word + "\n". Errors are intentionally
// swallowed: failure to persist is recoverable (the in-memory
// speller still picks up the word for this session) and surfacing
// it from a key handler would require an error channel that this
// pass deliberately does not introduce.
func appendUserWord(path, word string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", word)
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/catkin/ -run TestPopoverAddToWordlist -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/popover.go internal/catkin/catkin.go internal/catkin/popover_test.go
git commit -m "Pass 9d: AddToWordlist persistence + in-memory speller update"
```

---

## Task 11: Popover render and overlay positioning

**Files:**

- Modify: `internal/catkin/popover.go` (render funcs)
- Modify: `internal/catkin/catkin.go` (compose into View)
- Test: `internal/catkin/popover_test.go`

- [ ] **Step 1: Write failing tests for render shape**

Append to `popover_test.go`:

```go
func TestPopoverRenderShape(t *testing.T) {
	pop := popoverState{
		open:        true,
		word:        "tradeof",
		suggestions: []string{"tradeoff", "trade-off"},
		cursor:      0,
	}
	out := pop.render(40, Styles{})
	lines := strings.Split(out, "\n")
	// header + 2 suggestions + blank separator + actions + 2 borders = 7
	if len(lines) != 7 {
		t.Fatalf("popover lines = %d, want 7\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "tradeof") {
		t.Errorf("header line missing word: %q", lines[0])
	}
	if !strings.Contains(out, "tradeoff") || !strings.Contains(out, "trade-off") {
		t.Errorf("render missing suggestions:\n%s", out)
	}
}

func TestPopoverRenderZeroSuggestions(t *testing.T) {
	pop := popoverState{open: true, word: "qzwxy", suggestions: nil}
	out := pop.render(40, Styles{})
	lines := strings.Split(out, "\n")
	// header + actions + 2 borders = 4
	if len(lines) != 4 {
		t.Fatalf("zero-suggestion popover lines = %d, want 4\n%s", len(lines), out)
	}
}

func TestPopoverPositionBelow(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y"}}
	row, col := pop.position(2, 5, 80, 24)
	if row != 3 {
		t.Errorf("position row below cursor = %d, want 3", row)
	}
	if col != 5 {
		t.Errorf("position col = %d, want 5", col)
	}
}

func TestPopoverPositionAboveOnNoRoom(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y", "z"}}
	// Cursor near bottom — popover would overflow if drawn below.
	row, _ := pop.position(22, 0, 80, 24)
	if row >= 22 {
		t.Errorf("position row near bottom = %d, want above cursor", row)
	}
}

func TestPopoverHorizontalClamp(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y"}}
	_, col := pop.position(2, 78, 80, 24) // anchor near right edge
	if col+pop.width() > 80 {
		t.Errorf("position col+width = %d, want ≤ 80", col+pop.width())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/catkin/ -run "TestPopoverRender|TestPopoverPosition|TestPopoverHorizontal" -v`
Expected: FAIL — render/position/width undefined.

- [ ] **Step 3: Implement render and positioning**

Append to `popover.go`:

```go
import (
	"github.com/charmbracelet/lipgloss"
)

// width returns the popover's outer width including borders. Sized
// to the longest content row, capped at 32 cols (popover stays
// out of the way of long lines).
func (p popoverState) width() int {
	const cap = 32
	max := lipgloss.Width(`"` + p.word + `"`)
	for _, s := range p.suggestions {
		if w := lipgloss.Width("  " + s); w > max {
			max = w
		}
	}
	if w := lipgloss.Width("a add  i ignore  esc x"); w > max {
		max = w
	}
	if max > cap {
		max = cap
	}
	return max + 2 // borders
}

// height returns the popover's outer height including borders.
// header (1) + suggestions (N) + separator (0 or 1) + actions (1) + borders (2)
func (p popoverState) height() int {
	if len(p.suggestions) == 0 {
		return 4
	}
	return 4 + len(p.suggestions)
}

// render returns the framed popover content. width arg is the
// outer width to use; if 0, the natural width() is used.
func (p popoverState) render(_ int, st Styles) string {
	innerW := p.width() - 2
	header := lipgloss.NewStyle().Width(innerW).Render(`"` + p.word + `"`)
	var rows []string
	rows = append(rows, header)
	for i, s := range p.suggestions {
		marker := "  "
		if i == p.cursor {
			marker = "> "
		}
		row := lipgloss.NewStyle().Width(innerW).Render(marker + s)
		if i == p.cursor && st.PopoverSelected.GetForeground() != nil {
			row = st.PopoverSelected.Render(marker + s)
		}
		rows = append(rows, row)
	}
	if len(p.suggestions) > 0 {
		rows = append(rows, lipgloss.NewStyle().Width(innerW).Render(""))
	}
	rows = append(rows, lipgloss.NewStyle().Width(innerW).Render("a add  i ignore  esc x"))
	body := strings.Join(rows, "\n")
	frame := st.Popover
	if frame.GetBorderStyle() == (lipgloss.Border{}) {
		frame = frame.Border(lipgloss.RoundedBorder())
	}
	return frame.Render(body)
}

// position returns the (row, col) where the popover's top-left
// corner should sit, given the cursor row/col in screen
// coordinates and the viewport dimensions.
func (p popoverState) position(cursorRow, cursorCol, viewportW, viewportH int) (int, int) {
	row := cursorRow + 1
	if row+p.height() > viewportH {
		row = cursorRow - p.height()
		if row < 0 {
			row = 0
		}
	}
	col := cursorCol
	if col+p.width() > viewportW {
		col = viewportW - p.width()
		if col < 0 {
			col = 0
		}
	}
	return row, col
}

// overlay composites a popover onto rendered, returning a new
// string with the popover's rows replacing the corresponding
// horizontal slices of the body. ANSI-aware via ansiSpliceAtCol.
func overlay(body, pop string, row, col int) string {
	bodyLines := strings.Split(body, "\n")
	popLines := strings.Split(pop, "\n")
	for i, pl := range popLines {
		idx := row + i
		if idx < 0 || idx >= len(bodyLines) {
			continue
		}
		bodyLines[idx] = ansiSpliceAtCol(bodyLines[idx], col, lipgloss.Width(pl), pl)
	}
	return strings.Join(bodyLines, "\n")
}
```

- [ ] **Step 4: Compose the popover into Model.View**

Edit `catkin.go`'s `View`:

```go
func (m Model) View() string {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	body := RenderAnnotated(src, m.width, m.height-m.find.footerRows(), m.viewportTop, cur, m.styles, m.mode, m.annotations)
	if m.popover.open {
		row, col := m.popoverScreenPosition()
		body = overlay(body, m.popover.render(0, m.styles), row, col)
	}
	if !m.find.active() {
		return body
	}
	return body + "\n" + m.find.renderFindFooter(m.width)
}

// popoverScreenPosition translates the misspelled word's anchor
// into screen coordinates within the rendered viewport.
func (m Model) popoverScreenPosition() (int, int) {
	src := m.buf.Value()
	startRow, startCol := offsetToRowCol(src, byteOffsetToRune(src, m.popover.wordRange.Start))
	screenRow := startRow - m.viewportTop
	if screenRow < 0 {
		screenRow = 0
	}
	return m.popover.position(screenRow, startCol, m.width, m.height-m.find.footerRows())
}

// byteOffsetToRune is the inverse of byteOffsetForRune.
func byteOffsetToRune(src string, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	count := 0
	for i := range src {
		if i >= byteOff {
			return count
		}
		count++
	}
	return count
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/catkin/ -run "TestPopover" -v`
Expected: PASS for all popover tests.

Run: `go test ./internal/catkin/ -v`
Expected: full package green.

- [ ] **Step 6: Commit**

```bash
git add internal/catkin/popover.go internal/catkin/popover_test.go internal/catkin/catkin.go
git commit -m "Pass 9d: popover render + ANSI-aware overlay"
```

---

## Task 12: Cursor-leave auto-close

**Files:**

- Modify: `internal/catkin/catkin.go`
- Test: `internal/catkin/popover_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestPopoverClosesOnCursorLeave(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlSemicolon})
	if !m.popover.open {
		t.Fatalf("setup: popover should be open")
	}
	// Move cursor outside the misspelling range with arrow-right
	// repeated past the word's end.
	for i := 0; i < 20; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if !m.popover.open {
			break
		}
	}
	if m.popover.open {
		t.Errorf("popover should auto-close once cursor leaves the misspelling range")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/catkin/ -run TestPopoverClosesOnCursorLeave -v`
Expected: FAIL.

- [ ] **Step 3: Hook the check into afterEdit / cursor-moving Updates**

Edit `catkin.go`. Add the helper:

```go
// closePopoverIfCursorLeftRange clears the popover when the cursor
// is no longer inside its anchor range.
func (m Model) closePopoverIfCursorLeftRange() Model {
	if !m.popover.open {
		return m
	}
	off := byteOffsetForRune(m.buf.Value(), m.buf.RuneOffset())
	if !m.popover.wordRange.Contains(off) {
		return m.closePopover()
	}
	return m
}
```

Call it at the bottom of `afterEdit` (covers edits) and right after dispatching cursor-only key handling. The simplest spot: wrap the final `applyScrollOff(m)` call:

```go
func (m Model) afterEdit(b Buffer, cmd tea.Cmd) (Model, tea.Cmd) {
	prev := m.buf.Value()
	m.buf = b
	m.recordSnap()
	if m.buf.Value() != prev && len(m.annotators) > 0 {
		m.srcGen++
		cmd = tea.Batch(cmd, scheduleAnnotateCmd(m.srcGen))
	}
	m = m.closePopoverIfCursorLeftRange()
	return applyScrollOff(m), cmd
}
```

For pure cursor-movement keys that don't go through `afterEdit` (the bubbles textarea handles arrow-left/right inside `m.buf.Update`), confirm the path: in the existing `Update` body, after the buffer update, the result already flows through `afterEdit`. The check therefore covers that path.

- [ ] **Step 4: Run test**

Run: `go test ./internal/catkin/ -run TestPopoverClosesOnCursorLeave -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/catkin.go internal/catkin/popover_test.go
git commit -m "Pass 9d: popover auto-closes on cursor leaving range"
```

---

## Task 13: Live tmux verification

**Files:**

- None (capture-only). Captures land under `docs/superpowers/captures/2026-05-05-catkin-annotations/` per project convention.

- [ ] **Step 1: Build and install**

```bash
make install
```

Expected: `~/.local/bin/poplar` updated, no compile errors.

- [ ] **Step 2: Author a one-shot Catkin demo harness**

If a Catkin standalone demo binary exists under `cmd/`, use it. If not, the simplest path is a small ad-hoc test program (do not commit) at `/tmp/catkin-demo/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/catkin"
)

func main() {
	speller, err := catkin.NewSpeller(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m := catkin.New()
	m.SetSize(80, 24)
	m.RegisterAnnotator(catkin.NewSpellcheckAnnotator(speller))
	m.SetValue("Markdown spellcheck demo.\n\nThe tradeof is real and the perfomance is fine.\n\n```go\nfunc tradeof() {}\n```\n")
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

```bash
mkdir -p /tmp/catkin-demo
cd /tmp/catkin-demo
go mod init demo && go mod edit -replace github.com/glw907/poplar=/home/glw907/Projects/poplar && go mod tidy
go build -o /tmp/catkin-demo/demo .
```

- [ ] **Step 3: Capture squiggle render at 80×24**

Per `.claude/docs/tmux-testing.md`:

```bash
tmux new-session -d -s catkin -x 80 -y 24 "/tmp/catkin-demo/demo"
sleep 1.5
tmux capture-pane -t catkin -p > docs/superpowers/captures/2026-05-05-catkin-annotations/01-squiggle-80x24.txt
tmux kill-session -t catkin
```

Verify `tradeof` and `perfomance` show the squiggle ANSI sequence (underline) and that words inside the fenced block are unmarked.

- [ ] **Step 4: Capture popover (below cursor) at 120×40**

```bash
tmux new-session -d -s catkin -x 120 -y 40 "/tmp/catkin-demo/demo"
sleep 1.5
# Move cursor onto "tradeof" — depends on default cursor; send right-arrows.
tmux send-keys -t catkin "Right Right Right Right Right Right Right Right" "" "C-;"
sleep 0.5
tmux capture-pane -t catkin -p > docs/superpowers/captures/2026-05-05-catkin-annotations/02-popover-below-120x40.txt
tmux kill-session -t catkin
```

Verify the popover renders below the cursor with the misspelled word in the header and at least one suggestion.

- [ ] **Step 5: Capture popover (above cursor) at 80×24**

Author a second demo body where the misspelling sits near the bottom of the view:

```go
m.SetValue("\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nThe tradeof is real.\n")
```

Re-run the capture. Verify the popover sits above the cursor row.

- [ ] **Step 6: Capture 0-suggestion popover**

Body: `"qzxwfvbk"` (random keystroke noise — SymSpell returns no candidates within edit distance 2).

```bash
tmux new-session -d -s catkin -x 80 -y 24 "/tmp/catkin-demo/demo"
sleep 1.5
tmux send-keys -t catkin "C-;"
sleep 0.5
tmux capture-pane -t catkin -p > docs/superpowers/captures/2026-05-05-catkin-annotations/03-popover-zero-80x24.txt
tmux kill-session -t catkin
```

Verify only header + actions row are shown (no separator, no suggestion list).

- [ ] **Step 7: Verify add-to-wordlist round trip**

```bash
WL=/tmp/poplar-wl-test.txt
rm -f "$WL"
# Adjust demo to call m.SetUserWordlistPath(os.Getenv("CATKIN_WL")).
CATKIN_WL=$WL tmux new-session -d -s catkin -x 80 -y 24 "/tmp/catkin-demo/demo"
sleep 1.5
tmux send-keys -t catkin "C-;" "a"
sleep 0.5
tmux capture-pane -t catkin -p > docs/superpowers/captures/2026-05-05-catkin-annotations/04-add-roundtrip-80x24.txt
tmux kill-session -t catkin
cat "$WL"
```

Verify the captured file shows the squiggle gone after `a`, and `$WL` contains the word.

- [ ] **Step 8: Commit captures**

```bash
git add docs/superpowers/captures/2026-05-05-catkin-annotations/
git commit -m "Pass 9d: tmux captures (squiggle, popover above/below/zero, add round-trip)"
```

---

## Task 14: Pass-end consolidation

This step invokes the `poplar-pass` skill end-to-end ritual. Each step here is a checklist item the skill expects.

- [ ] **Step 1: Run /simplify against the diff**

Invoke the `simplify` skill on the Pass 9d diff. Apply genuine wins. Commit.

- [ ] **Step 2: Run the bubbletea conformance §1b checklist**

Open `docs/poplar/bubbletea-conventions.md` §10. Walk every item against the diff and the captures from Task 13. Note any deviation in the ADRs in step 4.

- [ ] **Step 3: Run make check**

```bash
make check
```

Expected: PASS (vet, voice-check, full test suite).

- [ ] **Step 4: Write ADR-0149 and ADR-0150**

```
docs/poplar/decisions/0149-catkin-annotation-pipeline.md
docs/poplar/decisions/0150-catkin-spellcheck.md
```

Each follows the template in `poplar-pass`. Cover, in 0149: range-based annotations, registry-ordered composition, debounced-tick + generation-counter idle, why pure (no goroutines), composition rule. In 0150: SymSpell with deletion-distance index, embedded en_US + project + user overlay, code/inline/URL skip masks, all-caps ≤4 heuristic, popover UX (Ctrl+;, 5-cap, dynamic height, add/ignore actions, cursor-leave auto-close).

- [ ] **Step 5: Update invariants.md**

Edit the Catkin section of `docs/poplar/invariants.md`. Add the annotation + spellcheck binding facts; replace any existing Catkin claim that's narrowed by 9d. Add ADR-0149 and ADR-0150 to the decision-index table.

- [ ] **Step 6: Update STATUS.md**

- Mark Pass 9d `done`.
- Replace the starter prompt with the Pass 9e starter (compose `internal/compose/`, Editor interface, CatkinEditor adapter, Draft, AssembleMIME, Seed{Reply,ReplyAll,Forward}). Pull from existing scaffolding in STATUS or earlier plan notes.

- [ ] **Step 7: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-05-catkin-annotations.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-05-catkin-annotations-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 8: Commit, push, install**

```bash
git add -A
git commit -m "Pass 9d: Catkin annotation pipeline + spellcheck (ADR-0149, ADR-0150)"
git push
make install
```

---

## Self-review notes

- **Spec coverage.** Each spec section maps to a task: primitive types (T1), registry + idle (T2), wordlists (T3), Speller.Check + extras (T4), Speller.Suggest (T5), LoadUserWordlist (T6), spellcheckAnnotator with skip masks (T7), squiggle render (T8), popover state machine + keys (T9), AddToWordlist persistence (T10), popover render + positioning (T11), cursor-leave auto-close (T12), live verification (T13), pass-end ritual (T14).
- **Type consistency.** `Range`, `Annotation`, `MisspellingPayload`, `Annotator`, `AnnotationSet`, `Speller`, `popoverState`, `spellcheckAnnotator` are introduced in T1/T2/T4/T7/T9 and used consistently downstream. Method names: `RegisterAnnotator`, `LoadUserWordlist`, `NewSpeller`, `Speller.Check`, `Speller.Suggest`, `NewSpellcheckAnnotator`, `IgnoreInSession`, `SetUserWordlistPath`. Msg types: `annotateRequestMsg`, `annotationsReadyMsg`. All match across tasks.
- **Open implementation note.** T7's `buildSkipMask` does substring search inside lines to recover absolute offsets for inline-code and link spans. If `walkSpans` proves too lossy in practice, the call-out in T7 directs the implementer to expose absolute offsets from `style.go` rather than papering over it here.
- **Risk: build script.** T3's `scripts/build-wordlist.sh` reaches the network. The committed `en_US.txt` is the source of truth; the script exists for reproducibility. Implementers can skip the script if `internal/catkin/spellcheck/en_US.txt` already exists.
