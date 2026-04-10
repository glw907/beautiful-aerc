# Poplar Pass 1: Scaffold + Fork

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fork aerc's worker code into the monorepo and create a minimal `poplar` binary that compiles alongside the three existing binaries.

**Architecture:** Copy aerc's worker/, models/, and supporting lib/ packages into internal/, rewrite all import paths from `git.sr.ht/~rjarry/aerc/` to `github.com/glw907/beautiful-aerc/internal/`, replace `config.AccountConfig` with a minimal poplar-owned struct, and add new Go dependencies (go-jmap, go-imap, etc.).

**Tech Stack:** Go 1.25, cobra, go-jmap, go-imap, go-message

**Source:** aerc v0.21.0 at `/tmp/goaerctmp/pkg/mod/git.sr.ht/~rjarry/aerc@v0.0.0-20250828093418-5549850facc2/`

---

## File Structure

### New directories and files

```
internal/
  aercfork/                    # All forked aerc code lives under this namespace
    LICENSE                    # aerc MIT license (required by license terms)
    log/                       # Forked lib/log (logging interface)
      logger.go
    parse/                     # Forked lib/parse (header parsing)
      header.go
    xdg/                       # Forked lib/xdg (path expansion)
      home.go
      xdg.go
    auth/                      # Forked lib/oauthbearer.go, xoauth2.go
      oauthbearer.go
      xoauth2.go
    keepalive/                 # Forked lib/keepalive_linux.go + dummy
      keepalive_linux.go
      keepalive_dummy.go
    models/                    # Forked models/
      models.go
    worker/                    # Forked worker core
      worker.go                # NewWorker factory
      worker_enabled.go        # Import side-effects for IMAP + JMAP
      handlers/
        register.go
      types/
        messages.go
        worker.go
        mfs.go
        search.go
        sort.go
        thread.go
      lib/
        foldermap.go
        headers.go
        search.go
        size.go
        sort.go
      middleware/
        foldermapper.go
        gmailworker.go
      imap/
        *.go                   # All IMAP worker files
        extensions/
          liststatus.go
          xgmext/
            client.go
            search.go
            terms.go
      jmap/
        *.go                   # All JMAP worker files
        cache/
          *.go                 # All JMAP cache files
  poplar/                      # Poplar's own config (not forked)
    config.go                  # AccountConfig struct replacing aerc's
cmd/poplar/
  main.go
  root.go
```

### Modified files

```
Makefile                       # Add poplar to build/install/clean
go.mod / go.sum                # New dependencies from aerc's go.mod
```

### Key design decision: `internal/aercfork/` namespace

All forked code goes under `internal/aercfork/` rather than directly in `internal/`. This:
- Makes the fork boundary visible (what's ours vs. what's aerc)
- Simplifies future cherry-picks (clear mapping to aerc source tree)
- Avoids name collisions with existing `internal/` packages

---

## Task 1: Add aerc's Go dependencies

**Files:**
- Modify: `go.mod`

This task adds the third-party dependencies that aerc's worker code needs. Adding them first means subsequent tasks can focus on code without dependency errors.

- [ ] **Step 1: Add dependencies**

```bash
cd ~/Projects/beautiful-aerc
go get git.sr.ht/~rockorager/go-jmap@v0.5.2
go get github.com/emersion/go-imap@v1.2.1
go get github.com/emersion/go-imap-sortthread@v1.2.0
go get github.com/emersion/go-message@v0.18.2
go get github.com/emersion/go-sasl@v0.0.0-20241020182733-b788ff22d5a6
go get github.com/emersion/go-smtp@v0.24.0
go get github.com/pkg/errors
go get github.com/syndtr/goleveldb/leveldb
go get golang.org/x/oauth2
```

- [ ] **Step 2: Verify**

