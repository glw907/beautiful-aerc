# Human-voice audit — `cmd/poplar/`

**Tally:** C1=3 (T2=3), C4=5 (T3=1, T7=1, T31=1, T11=2), C5=1 (T18=1), C6=3 (T9=1, T24=1, combined test-name=1), C7=3 (T10b=2, T13=1)

---

## C1 — Comment rot

### cache.go:108–112 — C1 (T2)

```go
// loadAccounts reads account config via the existing config loader.
func loadAccounts() ([]config.AccountConfig, string, error) {
	accts, path, err := config.Load("")
	return accts, path, err
}
```

Godoc on a 3-line wrapper that does nothing beyond delegating to `config.Load("")`. The comment's only content beyond the name is "via the existing config loader" — noise that adds zero caller information.

### cache.go:139–140 — C1 (T2)

```go
// formatThousands inserts commas as thousands separators.
func formatThousands(n int64) string {
```

Function name is `formatThousands`. Comment restates it verbatim in different words.

### config.go:9–10 — C1 (T2)

```go
// newConfigCmd creates the parent `poplar config` command.
func newConfigCmd() *cobra.Command {
```

Six-line function. Comment restates the name; "parent" qualifier adds nothing the call site doesn't already convey.

---

## C4 — Uniform verbosity

### config_cmd.go:29–82 — C4 (T3 / T31)

Three consecutive unexported `newXxxCmd` functions each receive an identical 2-line comment structure — `"creates the X subcommand, which Y"` — regardless of body complexity:

```go
// newConfigInitTemplateCmd creates the `poplar config init` subcommand,
// which writes a fresh self-documenting config template to disk.

// newConfigPathCmd creates the `poplar config path` subcommand,
// which prints the resolved config-file path without reading it.

// newConfigCheckCmd creates the `poplar config check` subcommand,
// which validates the config file and tests each account's connection.
```

`newConfigPathCmd` is a 4-line trivial function; `newConfigCheckCmd` loops over accounts attempting connections. Both get the same metronomic 2-sentence shape.

### cache.go:30–239 — C4 (T7 / T31)

Five unexported `newXxxCmd` / `runXxx` functions in a single file share a rigid subject-verb-object opener regardless of body complexity:

```go
// newCacheCmd assembles the `poplar cache` subcommand tree.
// newCacheEvictCmd assembles the `poplar cache evict` subcommand.
// newCacheVacuumCmd assembles the `poplar cache vacuum` subcommand.
```

```go
// runEvict opens each account (or one if scoped) and runs EvictByAge.
// Passing nil backend/tracker is safe — Evict only touches the bodies
// table and performs no backend I/O.

// runVacuum opens each account's database and runs VACUUM, reporting
// before/after file sizes.
```

A 3-line tree builder gets the same comment weight as a 25-line loop with account-scoped error handling.

### config_discover_folders.go:31–62 — C4 (T11 gerund-chorus within function)

Six consecutive error returns inside the `discover-folders` RunE closure all use a present-participle gerund as the context prefix:

```go
return fmt.Errorf("reading config: %w", err)
return fmt.Errorf("loading accounts: %w", err)
return fmt.Errorf("opening backend for account %q: %w", accounts[0].Name, err)
return fmt.Errorf("connecting backend for account %q: %w", accounts[0].Name, err)
return fmt.Errorf("listing folders: %w", err)
return fmt.Errorf("reading existing folder keys: %w", err)
```

Identical `"present-participle + noun: %w"` template. The choir reads machine-shaped.

### config_discover_folders.go:82–100 — C4 (T10b / T11)

`writeAtomically` makes five sequential filesystem calls; every error return follows the identical `"<gerund> temp file: %w"` template:

```go
return fmt.Errorf("creating temp file: %w", err)
return fmt.Errorf("writing temp file: %w", err)
return fmt.Errorf("syncing temp file: %w", err)
return fmt.Errorf("closing temp file: %w", err)
return fmt.Errorf("renaming temp file: %w", err)
```

The stack-unwound chain `"writing temp file: syncing temp file:"` collapses into noise.

---

## C5 — Naming tells

### cache.go:183 — C5 (T18)

```go
func parseEvictDuration(s string) (time.Duration, error) {
```

Name concatenates three concepts: `parse` + `Evict` + `Duration`. Reads like a docstring summary. `parseDuration` plus a comment about the `d`/`w` extension would be idiomatic; the `Evict` infix is redundant given the caller context (`newCacheEvictCmd`).

---

## C6 — Test boilerplate

### config_cmd_test.go and config_discover_folders_test.go — C6 (T9)

Test function names encode behavioral assertions (verb-object predicates):

```
TestConfigInitWritesTemplate          → "writes template" = verb-object assertion
TestConfigInitRefusesExisting         → "refuses existing" = verb-object assertion
TestConfigInitForceOverwrites         → "force overwrites" = verb-object assertion
TestConfigPathPrintsResolved          → "prints resolved" = verb-object assertion
TestConfigDiscoverFolders_DryRunShowsDiscoveredFolders → seven-word behavioral assertion
```

Names like `ShowsDiscoveredFolders` read as assertions the test is designed to prove, not labels for what is being tested.

### cache_test.go:13–20 — C6 (T24)

```go
func TestCacheStats_OutputFormat(t *testing.T) {
	// formatStatsLine is a pure helper used by the cache stats
	// subcommand. It takes a stats struct and returns the tab-aligned
	// row. Test it in isolation; integration with real accounts is
	// covered by manual smoke at install.
```

Four-sentence prose docstring inside a test function body explaining the testing strategy. The test name carries the documentation; the rationale reads as AI-generated narration.

---

## C7 — Error phrasing

### root.go:79–132 — C7 (T10b)

`runRoot` chains nine sequential `fmt.Errorf("noun phrase: %w", err)` returns in a ladder:

```go
return fmt.Errorf("load accounts: %w", err)
return fmt.Errorf("open backend: %w", err)
return fmt.Errorf("connect: %w", err)
return fmt.Errorf("load UI config: %w", err)
return fmt.Errorf("load cache config: %w", err)
return fmt.Errorf("open cache: %w", err)
return fmt.Errorf("start drainer: %w", err)
return fmt.Errorf("running poplar: %w", err)
```

Every step produces an identically-templated `"verb noun: %w"` string. `"running poplar: %w"` adds nothing beyond `p.Run()`'s own message.

### root.go:79, 97, 112 — C7 (T13)

```go
return fmt.Errorf("load accounts: %w", err)
return fmt.Errorf("load UI config: %w", err)
return fmt.Errorf("load cache config: %w", err)
```

No caller in `cmd/poplar/` ever calls `errors.Is`/`errors.As`; errors terminate in `main`'s `fmt.Fprintln`. `%w` reflexively exposes sentinel as package API surface where `%v` is correct.
