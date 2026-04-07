---
name: lowercase-errors
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: (errors\.New|fmt\.Errorf)\(\s*"[A-Z]
---

**Error strings must be lowercase** (go-conventions.md)

Error strings appear mid-sentence in larger messages — a capital letter or trailing punctuation looks wrong:

```go
// Correct
return fmt.Errorf("loading config %s: %w", path, err)
return errors.New("journal path not set")

// Wrong
return fmt.Errorf("Loading config %s: %w", path, err)
return errors.New("Journal path not set.")
```

Also check: error strings must not end with `.`, `!`, or `?`.

Source: go-conventions.md, CodeReviewComments – Error Strings
