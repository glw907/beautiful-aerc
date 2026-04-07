---
name: require-make-check
enabled: true
event: stop
pattern: .*
---

**Run `make check` before claiming done** (go-conventions.md)

`make check` runs `go vet ./...` + `go test ./...` — the mandatory gate before committing.

If the Makefile target doesn't exist yet for this project, run:
```
go vet ./... && go test ./...
```

Do not mark work complete until both pass with no failures.
