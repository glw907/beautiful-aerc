# Go Comment Style: Essays, Talks, and Proverbs

Primary sources for the poplar comment style guide. Faithful extraction
with verbatim quotes; no synthesis. Synthesis lives in the next pass.

---

## Rob Pike — Go Proverbs (2015 Gopherfest SV talk)

Source: https://go-proverbs.github.io/

### Stated purpose of comments

Not addressed directly in the proverbs themselves. The proverbs function as
a distilled philosophy of Go design; several bear directly on what makes
a comment earn its place.

### Direct quotes (verbatim)

The complete proverb list, verbatim:

> "Don't communicate by sharing memory, share memory by communicating."

> "Concurrency is not parallelism."

> "Channels orchestrate; mutexes serialize."

> "The bigger the interface, the weaker the abstraction."

> "Make the zero value useful."

> "interface{} says nothing."

> "Gofmt's style is no one's favorite, yet gofmt is everyone's favorite."

> "A little copying is better than a little dependency."

> "Syscall must always be guarded with build tags."

> "Cgo must always be guarded with build tags."

> "Cgo is not Go."

> "With the unsafe package there are no guarantees."

> "Clear is better than clever."

> "Reflection is never clear."

> "Errors are values."

> "Don't just check errors, handle them gracefully."

> "Design the architecture, name the components, document the details."

> "Documentation is for users."

> "Don't panic."

### Named anti-patterns

- Cleverness over clarity ("Clear is better than clever" implies the inverse
  is the anti-pattern: clever code that requires explanation to be understood).
