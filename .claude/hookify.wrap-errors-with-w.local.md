---
name: wrap-errors-with-w
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: fmt\.Errorf\([^)]*%v[^)]*\berr\b
---

**Use `%w` not `%v` when wrapping errors** (go-conventions.md)

`%v` converts the error to a string, destroying the error chain. `errors.Is` and `errors.As` will stop working on the wrapped error.

```go
// Correct — preserves error chain
return fmt.Errorf("reading day file %s: %w", path, err)

// Wrong — breaks errors.Is / errors.As
return fmt.Errorf("reading day file %s: %v", path, err)
```

Source: go-conventions.md, Go Blog – Error Wrapping, 100 Go Mistakes #49
