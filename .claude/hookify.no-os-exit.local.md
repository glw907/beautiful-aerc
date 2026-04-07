---
name: no-os-exit
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: os\.Exit\(
---

**os.Exit is only allowed in main.go** (go-conventions.md)

`os.Exit` bypasses deferred cleanup and makes code untestable. Per conventions, only `main()` prints errors and calls `os.Exit(1)`.

Library functions must return errors. Let the caller decide what to do.
