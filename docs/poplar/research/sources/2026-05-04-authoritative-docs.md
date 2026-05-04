# Authoritative Go Style Sources — 2026-05-04

Extraction pass for the comment/voice style guide synthesis. One H2
per source. Sub-sections: On comments, On godoc, On error phrasing,
On naming, Voice quotes. Final H2 collects cross-source agreements
and tensions.

---

## Effective Go

URL: https://go.dev/doc/effective_go  
Relevant sections: Commentary, Names (Getters, Interface names,
MixedCaps, Package names), Errors.

### On comments

Doc comments are the primary documentation mechanism. They must
immediately precede the top-level declaration with no intervening
blank line. The document does not articulate a "write only when
non-obvious" rule but its examples consistently model brief,
functional prose — the commentary section shows short, declarative
one-liners for exported types and functions.

Block comments (`/* */`) are reserved for package comments and
occasional large code disabling. Line comments (`//`) are the norm
everywhere else.

The document does not say when *not* to comment, but the density of
commented examples follows a clear pattern: exported symbols get a
sentence; internal helpers appear without comment. The "Commentary"
section itself is brief, suggesting the authors consider excessive
meta-commentary a sign of poor design.

### On godoc

- Every exported name should have a doc comment.
- "Comments that appear before top-level declarations, with no
  intervening newlines, are considered to document the declaration
  itself. These 'doc comments' are the primary documentation for a
  given Go package or command." (§ Commentary)
- Package names are single lowercase words; the package name appears
  in the import path's base — `src/encoding/base64` yields package
  name `base64`, not `encodingBase64`.
- The `bufio.Reader` example demonstrates the package-as-prefix
  pattern: "exported names in the package can use that fact to avoid
  repetition." (§ Names — Package names)

### On error phrasing

From § Errors:

> "When feasible, error strings should identify their origin, such as
> by having a prefix naming the operation or package that generated
> the error. For example, in package `image`, the string
> representation for a decoding error due to an unknown format is
> 'image: unknown format'."

The `PathError` example structures error text as
`op + " " + path + ": " + err.Error()`. The rationale is explicit:
"Such an error, which includes the problematic file name, the
operation, and the operating system error it triggered, is useful
even if printed far from the call that caused it; it is much more
informative than the plain 'no such file or directory'."

No explicit prohibition on capitalization or trailing punctuation
appears here (that rule is in CodeReviewComments).

### On naming

**Getters (§ Names — Getters):**

> "Go doesn't provide automatic support for getters and setters... it's
> neither idiomatic nor necessary to put `Get` into the getter's name.
> If you have a field called `owner` (lower case, unexported), the
> getter method should be called `Owner` (upper case, exported), not
> `GetOwner`. The use of upper-case names for export provides the hook
> to discriminate the field from the method."

**Interface names (§ Names — Interface names):**

> "By convention, one-method interfaces are named by the method name
> plus an -er suffix or similar modification to construct an agent
> noun: `Reader`, `Writer`, `Formatter`, `CloseNotifier` etc."

**Package-doubled types (§ Names — Package names):**

> "The function to make new instances of `ring.Ring`... would normally
> be called `NewRing`, but since `Ring` is the only type exported by
> the package, and since the package is called `ring`, it's called just
> `New`, which clients of the package see as `ring.New`."

And from the `bufio` example: `bufio.Reader`, not `bufio.BufReader` —
"users see it as `bufio.Reader`, which is a clear, concise name."

**MixedCaps (§ Names — MixedCaps):**

> "Finally, the convention in Go is to use `MixedCaps` or `mixedCaps`
> rather than underscores to write multiword names."

**Package names (§ Names — Package names):**

> "By convention, packages are given lower case, single-word names;
> there should be no need for underscores or mixedCaps. Err on the
> side of brevity, since everyone using your package will be typing
> that name."

### Voice quotes

1. "Go provides C-style `/* */` block comments and C++-style `//` line
   comments. Line comments are the norm; block comments appear mostly
   as package comments, but are useful within an expression or to
   disable large swaths of code." (§ Commentary)

2. "It's neither idiomatic nor necessary to put `Get` into the getter's
   name." (§ Names — Getters)

