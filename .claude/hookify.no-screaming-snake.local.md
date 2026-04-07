---
name: no-screaming-snake
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: \b[A-Z]{2,}_[A-Z]{2,}\b
---

**`SCREAMING_SNAKE_CASE` constants are non-idiomatic Go**

Go uses `MixedCaps` for all identifiers, including constants:

```go
// Correct
const MaxEntries = 100
const DefaultHour = 8

// Wrong
const MAX_ENTRIES = 100
const DEFAULT_HOUR = 8
```

Source: Google Go Style – Constant Names, CodeReviewComments
