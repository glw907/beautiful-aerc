---
name: no-util-packages
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: ^package\s+(util|utils|common|helper|helpers|shared|misc|lib)\s*$
---

**Vague package names like `util`, `common`, `helper` are banned**

Packages should be named by what they provide, not that they are "useful":

```
// Correct
package atomicfile   // writes files atomically
package timeparse    // parses jrnl date expressions

// Wrong
package util
package common
package helpers
```

A vague name is a signal the package is doing too many unrelated things. Split it by concern.

Source: CodeReviewComments – Package Names, Google Go Style, Uber Style Guide
