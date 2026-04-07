---
name: no-log-fatal
enabled: true
event: file
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: regex_match
    pattern: log\.Fatal
---

**log.Fatal is only allowed in main.go** (go-conventions.md)

`log.Fatal` calls `os.Exit(1)` after logging, which bypasses deferred cleanup and makes the call site untestable. Library functions must return errors instead.

Only `main()` is allowed to print and exit.