3. "Err on the side of brevity, since everyone using your package will
   be typing that name." (§ Names — Package names)

4. "Such an error... is useful even if printed far from the call that
   caused it; it is much more informative than the plain 'no such file
   or directory'." (§ Errors)

5. "the visibility of a name outside a package is determined by whether
   its first character is upper case" (§ Names)

---

## Go Doc Comments (tip.golang.org)

URL: https://tip.golang.org/doc/comment  
Author: Russ Cox. The canonical modern reference for doc comment
format, rendered by `go doc` and pkg.go.dev.

### On comments

- "Every exported (capitalized) name should have a doc comment."
  (§ package — opening rule)
- Doc comments use **complete sentences**. A sentence fragment is
  never correct form for a doc comment.
- "Doc comments should not explain internal details such as the
  algorithm used in the current implementation." (§ funcs) — the
  audience is callers, not maintainers.
- Length is proportional to complexity. A simple function warrants
  one sentence. Large packages may have multi-paragraph overviews
  with cross-links.
- "Reports whether" is the canonical phrase for boolean-returning
  functions. "The phrase 'or not' is unnecessary." (§ funcs)
- Unexported declarations are exempt from the doc comment rule;
  inline comments are appropriate for implementation detail.

### On godoc

**Sentence shape:**
- Package comments begin: `// Package [name] ...`
- Command comments begin with the program name (capitalized even if
  the binary name is lowercase): `// Cmd is ...`
- Function comments begin with the function name: `// Quote returns ...`
- Type comments state what instances represent: `// A Reader serves ...`
  or `// Reader serves ...`
- Methods on the same type should use the same receiver variable name
  throughout to "avoid needless variation." (§ funcs)

**Deprecated convention:**

> "Paragraphs starting with `Deprecated:` are treated as deprecation
> notices. Some tools will warn when deprecated identifiers are used."
> (§ syntax — Deprecations)

pkg.go.dev hides deprecated docs by default; gopls issues warnings
at use sites.

**Formatting — paragraphs:**
Unindented non-blank lines. `gofmt` preserves line breaks; it does
not rewrap. This enables semantic linefeeds (one sentence per line).
Consecutive backticks → left-quote; consecutive single-quotes →
right-quote.

**Formatting — lists:**
`*`, `+`, `-`, `•` or digit+`.`/`)` followed by space/tab. List
items contain only paragraphs (no code blocks, no nested lists).
`gofmt` normalizes bullets to `-` with two-space indent, continuation
four spaces.

**Formatting — links:**
Reference style: `[Text]: URL`. Doc links: `[Name]`, `[pkg.Name]`,
`[*pkg.Name]` — auto-hyperlinked to exported identifiers.

**Formatting — headings:**
`# Heading text` on its own line, blank lines above and below.
Available since Go 1.19.

**Formatting — code blocks:**
Indented lines (not list markers) render as preformatted text.
`gofmt` normalizes to single-tab indent with surrounding blank lines.

### On error phrasing

Not addressed in this document.

### On naming

- Single-letter names like `a` risk "being mistaken for ordinary
  words" in doc comments — prefer explicit subjects. (§ funcs)
- Receiver names: identical across all methods of a type. (§ funcs)
- The document's primary concern is doc comment structure, not
  identifier naming; see Effective Go and CodeReviewComments for
  naming rules.

### Voice quotes

1. "Every exported (capitalized) name should have a doc comment."
   (§ package)

2. "Doc comments should not explain internal details such as the
   algorithm used in the current implementation." (§ funcs)

3. "Paragraphs starting with `Deprecated:` are treated as deprecation
   notices." (§ syntax — Deprecations)

4. "Doc comments use complete sentences." (§ packages)

5. "The phrase 'or not' is unnecessary." (on "reports whether or not")
   (§ funcs)

---

## Go Code Review Comments

URL: https://go.dev/wiki/CodeReviewComments  
Community wiki, maintained by the Go team. Widely cited in code
review tooling and linters.

### On comments

**§ Doc Comments:**

> "All top-level, exported names should have doc comments, as should
> non-trivial unexported type or function declarations."

The qualifier "non-trivial" is meaningful: trivial unexported helpers
(single-purpose, obvious from name) do not require documentation.