Run: `go mod tidy && go build ./...`
Expected: compiles (existing binaries still build)

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "Add aerc worker dependencies for poplar fork"
```

---

## Task 2: Fork support libraries (log, parse, xdg, auth, keepalive)

**Files:**
- Create: `internal/aercfork/LICENSE`
- Create: `internal/aercfork/log/logger.go`
- Create: `internal/aercfork/parse/header.go`
- Create: `internal/aercfork/xdg/home.go`
- Create: `internal/aercfork/xdg/xdg.go`
- Create: `internal/aercfork/auth/oauthbearer.go`
- Create: `internal/aercfork/auth/xoauth2.go`
- Create: `internal/aercfork/keepalive/keepalive_linux.go`
- Create: `internal/aercfork/keepalive/keepalive_dummy.go`

These are leaf dependencies with no internal imports (except log, which parse uses). Fork them first so downstream packages can import them.

- [ ] **Step 1: Create LICENSE**

Copy aerc's MIT license to `internal/aercfork/LICENSE`:
```
Copyright (c) 2018-2019 Drew DeVault
Copyright (c) 2021-2022 Robin Jarry

The MIT License
[full text]
```

- [ ] **Step 2: Fork log/logger.go**

Copy from `$AERC/lib/log/logger.go`. No import rewrites needed — it only imports stdlib. Change package to `log` (same name, different path).

We do NOT need `panic-logger.go` — it's only used by aerc's UI crash handler.

- [ ] **Step 3: Fork parse/header.go**

Copy from `$AERC/lib/parse/header.go`. Rewrite imports:
- `git.sr.ht/~rjarry/aerc/lib/log` -> `github.com/glw907/beautiful-aerc/internal/aercfork/log`

Other imports (`github.com/emersion/go-message/mail`, `strings`) stay as-is.

- [ ] **Step 4: Fork xdg/home.go and xdg/xdg.go**

Copy from `$AERC/lib/xdg/`. These only import stdlib. No rewrites needed.

- [ ] **Step 5: Fork auth/oauthbearer.go**

Copy from `$AERC/lib/oauthbearer.go`. Change package from `lib` to `auth`. Imports are all external (`go-imap`, `go-sasl`, `oauth2`) — no rewrites needed.

- [ ] **Step 6: Fork auth/xoauth2.go**

Copy from `$AERC/lib/xoauth2.go`. Change package from `lib` to `auth`. Rewrite:
- `git.sr.ht/~rjarry/aerc/lib/xdg` -> `github.com/glw907/beautiful-aerc/internal/aercfork/xdg`

- [ ] **Step 7: Fork keepalive (linux + dummy)**

Copy from `$AERC/lib/keepalive_linux.go` and `$AERC/lib/keepalive_dummy.go`. Change package from `lib` to `keepalive`. Stdlib only — no rewrites.

- [ ] **Step 8: Verify**

Run: `go build ./internal/aercfork/...`
Expected: all support packages compile

- [ ] **Step 9: Commit**

```bash
git add internal/aercfork/
git commit -m "Fork aerc support libraries (log, parse, xdg, auth, keepalive)"
```

---

## Task 3: Create poplar AccountConfig and fork models

**Files:**
- Create: `internal/poplar/config.go`
- Create: `internal/aercfork/models/models.go`

The AccountConfig struct replaces aerc's 30+ field struct with only what the workers need. The models package is forked with the config import replaced.

- [ ] **Step 1: Create internal/poplar/config.go**

```go
// Package poplar provides poplar-specific types shared across the application.
package poplar

import (
	"time"

	"github.com/emersion/go-message/mail"
)

