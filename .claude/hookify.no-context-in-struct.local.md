---
name: no-context-in-struct
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: \b\w+\s+context\.Context\b
---

**Do not store `context.Context` in a struct**

Contexts are request-scoped. Storing one in a struct makes its lifetime ambiguous and breaks cancellation propagation. Pass it as the first function parameter instead:

```go
// Correct
func (j *Journal) Load(ctx context.Context, path string) error { ... }

// Wrong
type Journal struct {
    ctx  context.Context  // don't do this
    path string
}
```

Exception: types that explicitly wrap a context (e.g. `context.WithValue` return types) are fine.

Source: CodeReviewComments – Contexts, Google Go Style – Contexts