**§ Comment Sentences:**

> "Comments documenting declarations should be full sentences, even if
> that seems a little redundant. This approach makes them format well
> when extracted into godoc documentation. Comments should begin with
> the name of the thing being described and end in a period."

This is the tightest statement of the "name-first, period-last" rule.

**§ Package Comments:**

> "Package comments, like all comments to be presented by godoc, must
> appear adjacent to the package clause, with no blank line."

### On godoc

- Exported names: always require doc comment.
- Non-trivial unexported types/functions: also document (§ Doc Comments).
- Named result parameters add godoc noise:

  ```go
  func (n *Node) Parent1() (node *Node) {}  // Repetitive in godoc
  ```

  Prefer `func (n *Node) Parent1() *Node {}`. (§ Named Result Parameters)
- Package comments open with a capitalized sentence:
  `// Package math provides basic constants and mathematical functions.`

### On error phrasing

**§ Error Strings:**

> "Error strings should not be capitalized (unless beginning with
> proper nouns or acronyms) or end with punctuation, since they are
> usually printed following other context."

Canonical example:
- Use: `fmt.Errorf("something bad")`
- Not: `fmt.Errorf("Something bad.")`

Rationale: `log.Printf("Reading %s: %v", filename, err)` should not
produce a spurious mid-message capital. The rule explicitly does not
apply to logging output, which "is implicitly line-oriented and not
combined inside other messages."

### On naming

**§ Package Names:**

> "All references to names in your package will be done using the
> package name, so you can omit that name from the identifiers."

Example: In package `chubby`, type `File` (not `ChubbyFile`).

> "Avoid meaningless package names like util, common, misc, api,
> types, and interfaces."

**§ Mixed Caps:**

> "This applies even when it breaks conventions in other languages.
> For example an unexported constant is `maxLength` not `MaxLength` or
> `MAX_LENGTH`."

**§ Initialisms:**

> "Words in names that are initialisms or acronyms (e.g. 'URL' or
> 'NATO') have a consistent case. For example, 'URL' should appear as
> 'URL' or 'url' (as in 'urlPony', or 'URLPony'), never as 'Url'."
> `ServeHTTP` not `ServeHttp`. `appID` not `appId`.

Getters: same rule as Effective Go — no `Get` prefix. Not restated
explicitly but implied by the broader naming sections.

### Voice quotes

1. "Comments should begin with the name of the thing being described
   and end in a period." (§ Comment Sentences)

2. "Error strings should not be capitalized... or end with
   punctuation, since they are usually printed following other
   context." (§ Error Strings)

3. "Avoid meaningless package names like util, common, misc, api,
   types, and interfaces." (§ Package Names)

4. "This applies even when it breaks conventions in other languages."
   (§ Mixed Caps, on `maxLength` vs `MAX_LENGTH`)

5. "'URL' should appear as 'URL' or 'url'... never as 'Url'."
   (§ Initialisms)

---

## Google Go Style Guide

URLs: https://google.github.io/styleguide/go/decisions  
     https://google.github.io/styleguide/go/guide  
Large corporate guide; spans a "guide" (principles) and "decisions"
(specific rules) site. The most comprehensive of the five sources.

### On comments

**§ doc-comments (decisions):**

> "All top-level exported names must have doc comments, as should
> unexported type or function declarations with unobvious behavior
> or meaning."

The qualifier "unobvious behavior or meaning" is stricter than
CodeReviewComments' "non-trivial" — it explicitly excludes obvious
unexported helpers.

**§ clarity-rationale (guide):**

> "It is often better for comments to explain why something is done,
> not what the code is doing."

And immediately: "Allow the code to speak for itself (e.g., by
making the symbol names themselves self-describing) rather than
adding redundant comments." Commentary that restates the code
"obscures the code's purpose by adding clutter, restating what the
code already says, contradicting the code, or adding maintenance
burden."

**§ comment-line-length (decisions):**

"There is no fixed line length for comments in Go." Soft target of
80–100 columns, but semantic grouping wins over column alignment.

**Inline comments:**

> "Avoid adding inline comments to specific function arguments where
> possible." (§ function-formatting, decisions)

