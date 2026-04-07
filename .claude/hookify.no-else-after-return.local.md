---
name: no-else-after-return
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: (return|panic\([^)]*\))\s*\n\s*\}\s*else\s*\{
---

**Don't use `else` after `return`, `panic`, or `continue`** (go-conventions.md)

The else branch is unreachable — remove it and dedent the happy path:

```go
// Correct — guard clause style
if err != nil {
    return fmt.Errorf("loading: %w", err)
}
doSomething()

// Wrong — unnecessary nesting
if err != nil {
    return fmt.Errorf("loading: %w", err)
} else {
    doSomething()
}
```

Source: go-conventions.md ("Return early. Guard clauses, not if/else chains"), Uber Style – Unnecessary Else, Google Style – Indent Error Flow
