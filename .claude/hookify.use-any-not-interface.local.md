---
name: use-any-not-interface
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: \binterface\{\}
---

**Use `any` instead of `interface{}`** (Go 1.18+)

`any` is the canonical alias since Go 1.18 and is more readable:

```go
// Correct
func marshal(v any) ([]byte, error) { ... }
var data map[string]any

// Wrong
func marshal(v interface{}) ([]byte, error) { ... }
var data map[string]interface{}
```

Source: CodeReviewComments – Use any, Google Go Style