Prefer named constants, named types, or well-named variables over
`/* name */` comments on call-site arguments.

**Signal boosting:**

When code looks similar to an expected pattern but differs in a
subtle, consequential way, a comment is warranted specifically to
"boost the signal." The guide names this explicitly. (§ concision,
guide)

### On godoc

**§ doc-comments (decisions):**
- "If you have doc comments for unexported code, follow the same
  custom as if it were exported (namely, starting the comment with
  the unexported name)."
- Sentence shape examples: `"A Request represents a request to run
  a command"` and `"Encode writes the JSON encoding of req to w."` —
  both name-first, complete sentence, period at end.
- Package comments must appear immediately above the `package`
  clause with no blank line between. (§ package-comments, decisions)

**§ examples (decisions):**

> "Packages should clearly document their intended usage. Try to
> provide a runnable example; examples show up in Godoc."

Runnable examples live in `_test.go` files, using the `Example`
function convention.

**Comment sentences (§ comment-sentences, decisions):**
"Comments that are complete sentences should be capitalized and
punctuated like standard English sentences. Comments that are
sentence fragments have no such requirements for punctuation or
capitalization." — This is the only source to explicitly distinguish
full-sentence comments from fragment comments and apply different
rules to each.

### On error phrasing

**§ error-strings (decisions):**

> "Error strings should not be capitalized (unless beginning with an
> exported name, a proper noun or an acronym) and should not end with
> punctuation."

Rationale: "error strings usually appear within other context before
being printed to the user."

- Bad: `fmt.Errorf("Something bad happened.")`
- Good: `fmt.Errorf("something bad happened")`

Additional nuance not in CodeReviewComments:

> "On the other hand, the style for the full displayed message
> (logging, test failure, API response, or other UI) depends, but
> should typically be capitalized."

This separates error *string values* (lowercase, no punctuation)
from *logged/displayed messages* (normal sentence casing).

### On naming

**§ getters (decisions):**

> "Function and method names should not use a `Get` or `get` prefix,
> unless the underlying concept uses the word 'get' (e.g. an HTTP
> GET). Prefer starting the name with the noun directly, for example
> use `Counts` over `GetCounts`."

**§ repetition (decisions) — package vs. exported symbol:**

> "When naming exported symbols, the name of the package is always
> visible outside your package, so redundant information between the
> two should be reduced or eliminated."

Concrete examples: `widget.NewWidget` → `widget.New`;
`db.LoadFromDatabase` → `db.Load`.

**§ variable-names (decisions):**

> "The general rule of thumb is that the length of a name should be
> proportional to the size of its scope and inversely proportional
> to the number of times that it is used within that scope."

And: "Omit words that are clear from the surrounding context. For
example, in the implementation of a `UserCount` method, a local
variable called `userCount` is probably redundant; `count`, `users`,
or even `c` are just as readable."

**§ constant-names (decisions):**
"Constant names must use MixedCaps like all other names in Go."

**§ initialisms (decisions):**
Same as CodeReviewComments: `URL`/`url`, never `Url`; `ServeHTTP`
not `ServeHttp`.

**§ naming (guide):**
Names should "Not feel repetitive when they are used," "Take the
context into consideration," and "Not repeat concepts that are
already clear."

### Voice quotes

1. "Allow the code to speak for itself (e.g., by making the symbol
   names themselves self-describing) rather than adding redundant
   comments." (§ clarity-rationale, guide)

2. "Comments that are complete sentences should be capitalized and
   punctuated like standard English sentences. Comments that are
   sentence fragments have no such requirements." (§ comment-sentences,
   decisions)

3. "Function and method names should not use a `Get` or `get` prefix,
   unless the underlying concept uses the word 'get' (e.g. an HTTP
   GET)." (§ getters, decisions)

4. "The length of a name should be proportional to the size of its
   scope and inversely proportional to the number of times that it is
   used within that scope." (§ variable-names, decisions)

5. "Redundant information between [package name and symbol name]
   should be reduced or eliminated." (§ repetition, decisions)

---

## Uber Go Style Guide

URL: https://github.com/uber-go/guide/blob/master/style.md  
Raw markdown: https://raw.githubusercontent.com/uber-go/guide/master/style.md  
Production-oriented guide from Uber's Go platform team. Heaviest
coverage on error handling and practical code structure; lightest on
pure documentation conventions.

