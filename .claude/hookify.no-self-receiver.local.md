---
name: no-self-receiver
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: func\s*\(\s*(this|self|me)\s+
---

**Receiver named `this`, `self`, or `me` is non-idiomatic Go**

Go receivers should be a short abbreviation of the type name — typically 1-2 letters:

```go
// Correct
func (e *Entry) String() string { ... }
func (j *Journal) Add(e Entry) { ... }

// Wrong
func (this *Entry) String() string { ... }
func (self *Journal) Add(e Entry) { ... }
```

Source: CodeReviewComments – Receiver Names, Google Go Style
