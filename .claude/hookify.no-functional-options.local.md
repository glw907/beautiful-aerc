---
name: no-functional-options
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: func With[A-Z]
---

**Functional options are banned** (go-conventions.md)

`func WithBar(...)` and `func WithBaz(...)` are a JavaScript/Java pattern. Use a plain config struct instead:

```go
// Correct
func NewFoo(cfg Config) *Foo { ... }

// Wrong
func NewFoo(opts ...Option) *Foo { ... }
func WithBar(v string) Option { ... }
```

Plain structs are simpler, testable, and idiomatic Go for CLI tools.