### On comments

The guide has no dedicated comments section. Commentary philosophy
is demonstrated implicitly through examples:

- The **Error Naming** section uses a comment to explain visibility
  intent: "The following two errors are exported so that users of
  this package can match them with errors.Is." — comments explain
  *why* an API decision was made, not *what* the code does.
- The **Avoid Naked Parameters** section recommends C-style inline
  comments (`/* name */`) for unclear boolean/numeric arguments at
  call sites — but prefers named types that eliminate the need.
- No guidance on comment length, voice, or when to omit.

### On godoc

Not explicitly addressed. The guide assumes readers know Go godoc
conventions and focuses on code shape rather than documentation
format.

Public API surface is called out: "exported error variables or types
... will become part of the public API of the package," implying
they require documentation, but the guide does not prescribe format.

### On error phrasing

**§ Error Wrapping:**

> "Add context to the error message where possible so that instead of
> a vague error such as 'connection refused', you get more useful
> errors such as 'call service foo: connection refused'."

> "Keep the context succinct by avoiding phrases like 'failed to',
> which state the obvious and pile up as the error percolates up
> through the stack."

The bad/good contrast: `"failed to x: failed to y: failed to create
new store: the error"` → `"x: y: new store: the error"`. The
stripped-down form is a stronger stylistic claim than any other
source makes — "failed to" is not merely redundant but actively
harmful accumulation.

**§ Error Naming:**

> "For error values stored as global variables, use the prefix `Err`
> or `err` depending on whether they're exported."

Custom error types: suffix `Error` (e.g., `ValidationError`), not
prefix `Err`.

**Handle errors once (§ Handle Errors Once):**

> "The caller should not, for example, log the error and then return
> it, because its callers may handle the error as well."

Log *or* return; not both. This affects how error strings are
composed — they must be legible in isolation because they may surface
only once, either in a log or a returned value, not both.

### On naming

**§ Package Names:**

> "choose a name that is: All lower-case. No capitals or underscores.
> Does not need to be renamed using named imports at most call sites."

Package-doubled types are avoided implicitly: `client` provides
`Client`, not `ClientClient`.

**§ Function Names:**

> "We follow the Go community's convention of using MixedCaps for
> function names. An exception is made for test functions, which may
> contain underscores for the purpose of grouping related test cases."

**§ Error Naming:**
`Err`/`err` prefix for sentinel error variables; `Error` suffix for
custom error types.

**§ Prefix Unexported Globals with _:**

> "Prefix unexported top-level vars and consts with _ to make it
> clear when they are used that they are global symbols."

This is a Uber-specific convention — none of the other sources
mention it. Go community practice is mixed; the standard library
does not follow this rule.

**Getters:** Not addressed.
**Descriptive locals:** Not addressed explicitly.

### Voice quotes

1. "Keep the context succinct by avoiding phrases like 'failed to',
   which state the obvious and pile up as the error percolates up
   through the stack." (§ Error Wrapping)

2. "Add context to the error message where possible so that instead
   of a vague error such as 'connection refused', you get more useful
   errors such as 'call service foo: connection refused'."
   (§ Error Wrapping)

3. "The caller should not, for example, log the error and then return
   it, because its callers may handle the error as well."
   (§ Handle Errors Once)

4. "We follow the Go community's convention of using MixedCaps for
   function names. An exception is made for test functions."
   (§ Function Names)

5. "Prefix unexported top-level vars and consts with _ to make it
   clear when they are used that they are global symbols."
   (§ Prefix Unexported Globals with _)

---

## Synthesis Hooks

### Cross-source agreements (all or nearly all sources)

1. **Godoc sentences begin with the symbol name.** Effective Go,
   tip.golang.org, CodeReviewComments, and Google all demonstrate or
   state this. A doc comment for `Foo` opens with `// Foo ...`. This
   is the single most consistent rule across all sources.

