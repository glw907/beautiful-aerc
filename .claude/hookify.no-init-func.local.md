---
name: no-init-func
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: func init\(\)
---

**Package-level init() is banned** (go-conventions.md)

`init()` hides side effects and makes initialization order unpredictable. Pass dependencies explicitly instead:

```go
// Correct: explicit initialization
func NewStore(cfg Config) (*Store, error) { ... }

// Wrong: hidden global setup
func init() { globalStore = mustConnect(cfg) }
```