- Empty interfaces as documentation ("interface{} says nothing" — a type that
  can't be read as documentation of its purpose).
- Weak abstractions signaled by fat interfaces.

### Author's own voice

The proverbs are aphoristic: short imperative or declarative sentences, zero
hedging, no qualification. They read like commands carved into stone. Length:
4–12 words per proverb. No conjunctions softening the stance. The authority
comes from compression — every word load-bearing, none decorative.

---

## Rob Pike — "Notes on Programming in C" (Pike Style, 1989)

Source: https://www.lysator.liu.se/c/pikestyle.html

### Stated purpose of comments

Pike's model: comments are introductions to what follows, not translations of
what's already legible from the code. He writes explicitly that self-documenting
code is the goal and comments are a fallback for the cases where code cannot
carry its own explanation:

> "If the code is clear, and uses good type names and variable names, it should
> explain itself."

Comments earn a place in three specific roles only:
- Explaining the use of global variables and types ("the one thing I always
  comment in large programs").
- As an introduction to an unusual or critical procedure.
- To mark off sections of a large computation.

### Direct quotes (verbatim)

> "I tend to err on the side of eliminating comments, for several reasons."

> "If the code is clear, and uses good type names and variable names, it should
> explain itself."

> "Comments aren't checked by the compiler, so there is no guarantee they're
> right, especially after the code is modified."

> "The issue of typography: comments clutter code."

> "Almost exclusively, I use them as an introduction to what follows. Examples:
> explaining the use of global variables and types (the one thing I always
> comment in large programs); as an introduction to an unusual or critical
> procedure; or to mark off sections of a large computation."

> "If your code needs a comment to be understood, it would be better to rewrite
> it so it's easier to understand."

> "Avoid cute typography in comments, avoid big blocks of comments except
> perhaps before vital sections."

### Named anti-patterns

**The obvious restatement** (Pike's own example):

```c
i=i+1;           /* Add one to i */
```

**Excessive decoration**:

```c
/************************************
 *                                  *
 *          Add one to i            *
 *                                  *
 ************************************/
```

Both are named explicitly as failures of judgment. The first is redundancy;
the second is redundancy dressed up as ceremony.

### Author's own voice

Flat, assertive, pedagogical. Sentences are short and complete. He uses
first-person sparingly ("I tend to", "I use them") and only to establish
a practice, not to qualify a claim. No hedging words ("perhaps", "maybe",
"could"). The typography section is almost impatient — he states the problem
and moves on without elaboration. The essay reads as if written in one draft
and not revised for softness.

---

## Andrew Gerrand — "Godoc: Documenting Go Code" (Go Blog, 2011)

Source: https://go.dev/blog/godoc

### Stated purpose of comments

Godoc's purpose is to couple documentation to code so they evolve together.
The comment's purpose is machine-readability combined with human readability:
the same text must serve both `go doc` output and a human reader browsing
source.

> "The Go project takes documentation seriously. Documentation is a huge part
> of making software accessible and maintainable. Of course it must be well-
> written and accurate, but it also must be easy to write and to maintain.
> Ideally, it should be coupled to the code itself so the documentation evolves
> along with the code."

> "Godoc comments are just good comments, the sort you would want to read even
> if godoc didn't exist."

### Direct quotes (verbatim)

> "to document a type, variable, constant, function, or even a package, write
> a regular comment directly preceding its declaration, with no intervening
> blank line."

> "Notice this comment is a complete sentence that begins with the name of the
> element it describes. This important convention allows us to generate
> documentation in a variety of formats, from plain text to HTML to UNIX man
> pages, and makes it read better when tools truncate it for brevity, such as
> when they extract the first line or sentence."

> "Godoc comments are just good comments, the sort you would want to read even
> if godoc didn't exist."

> "The best thing about godoc's minimal approach is how easy it is to use."

Stdlib example used as illustration:

```go
// Fprint formats using the default formats for its operands and writes to w.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func Fprint(w io.Writer, a ...interface{}) (n int, err error) {
```

### Named anti-patterns

- Doc comments with an intervening blank line between comment and declaration
  (godoc won't associate them).
- Comments that don't begin with the name of the element described (breaks
  machine extraction and truncated-text reading).
- Incomplete sentences (breaks format portability).

### Author's own voice

Gerrand writes in a measured, approachable technical style. Third-person
declarative for rules ("The convention is simple"), first-person plural
("we generate documentation") when speaking for the project. Sentences run
longer than Pike's but stay on one idea per sentence. The tone is encouraging
rather than prescriptive — he frames godoc as minimal and easy rather than
demanding.

---

## Go Doc Comments (official reference, go.dev/doc/comment)

Source: https://go.dev/doc/comment

This document supersedes and expands the 2011 Gerrand post. It is the current
authoritative reference.

### Stated purpose of comments

> "Doc comments focus on what the operation returns or does, detailing what the
> caller needs to know."

> "Doc comments should not explain internal details such as the algorithm used
> in the current implementation. Those are best left to comments inside the
> function body."

### Direct quotes (verbatim)

> "Every exported (capitalized) name should have a doc comment."

> "An explicit subject often makes the wording clearer, and it makes the text
> easier to search, whether on a web page or a command line."

> "For a package comment, that means the first sentence begins with 'Package'."

> "By default, programmers should expect that a type is safe for use only by a
> single goroutine at a time."

Function comment examples from stdlib, verbatim:

```go
// HasPrefix reports whether the string s begins with prefix.
func HasPrefix(s, prefix string) bool
```

```go
// Copy copies from src to dst until either EOF is reached
// on src or an error occurs. It returns the total number of bytes
// written and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
func Copy(dst Writer, src Reader) (n int64, err error) {
```

```go
// Quote returns a double-quoted Go string literal representing s.
// The returned string uses Go escape sequences (\t, \n, \xFF, Ā)
// for control characters and non-printable characters as defined by IsPrint.
func Quote(s string) string {
```

Deprecation pattern:

> "To signal that an identifier should not be used, add a paragraph to its doc
> comment that begins with 'Deprecated:' followed by some information about
> the deprecation."

### Named anti-patterns

Documented under "Common mistakes and pitfalls":

1. Unindented numbered lists — the last line becomes a code block in godoc.
2. Indenting a wrapped unindented line of text: "The most common mistake is
   indenting a wrapped unindented line of text."
3. Nested lists — Go doc comments do not support them.
4. Brace-delimited code blocks without consistent indentation.

### Author's own voice

The reference doc uses the imperative ("Doc comments focus on...") and the
third-person declarative ("Every exported name should have..."). Formal,
prescriptive. No first-person, no hedging. Each rule stated once and not
repeated. The examples do the work of justification; the prose does not argue.

---

## Dave Cheney — Practical Go (QCon China presentation)

Source: https://dave.cheney.net/practical-go/presentations/qcon-china.html

### Stated purpose of comments

Cheney identifies three things a comment can do: explain what something does,
explain how it works, or explain why it exists. He argues the "why" is the
rarest and most valuable — the other two can often be read from the code.

He cites Thomas and Hunt as a framing device:
> "Good code has lots of comments, bad code requires lots of comments."

For public API documentation specifically:
> "Always document public symbols."

But immediately qualifies: documenting an interface implementation with
"implements the io.Reader interface" adds no value and should be omitted.

### Direct quotes (verbatim)

> "Comments on variables and constants should describe their contents not their
> purpose."

On the anti-pattern of describing where something is used rather than what it
contains:
> "Don't name your variables for their types."

On the relationship between comments and code quality:
> "Don't comment bad code — rewrite it."

On naming:
> "Choose identifiers for clarity, not brevity."

> "Obvious code is important. What you can do in one line you should do in
> three."

Citing Andrew Gerrand's principle (via Cheney):
> "The greater the distance between a name's declaration and its uses, the
> longer the name should be."

On error handling:
> "Only handle an error once."

> "Never inspect the output of error.Error."

The `Error()` method "exists for humans (logs/display), not for program logic.
Comparing string representations is a code smell."

### Named anti-patterns

- Documenting interface implementations with generic phrases ("implements
  io.Reader") — no information content.
- Naming variables for their types (`configMap` when the declaration already
  says it's a map).
- Logging an error and returning it — "Java anyone?" — violates the single-
  decision rule and produces duplicate log entries.
- Inspecting `error.Error()` string output in program logic.
- Sentinel errors as public API (creates coupling and rigidity).

### Author's own voice

Cheney writes conversationally but precisely. He uses "you" throughout,
addresses the reader directly, and builds arguments with concrete before-and-
after code examples. He cites other authorities (Dijkstra, Reinhold, Cox,
Gerrand, Thomas/Hunt) to ground his advice in consensus rather than personal
preference. The tone is collegial — he frames bad patterns as easy mistakes
rather than failures of intelligence. Sentences are medium length, often
paired: problem sentence, then solution sentence.

---

## Dave Cheney — "Don't just check errors, handle them gracefully" (2016)

Source: https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully

### Stated purpose of comments

Not directly addressed in this post. The implicit argument is that good error
handling *is* a form of documentation — an error message is the comment the
runtime delivers to whoever is debugging.

### Direct quotes (verbatim)

> "Errors are just values."

> "Never inspect the output of error.Error."

The `Error()` method "exists for humans (logs/display), not for program logic."

Comparing error strings is:
> "a code smell."

On the single-decision rule:
> "You should only handle errors once."

On the failure mode of logging-and-returning:
> "Java anyone?"

On opaque errors (the recommended strategy):
Treat errors as black boxes — inspect behavior, not type or string content.

On wrapping:
> "Add context to errors."

### Named anti-patterns

- Sentinel errors: creates API coupling, "creates rigidity."
- Error type assertions: requires public error types, brittle coupling across
  packages.
- Multiple handling: logging at multiple stack levels produces duplicate output
  while the top-level caller loses context.
- String-matching on `error.Error()` output: a "code smell."

### Author's own voice

Same conversational-direct register as the Practical Go talk but compressed
into a blog post structure. Pithy labels ("opaque errors", "sentinel errors",
"error types") organize the argument. The "Java anyone?" aside is the most
opinionated moment — Cheney allows himself one barb per essay, aimed at an
anti-pattern by naming its origin. Otherwise measured and grounded in examples.

---

## Effective Go (Go project, 2009; not actively updated)

Source: https://go.dev/doc/effective_go

### Stated purpose of comments

Comments preceding top-level declarations are "doc comments" — the primary
documentation for a given Go package or command. The mechanical purpose is
godoc extraction; the human purpose is understanding without reading the
implementation.

> "Comments that appear before top-level declarations, with no intervening
> newlines, are considered to document the declaration itself."

### Direct quotes (verbatim)

On package names:
> "By convention, packages are given lower case, single-word names; there
> should be no need for underscores or mixedCaps. Err on the side of brevity,
> since everyone using your package will be typing that name."

On avoiding repetition in exported names:
> "The importer of a package will use the name to refer to its contents, so
> exported names in the package can use that fact to avoid repetition... the
> buffered reader type in the bufio package is called Reader, not BufReader,
> because users see it as bufio.Reader, which is a clear, concise name."

On getters:
> "Go doesn't provide automatic support for getters and setters... it's
> neither idiomatic nor necessary to put Get into the getter's name. If you
> have a field called owner (lower case, unexported), the getter method should
> be called Owner (upper case, exported), not GetOwner."

On error strings:
> "When feasible, error strings should identify their origin, such as by having
> a prefix naming the operation or package that generated the error. For
> example, in package image, the string representation for a decoding error
> due to an unknown format is 'image: unknown format'."

On os.PathError as a model for rich errors:
> "Such an error, which includes the problematic file name, the operation, and
> the operating system error it triggered, is useful even if printed far from
> the call that caused it; it is much more informative than the plain 'no such
> file or directory'."

On interface names:
> "By convention, one-method interfaces are named by the method name plus an
> -er suffix or similar modification to construct an agent noun: Reader,
> Writer, Formatter, CloseNotifier etc."

On control flow style:
> "In the Go libraries, you'll find that when an if statement doesn't flow
> into the next statement—that is, the body ends in break, continue, goto, or
> return—the unnecessary else is omitted."

### Named anti-patterns

- `GetOwner` as a getter name (the `Get` prefix is non-idiomatic).
- Package-doubled names: `bufio.BufReader` instead of `bufio.Reader`.
- Underscores or mixedCaps in package names.
- Capitalized error strings or strings ending in punctuation (error strings
  are combined with other context by callers).

### Author's own voice

Effective Go is written in plural first-person ("In the Go libraries, you'll
find...") and second-person ("Err on the side of brevity"). It is
authoritative but not rigid — it uses "when feasible", "it's often
appropriate", suggesting awareness that rules have exceptions. The document
reads as a guide written by the language's creators for their own community,
combining prescription with explanation of the reasoning.

---

## Go Code Review Comments (Go Wiki)

Source: https://go.dev/wiki/CodeReviewComments

### Stated purpose of comments

Comments documenting declarations should serve godoc. All top-level exported
names must have doc comments. Non-trivial unexported types or functions should
also have comments. The test is extractability and readability at the call
site, not at the definition.

### Direct quotes (verbatim)

On comment form:
> "Comments documenting declarations should be full sentences, even if redundant.
> Comments should begin with the name of the thing being described and end in a
> period."

On error strings — the most-cited rule in this document:
> "Error strings should not be capitalized (unless beginning with proper nouns
> or acronyms) or end with punctuation, since they are usually printed following
> other context."

Example given:
```go
// CORRECT
fmt.Errorf("something bad")

// INCORRECT
fmt.Errorf("Something bad")

// In context
log.Printf("Reading %s: %v", filename, err)
// Output: "Reading file.txt: something bad"
```

> "This does not apply to logging, which is implicitly line-oriented and not
> combined inside other messages."

On variable names:
> "Variable names in Go should be short rather than long. This is especially
> true for local variables with limited scope. Prefer c to lineCount. Prefer i
> to sliceIndex."

> "The basic rule: the further from its declaration that a name is used, the
> more descriptive the name must be."

On acronyms/initialisms:
> "Words that are initialisms have consistent case: 'URL' or 'url', never 'Url'."

On package names:
> "All references to names in your package will be done using the package name,
> so you can omit that name from the identifiers."
>
> "Avoid meaningless package names: util, common, misc, api, types, interfaces."

On named result parameters:
```go
// Less clear in godoc
func (n *Node) Parent1() (node *Node) {}

// Better - clearer in godoc
func (n *Node) Parent1() *Node {}
```

Exception where names add clarity:
```go
// Location returns f's latitude and longitude.
// Negative values mean south and west, respectively.
func (f *Foo) Location() (lat, long float64, err error)
```

On in-band errors:
> "Don't use sentinel values (-1, null, '') to signal errors."

On panic:
> "Don't use panic for normal error handling. Use error and multiple return
> values."

### Named anti-patterns

- Capitalized error strings.
- Error strings ending in punctuation.
- `util`, `common`, `misc`, `api`, `types`, `interfaces` as package names.
- Redundant package-name prefix in identifiers (e.g., `chubby.ChubbyFile`).
- Named return parameters when they add no godoc clarity.
- In-band sentinel values for errors (C-style `-1` / `null` / `""`).

### Author's own voice

The wiki uses terse declarative rules, each illustrated with a code block.
No hedging, no first-person. Rules are stated once; rationale is one sentence
if it appears at all. The closest analogue to a style manual — terse, item-
by-item, expecting the reader to apply judgment about when the rule fires.

---

## Russ Cox — "A Name's Length Should Not Exceed Its Information Content" (2010)

Source: https://research.swtch.com/names

### Stated purpose of comments

Not addressed in this post; the focus is naming. But the principle transfers:
a name (or a comment) should contain no more words than its information
requires. Padding is a failure.

### Direct quotes (verbatim)

> "A name's length should not exceed its information content."

> "Make every name tell."

On the failure mode of over-long names, using `getParametersAsNamedValuePairArray`
as the example: the function signature already conveys that parameters are
returned as an array; the name adds noise. Shorter alternatives like `params()`
or `queryParams()` communicate more efficiently.

### Named anti-patterns

- Names longer than their information content.
- Names that restate what the type signature already says.
- Redundant words in any name (see `getParametersAsNamedValuePairArray`).

### Author's own voice

Cox writes with clarity and pragmatism, using concrete examples from real
codebases. He grounds his philosophy in measurable criteria (information
density) rather than aesthetics. The tone is collaborative rather than
prescriptive — he frames choices as engineering tradeoffs, acknowledges
context-dependence (scope determines needed specificity), and does not issue
rules without examples. Measured, accessible, technically grounded.

---

## Rob Pike — "Errors are values" (Go Blog, 2015)

Source: https://go.dev/blog/errors-are-values

### Stated purpose of comments

Not the focus of this post, but Pike models a style: the post itself is a
working example of how Pike explains philosophy through code examples rather
than through prose assertion.

### Direct quotes (verbatim)

> "Errors are values."

> "the obvious target is Go itself" — but this misses "a fundamental point
> about errors: Errors are values."

> "the client's code therefore feels more natural: loop until done, then worry
> about errors. Error handling does not obscure the flow of control."

Despite promoting error abstraction, Pike ends with:
> "But remember: Whatever you do, always check your errors!"

### Named anti-patterns

- Treating error handling as necessarily repetitive ("if err != nil everywhere
  is a code smell" is the perception Pike argues against).
- Stopping at the mechanical observation (nil check) without asking how to
  program the error value itself.

### Author's own voice

Pragmatic and conversational. Pike uses "Sure, there is a nil check..."
acknowledging the surface perception before dismantling it. He is
self-correcting — he credits the concern before arguing against it. No
hedging words, but a collegial register. The final sentence ("always check
your errors!") is the only exclamation point in the post, used deliberately
to underscore that the philosophical argument doesn't license sloppiness.

---

## Cross-cutting principles

- **Comments document WHY, never WHAT.** Universal across Pike (pikestyle),
  Cheney (Practical Go), Effective Go, and Code Review Comments. Code that
  needs a comment to explain *what* it does is a candidate for rewriting.
  The exception is doc comments on exported symbols, which document *what
  the caller needs to know* — but even there, the implementation rationale
  is explicitly out of scope (go.dev/doc/comment).

- **Comments earn their place or they don't exist.** Pike: "I tend to err
  on the side of eliminating comments." The burden of proof falls on the
  comment, not on its absence. Silence is the default; a comment is a
  decision.

- **Self-documenting code is the primary tool; comments are fallback.**
  Pike (pikestyle), Cheney ("Don't comment bad code — rewrite it"), and
  Code Review Comments all converge: a comment that explains bad code
  should instead motivate a rewrite.

- **Exported symbols require doc comments; unexported symbols default to
  none.** Code Review Comments: "All top-level, exported names should have
  doc comments." Cheney: documenting unexported internals with obvious
  observations is noise. The one exception: non-trivial unexported types or
  functions may earn a comment.

- **Doc comments describe what the caller needs, not how it's implemented.**
  go.dev/doc/comment is explicit: implementation details belong in comments
  inside the function body, not in the doc comment above the signature.

- **Error strings are lowercase, no trailing punctuation.** Code Review
  Comments states this as a hard rule. Effective Go explains the reason:
  error strings are composed with other context by callers (`log.Printf`
  etc.), so capitalization and punctuation break the composition.

- **Names should not exceed their information content.** Cox (swtch.com/names),
  Effective Go (avoid package-doubled names), Code Review Comments (short
  local names). The type signature, the package context, and the call site
  all supply information that the name need not repeat.

- **Consistency within a scope matters more than absolute name length.**
  Code Review Comments: "The further from its declaration, the more descriptive
  the name must be." A one-letter loop variable is correct for a 5-line loop;
  the same letter is wrong for a variable used 100 lines later.

- **Error handling is a call site decision, not a library decision.**
  Cheney: "Only handle an error once." Handling means *one* action — return,
  log, or wrap — not a combination. Code that logs and returns forces the
  caller to see duplicate output and loses the clean error chain.

- **Comments that become stale are worse than no comments.** Pike explicitly:
  "Comments aren't checked by the compiler, so there is no guarantee they're
  right, especially after the code is modified." This is the mechanical
  argument for preferring code clarity over comment coverage.

- **One genuine disagreement: godoc verbosity for unexported symbols.**
  Cheney (Practical Go) implies that even exported symbols don't need godoc
  if the comment adds no information (e.g., "implements io.Reader"). Code
  Review Comments says non-trivial unexported types or functions "should also
  have doc comments." Pike (pikestyle, 1989) says global variables always
  get a comment. The emerging consensus: the test is information content,
  not symbol visibility — but the Go project's official guidance leans toward
  more coverage rather than less for exported names.

---

## Voice palette

**Pike-aphoristic.** 4–12 word imperative or declarative sentences. No
qualification, no hedging, no examples unless the example *is* the point.
Commands delivered as obvious truths. Compression is the signal of
confidence. ("Clear is better than clever." "Documentation is for users.")

**Cheney-conversational.** Addresses the reader as "you." Builds arguments
with concrete before/after code examples. Cites other authorities to establish
consensus rather than assertion. Allows one opinionated aside per essay
("Java anyone?"). Medium-length sentences paired: problem, then solution.
Collegial register; treats bad patterns as easy mistakes, not failures.

**stdlib-formal.** Third-person declarative, 1–3 sentence average per rule.
No first-person. Rationale in one sentence if it appears at all; the example
carries the rest. Used by Effective Go, Code Review Comments, and go.dev/doc/
comment. Authoritative without being personal.

**Cox-measured.** Grounds abstractions in concrete examples from real
codebases. Frames choices as engineering tradeoffs with costs and benefits on
both sides. Collaborative rather than prescriptive. Longer paragraphs than
Pike, shorter than Cheney. Acknowledges context-dependence before stating the
principle. ("A name's length should not exceed its information content" is
stated after the example, not before.)

**Gerrand-welcoming.** Similar to stdlib-formal but warmer. Uses first-person
plural ("we generate documentation") when speaking for the project. Frames
rules as minimal and easy rather than demanding. Encourages rather than
prescribes. The tone of a senior developer onboarding a capable newcomer.