2. **Every exported name must have a doc comment.** Stated explicitly
   in tip.golang.org ("Every exported (capitalized) name should have a
   doc comment"), CodeReviewComments, and Google. Effective Go implies
   it through consistent example. Uber assumes it.

3. **Error strings: lowercase, no trailing punctuation.** Stated
   explicitly in CodeReviewComments (§ Error Strings) and Google
   (§ error-strings, decisions). The rationale is identical in both:
   error strings compose with surrounding context via `%v` chains, so
   a leading capital or trailing period produces malformed output.
   Effective Go's examples are consistent with this, though not
   stated as a rule.

4. **No `Get` prefix on getter methods.** Effective Go (§ Getters)
   and Google (§ getters) state this explicitly. CodeReviewComments
   implies it. Uber does not address it.

5. **No package-doubled type names.** Effective Go (`ring.New` not
   `ring.NewRing`; `bufio.Reader` not `bufio.BufReader`), Google
   (`widget.New` not `widget.NewWidget`), and CodeReviewComments
   (`chubby.File` not `chubby.ChubbyFile`) all prohibit redundancy
   between package name and exported name.

6. **MixedCaps throughout; no underscores; no ALL_CAPS constants.**
   Effective Go, CodeReviewComments, Google, and Uber all state this.
   The rule holds even for unexported constants (`maxLength`, not
   `MAX_LENGTH`).

7. **Initialisms keep consistent case: `URL` or `url`, never `Url`.**
   CodeReviewComments (§ Initialisms) and Google (§ initialisms) both
   state this with identical examples. `ServeHTTP` not `ServeHttp`.

8. **Comments explain why, not what.** Google (§ clarity-rationale)
   states this most explicitly. Uber's examples embody it. Effective
   Go's sparse comments model it. tip.golang.org's rule that doc
   comments should not "explain internal details such as the algorithm
   used in the current implementation" is the same principle applied
   to godoc specifically.

9. **Variable name length is proportional to scope size.** Google
   (§ variable-names) states the principle most precisely. Effective
   Go's examples follow it. Short names (`c`, `b`, `n`) are idiomatic
   at tight scope; descriptive names are warranted at wide scope.

10. **Error context is additive and succinct.** Uber (§ Error Wrapping)
    and Effective Go's `PathError` example both model this — the chain
    `op: path: underlying` conveys progressively more specific context.
    Uber explicitly bans "failed to" as noise; Effective Go bans "no
    such file or directory" as not enough.

### Tensions and disagreements

1. **How much to document unexported code.** CodeReviewComments says
   "non-trivial unexported type or function declarations" should have
   doc comments. Google says "unexported type or function declarations
   with unobvious behavior or meaning." These are close but not
   identical. CodeReviewComments' "non-trivial" could include complex
   functions whose intent is still obvious from the name; Google's
   "unobvious" is a harder bar. tip.golang.org exempts unexported
   declarations entirely from the doc comment rule, leaving them to
   inline comments. In practice, the synthesis pass should treat
   Google's "unobvious" as the operative standard: document when the
   name doesn't tell the whole story.

2. **Sentence fragments in comments.** Google explicitly distinguishes:
   full-sentence comments require capitalization and terminal period;
   sentence fragments do not. CodeReviewComments requires "full
   sentences" for comments "documenting declarations" but is silent on
   inline comments. tip.golang.org says "complete sentences" for doc
   comments. None address the practical question of short imperative
   inline notes (`// fast path` vs `// Fast path.`). This is a genuine
   gap — the synthesis guide must take a position.

3. **"failed to" prefix in errors.** Uber explicitly forbids
   "failed to" as noise that accumulates. Effective Go's examples
   (`"image: unknown format"`, `"open /etc/passwd: no such file"`)
   never use it. But many Go standard library errors do use it
   (`"failed to parse"`, `"failed to connect"`), and it is common in
   real-world Go codebases. CodeReviewComments and Google's decisions
   page do not address this phrase. The Uber position is stylistically
   stronger but is a Uber-specific rule, not a community consensus.

4. **Underscore prefix for unexported globals.** Uber recommends `_`
   prefix on unexported package-level vars and consts (`_defaultPort`).
   No other source mentions this. It is not idiomatic in the standard
   library or major OSS Go projects. A synthesis guide for a project
   following Effective Go + CodeReviewComments conventions should
   probably reject this rule rather than synthesize it.
