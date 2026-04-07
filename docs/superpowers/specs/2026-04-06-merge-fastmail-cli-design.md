# Merge fastmail-cli into beautiful-aerc

> **For agentic workers:** This spec documents a code move, not a feature build.

**Goal:** Consolidate fastmail-cli (Fastmail JMAP CLI) into the
beautiful-aerc repository so both tools ship from a single codebase
and Stow package.

**Motivation:** Both tools serve the same domain -- aerc email
configuration and Fastmail integration. Publishing them together
simplifies installation, testing, and contribution for future users.

---

## Decisions

| Question | Answer |
|----------|--------|
| Binary names | Keep both: `beautiful-aerc` and `fastmail-cli` |
| Old repo (`glw907/fastmail-cli`) | Delete (hours old, private, no consumers) |
| fastmail-cli's design docs | Drop (completed task, code is source of truth) |
| Package layout | Flat: `internal/jmap/`, `internal/header/`, `internal/rules/` alongside existing packages |

---

## Repository Structure After Merge

```
beautiful-aerc/
  cmd/
    beautiful-aerc/          # existing (headers, html, plain, pick-link, save)
    fastmail-cli/            # new (rules, masked, folders, version)
  internal/
    palette/                 # existing
    filter/                  # existing
    picker/                  # existing
    corpus/                  # existing
    jmap/                    # from fastmail-cli
    header/                  # from fastmail-cli
    rules/                   # from fastmail-cli
  e2e/                       # existing beautiful-aerc e2e tests
  e2e-fastmail/              # fastmail-cli e2e tests (renamed to avoid collision)
```

---

## Files Moved

From `~/Projects/fastmail-cli/` into `~/Projects/beautiful-aerc/`:

| Source | Destination |
|--------|-------------|
| `cmd/fastmail-cli/` (all .go files) | `cmd/fastmail-cli/` |
| `internal/jmap/` | `internal/jmap/` |
| `internal/header/` | `internal/header/` |
| `internal/rules/` | `internal/rules/` |
| `e2e/` | `e2e-fastmail/` |

---

## Files Merged

| File | Action |
|------|--------|
| `Makefile` | Add `fastmail-cli` binary target; `build`, `install`, `clean` cover both binaries |
| `.gitignore` | Add `/fastmail-cli` |
| `CLAUDE.md` | Add fastmail-cli section (architecture, command structure, env vars) |
| `.golangci.yml` | No change needed (files are identical) |

---

## Files Dropped

| File | Reason |
|------|--------|
| fastmail-cli `go.mod` / `go.sum` | Absorbed into beautiful-aerc's module |
| fastmail-cli `Makefile` | Merged into beautiful-aerc's |
| fastmail-cli `CLAUDE.md` | Merged into beautiful-aerc's |
| fastmail-cli `.gitignore` | Merged into beautiful-aerc's |
| fastmail-cli `docs/superpowers/` | Completed task docs |

---

## Import Path Changes

All Go imports in `cmd/fastmail-cli/` change:

```
github.com/glw907/fastmail-cli/internal/jmap
github.com/glw907/fastmail-cli/internal/header
github.com/glw907/fastmail-cli/internal/rules
```

becomes:

```
github.com/glw907/beautiful-aerc/internal/jmap
github.com/glw907/beautiful-aerc/internal/header
github.com/glw907/beautiful-aerc/internal/rules
```

---

## Hookify Rules

fastmail-cli has 16 hookify rule files in `.claude/`. beautiful-aerc
has none. All 16 are copied into beautiful-aerc's `.claude/` as-is.

---

## Makefile Changes

Current beautiful-aerc Makefile builds one binary. After merge:

```makefile
BINARIES := beautiful-aerc fastmail-cli

build:
	go build -o beautiful-aerc ./cmd/beautiful-aerc
	go build -o fastmail-cli ./cmd/fastmail-cli

install: build
	GOBIN=$(HOME)/.local/bin go install ./cmd/beautiful-aerc
	GOBIN=$(HOME)/.local/bin go install ./cmd/fastmail-cli

clean:
	rm -f beautiful-aerc fastmail-cli
```

`test`, `vet`, `lint`, `check` remain unchanged (`go test ./...`
already covers all packages).

---

## External References Updated

| Location | Change |
|----------|--------|
| `~/.claude/CLAUDE.md` | Update fastmail-cli project path from `~/Projects/fastmail-cli/` to `~/Projects/beautiful-aerc/` |

---

## No Changes Needed

- aerc `binds.conf` -- calls `fastmail-cli` by name, binary name unchanged
- `~/.local/bin/fastmail-cli` -- `make install` places it there
- Environment variables (`FASTMAIL_API_TOKEN`, `AERC_RULES_FILE`, etc.)
- aerc `aerc.conf` filter configuration

---

## Cleanup

After merge is committed and verified:

1. Delete `~/Projects/fastmail-cli/` directory
2. Delete `glw907/fastmail-cli` GitHub repo via `gh repo delete`