// AccountConfig holds the configuration for a single email account.
// This replaces aerc's config.AccountConfig with only the fields
// that the forked workers actually use.
type AccountConfig struct {
	Name           string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string
	Headers        []string
	HeadersExclude []string
	CheckMail      time.Duration

	// Identity
	From    *mail.Address
	Aliases []*mail.Address
	CopyTo  []string

	// Outgoing
	Outgoing          string
	OutgoingCredCmd   string
}
```

- [ ] **Step 2: Fork models/models.go**

Copy from `$AERC/models/models.go`. Rewrite:
- `git.sr.ht/~rjarry/aerc/lib/parse` -> `github.com/glw907/beautiful-aerc/internal/aercfork/parse`

Do NOT fork `models/templates.go` — it's UI-specific.

- [ ] **Step 3: Verify**

Run: `go build ./internal/poplar/... ./internal/aercfork/models/...`
Expected: compiles

- [ ] **Step 4: Commit**

```bash
git add internal/poplar/ internal/aercfork/models/
git commit -m "Add poplar AccountConfig and fork aerc models"
```

---

## Task 4: Fork worker/types and worker/handlers

**Files:**
- Create: `internal/aercfork/worker/types/messages.go`
- Create: `internal/aercfork/worker/types/worker.go`
- Create: `internal/aercfork/worker/types/mfs.go`
- Create: `internal/aercfork/worker/types/search.go`
- Create: `internal/aercfork/worker/types/sort.go`
- Create: `internal/aercfork/worker/types/thread.go`
- Create: `internal/aercfork/worker/handlers/register.go`

The critical change here: `types/messages.go` imports `config.AccountConfig` for the `Configure` message. Replace with our `poplar.AccountConfig`.

- [ ] **Step 1: Fork worker/types/worker.go**

Copy from `$AERC/worker/types/worker.go`. Rewrite:
- `git.sr.ht/~rjarry/aerc/lib/log` -> `github.com/glw907/beautiful-aerc/internal/aercfork/log`
- `git.sr.ht/~rjarry/aerc/models` -> `github.com/glw907/beautiful-aerc/internal/aercfork/models`

- [ ] **Step 2: Fork worker/types/messages.go**

Copy from `$AERC/worker/types/messages.go`. Rewrite:
- `git.sr.ht/~rjarry/aerc/config` -> `github.com/glw907/beautiful-aerc/internal/poplar`
- `git.sr.ht/~rjarry/aerc/models` -> `github.com/glw907/beautiful-aerc/internal/aercfork/models`

In the `Configure` struct, change:
```go
// Before:
Config *config.AccountConfig
// After:
Config *poplar.AccountConfig
```

Update the import alias if needed (`poplar "github.com/glw907/beautiful-aerc/internal/poplar"`).

- [ ] **Step 3: Fork remaining types files**

Copy `mfs.go`, `search.go`, `sort.go`, `thread.go`. Rewrite aerc imports to aercfork paths. These files import `models` and `log` — same rewrites as above.

- [ ] **Step 4: Fork worker/handlers/register.go**

Copy from `$AERC/worker/handlers/register.go`. Rewrite:
- `git.sr.ht/~rjarry/aerc/worker/types` -> `github.com/glw907/beautiful-aerc/internal/aercfork/worker/types`

- [ ] **Step 5: Verify**

Run: `go build ./internal/aercfork/worker/types/... ./internal/aercfork/worker/handlers/...`
Expected: compiles

- [ ] **Step 6: Commit**

```bash
git add internal/aercfork/worker/types/ internal/aercfork/worker/handlers/
git commit -m "Fork aerc worker types and handler registry"
```

---

## Task 5: Fork worker/lib and worker/middleware

**Files:**
- Create: `internal/aercfork/worker/lib/foldermap.go`
- Create: `internal/aercfork/worker/lib/headers.go`
- Create: `internal/aercfork/worker/lib/search.go`
- Create: `internal/aercfork/worker/lib/size.go`
- Create: `internal/aercfork/worker/lib/sort.go`
- Create: `internal/aercfork/worker/middleware/foldermapper.go`
- Create: `internal/aercfork/worker/middleware/gmailworker.go`

Do NOT fork `worker/lib/maildir.go` — not needed for IMAP/JMAP.

- [ ] **Step 1: Fork worker/lib files**

Copy all except `maildir.go`. Rewrite aerc imports:
- `git.sr.ht/~rjarry/aerc/lib/log` -> log
- `git.sr.ht/~rjarry/aerc/models` -> models
- `git.sr.ht/~rjarry/aerc/worker/types` -> types
- `git.sr.ht/~rjarry/aerc/config` -> poplar (if used)

- [ ] **Step 2: Fork worker/middleware files**

Copy `foldermapper.go` and `gmailworker.go`. Same import rewrites plus:
- `git.sr.ht/~rjarry/aerc/worker/lib` -> `github.com/glw907/beautiful-aerc/internal/aercfork/worker/lib`
- `git.sr.ht/~rjarry/aerc/worker/handlers` -> handlers

- [ ] **Step 3: Verify**

Run: `go build ./internal/aercfork/worker/lib/... ./internal/aercfork/worker/middleware/...`
Expected: compiles

- [ ] **Step 4: Commit**

```bash
git add internal/aercfork/worker/lib/ internal/aercfork/worker/middleware/
git commit -m "Fork aerc worker lib and middleware"
```

---

## Task 6: Fork JMAP worker

**Files:**
- Create: `internal/aercfork/worker/jmap/*.go` (all files)
- Create: `internal/aercfork/worker/jmap/cache/*.go` (all cache files)

- [ ] **Step 1: Fork all JMAP files**

Copy all files from `$AERC/worker/jmap/` and `$AERC/worker/jmap/cache/`. Rewrite all aerc imports:
- `git.sr.ht/~rjarry/aerc/config` -> `github.com/glw907/beautiful-aerc/internal/poplar`
- `git.sr.ht/~rjarry/aerc/models` -> models
- `git.sr.ht/~rjarry/aerc/lib/log` -> log
- `git.sr.ht/~rjarry/aerc/worker/types` -> types
- `git.sr.ht/~rjarry/aerc/worker/handlers` -> handlers
- `git.sr.ht/~rjarry/aerc/worker/jmap/cache` -> cache (within jmap package)

In `worker.go`, change:
```go
// Before:
config struct {
    account    *config.AccountConfig
// After:
config struct {
    account    *poplar.AccountConfig
```

In `configure.go`, all `msg.Config.*` field accesses must match our `poplar.AccountConfig` fields. The fields used are: `.Params`, `.Name`, `.Source` — all present in our struct.

- [ ] **Step 2: Verify**

Run: `go build ./internal/aercfork/worker/jmap/...`
Expected: compiles

- [ ] **Step 3: Commit**

```bash
git add internal/aercfork/worker/jmap/
git commit -m "Fork aerc JMAP worker"
```

---

## Task 7: Fork IMAP worker

**Files:**
- Create: `internal/aercfork/worker/imap/*.go` (all files)
- Create: `internal/aercfork/worker/imap/extensions/liststatus.go`
- Create: `internal/aercfork/worker/imap/extensions/xgmext/*.go`

- [ ] **Step 1: Fork all IMAP files**

Copy all files from `$AERC/worker/imap/`, `$AERC/worker/imap/extensions/`. Rewrite all aerc imports:
- `git.sr.ht/~rjarry/aerc/config` -> poplar
- `git.sr.ht/~rjarry/aerc/models` -> models
- `git.sr.ht/~rjarry/aerc/lib/log` -> log
- `git.sr.ht/~rjarry/aerc/lib` -> split into auth and keepalive imports
- `git.sr.ht/~rjarry/aerc/lib/xdg` -> xdg
- `git.sr.ht/~rjarry/aerc/worker/types` -> types
- `git.sr.ht/~rjarry/aerc/worker/handlers` -> handlers
- `git.sr.ht/~rjarry/aerc/worker/lib` -> worker lib
- `git.sr.ht/~rjarry/aerc/worker/middleware` -> middleware
- `git.sr.ht/~rjarry/aerc/worker/imap/extensions` -> extensions (relative)

The `lib.OAuthBearer` and `lib.Xoauth2` types move to `auth.OAuthBearer` and `auth.Xoauth2`. The `lib.SetTcpKeepaliveProbes` and `lib.SetTcpKeepaliveInterval` functions move to `keepalive.SetTcpKeepaliveProbes` and `keepalive.SetTcpKeepaliveInterval`.

In `configure.go`, `msg.Config.*` accesses must match our struct. Fields used: `.Name`, `.Source`, `.Folders`, `.Headers`, `.HeadersExclude`, `.CheckMail`, `.Params` — all present.

- [ ] **Step 2: Verify**

Run: `go build ./internal/aercfork/worker/imap/...`
Expected: compiles

- [ ] **Step 3: Commit**

```bash
git add internal/aercfork/worker/imap/
git commit -m "Fork aerc IMAP worker"
```

---

## Task 8: Fork worker entry point and enabled imports

**Files:**
- Create: `internal/aercfork/worker/worker.go`
- Create: `internal/aercfork/worker/worker_enabled.go`

- [ ] **Step 1: Fork worker/worker.go**

Copy from `$AERC/worker/worker.go`. Rewrite:
- `git.sr.ht/~rjarry/aerc/worker/handlers` -> handlers
- `git.sr.ht/~rjarry/aerc/worker/types` -> types

- [ ] **Step 2: Create worker_enabled.go**

Only import IMAP and JMAP (not maildir, mbox, notmuch):

```go
package worker

import (
	_ "github.com/glw907/beautiful-aerc/internal/aercfork/worker/imap"
	_ "github.com/glw907/beautiful-aerc/internal/aercfork/worker/jmap"
)
```

- [ ] **Step 3: Verify**

Run: `go build ./internal/aercfork/worker/...`
Expected: entire forked worker tree compiles

- [ ] **Step 4: Commit**

```bash
git add internal/aercfork/worker/worker.go internal/aercfork/worker/worker_enabled.go
git commit -m "Fork aerc worker entry point (IMAP + JMAP only)"
```

---

## Task 9: Create cmd/poplar and update Makefile

**Files:**
- Create: `cmd/poplar/main.go`
- Create: `cmd/poplar/root.go`
- Modify: `Makefile`

- [ ] **Step 1: Create cmd/poplar/main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Create cmd/poplar/root.go**

```go
package main

import (
	"github.com/spf13/cobra"

	// Import forked workers for init() side effects (handler registration)
	_ "github.com/glw907/beautiful-aerc/internal/aercfork/worker"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "poplar",
		Short:        "A bubbletea-based terminal email client",
		SilenceUsage: true,
	}
	return cmd
}
```

The blank import of the worker package triggers `worker_enabled.go` which triggers IMAP and JMAP `init()` functions to register their handler factories. This validates that the entire forked tree compiles and links correctly.

- [ ] **Step 3: Update Makefile**

Add poplar to all targets:

```makefile
build:
	go build -o mailrender ./cmd/mailrender
	go build -o fastmail-cli ./cmd/fastmail-cli
	go build -o tidytext ./cmd/tidytext
	go build -o poplar ./cmd/poplar

install:
	GOBIN=$(HOME)/.local/bin go install ./cmd/mailrender
	GOBIN=$(HOME)/.local/bin go install ./cmd/fastmail-cli
	GOBIN=$(HOME)/.local/bin go install ./cmd/tidytext
	GOBIN=$(HOME)/.local/bin go install ./cmd/poplar

clean:
	rm -f mailrender fastmail-cli tidytext poplar
```

- [ ] **Step 4: Run the gate**

Run: `make build`
Expected: all four binaries compile successfully

Run: `make check`
Expected: vet + tests pass (existing tests unaffected)

Run: `./poplar --help`
Expected: prints usage for poplar

- [ ] **Step 5: Commit**

```bash
git add cmd/poplar/ Makefile
git commit -m "Add poplar binary scaffold and update Makefile"
```

---

## Task 10: Clean up and tidy

- [ ] **Step 1: Run go mod tidy**

```bash
go mod tidy
```

- [ ] **Step 2: Final gate check**

Run: `make check`
Expected: PASS

Run: `make build`
Expected: four binaries in project root

- [ ] **Step 3: Commit if go.sum changed**

```bash
git add go.mod go.sum
git commit -m "Tidy go module after poplar fork"
```
