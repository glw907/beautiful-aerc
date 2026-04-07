---
name: no-nolint
enabled: true
event: file
action: block
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.go$
  - field: new_text
    operator: contains
    pattern: "//nolint"
---

**nolint pragmas are banned** (go-conventions.md)

`//nolint` silences the linter instead of fixing the problem. The convention is explicit: "No nolint pragmas. Fix the code instead."

Address the underlying lint issue directly.
