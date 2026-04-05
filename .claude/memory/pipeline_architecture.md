---
name: HTML filter pipeline architecture
description: Current Go pipeline stages for HTML-to-markdown email conversion, ordering, and what each stage does
type: project
---

The HTML email filter is a Go binary (`beautiful-aerc html`) that
converts HTML email to syntax-highlighted markdown for aerc's viewer.

**Pipeline stages (in order):**

1. `prepareHTML` -- strip Mozilla attributes, hidden elements
   (display:none divs), and zero-size tracking images
2. `runPandoc` -- HTML to markdown via pandoc with extensions disabled
   (-raw_html, -native_divs, -native_spans, -header_attributes,
   -bracketed_spans, -fenced_divs, -inline_code_attributes,
   -link_attributes) plus unwrap-tables.lua Lua filter
3. `html.UnescapeString` -- decode HTML entities
4. `cleanPandocArtifacts` -- trailing backslashes, escaped punctuation,
   consecutive bold markers, stray bold, superscript carets, nested
   headings, empty headings
5. `normalizeBoldMarkers` -- balance ** markers per paragraph, strip
   unpaired trailing markers
6. `normalizeLists` -- convert Unicode bullets to markdown items, strip
   excess indentation, compact loose lists
7. `normalizeWhitespace` -- NBSP, zero-width chars, blank line cleanup
8. `convertToFootnotes` -- reference-style links to numbered footnotes
9. `styleFootnotes` -- ANSI colors for footnote markers and ref section
10. `highlightMarkdown` -- ANSI colors for headings, bold, italic, rules

**Key files:**
- `internal/filter/html.go` -- pipeline stages 1, 3-7, 10
- `internal/filter/footnotes.go` -- stages 8-9
- `internal/palette/palette.go` -- color token loading
- `.config/aerc/filters/unwrap-tables.lua` -- pandoc Lua filter

**Why:** Understanding the stage ordering matters because fixes must
target the right stage. Regex cleanup after pandoc is intentional --
pandoc's markdown output has artifacts that can't be prevented by
pandoc flags alone.

**How to apply:** When debugging a rendering issue, trace the email
through each stage to find where the problem is introduced. Use
`corpus/` to save problem emails for batch fixing.
