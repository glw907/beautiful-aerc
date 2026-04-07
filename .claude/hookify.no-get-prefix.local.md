---
name: no-get-prefix
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: func\s*\([^)]+\)\s+Get[A-Z]
---

**`Get` prefix on getter methods is non-idiomatic Go**

The exported first letter already signals "accessor". Drop the `Get`:

```go
// Correct
func (u *User) Name() string { return u.name }
func (j *Journal) Path() string { return j.path }

// Wrong
func (u *User) GetName() string { return u.name }
func (j *Journal) GetPath() string { return j.path }
```

Source: Effective Go – Getters, Google Go Style, CodeReviewComments
