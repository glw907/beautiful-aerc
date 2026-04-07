---
name: nil-slice-not-empty
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: :=\s*\[\]\w[\w*]*\{\}
---

**Prefer `var t []T` over `t := []T{}` for empty slices**

`[]T{}` allocates an empty non-nil slice unnecessarily. A nil slice is idiomatic when no elements are being added immediately:

```go
// Correct
var entries []Entry
var tags []string

// Wrong (unnecessary allocation)
entries := []Entry{}
tags := []string{}
```

**Exception:** When `nil` vs empty matters at a JSON or API boundary (JSON encodes nil slice as `null`, empty slice as `[]`), `[]T{}` is intentional — add a comment explaining why.

Source: CodeReviewComments – Declaring Empty Slices, Uber Style Guide
