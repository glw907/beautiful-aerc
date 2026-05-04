# Human-voice audit — `internal/config/`

**Tally:** C1=4 (T2=3, T5=1), C4=4 (T3/T31=2, T7=1, T17=2), C5=2 (T18=2), C6=3 (T9=2, T24=1), C7=2 (T10b=1, T13=1), C8=1 (T19=1)

Total: 17 findings.

---

## C1 — Comment rot

### accounts.go:196–198 — C1 (T2)

```go
// knownProvidersList returns a sorted, comma-separated list of all
// recognized provider names (presets + bare "imap"/"jmap").
func knownProvidersList() string {
```

Unexported helper. Name + signature say everything; sorted/comma-separated is observable from the four-line body.

### accounts.go:208–211 — C1 (T2)

```go
// isShellName reports whether s is a valid shell variable name:
// starts with a letter or underscore, followed by letters, digits,
// or underscores only.
func isShellName(s string) bool {
```

Second sentence copies the body's POSIX rule; one sentence (`"reports whether s is a valid POSIX shell variable name."`) suffices.

### diderror.go:5–7 — C1 (T2)

```go
// suggestProvider returns the closest known-provider name to s
// within Levenshtein distance 2, or "" if no close match exists.
// Includes the imap/jmap fallbacks in the search space.
func suggestProvider(s string) string {
```

Threshold and fallback set both visible in the 10-line body.

### template.go:5–9 — C1 (T5)

```go
// Template returns poplar's self-documenting config.toml template.
// poplar writes this to disk on first launch when no config exists.
//
// The output is intentionally checked against a golden file so any
// formatting drift surfaces in code review.
func Template() string {
```

Third sentence is task-framing — testing process belongs in `template_test.go` or a commit message.

---

## C4 — Uniform verbosity

### accounts.go:176–193 + cache.go:80–82 + ui.go:108–110 — C4 (T3 / T31)

```go
// resolveEnv replaces a leading "$VAR" with os.Getenv("VAR"). The
// only supported form is the bare $VAR token; anything else is
// returned unchanged so passwords containing a literal "$" still
// work. Empty env returns an error so the user gets a clear
// failure on misconfiguration.
func resolveEnv(s string) (string, error) {

// parseSize parses a size string with optional KB/MB/GB/TB suffix
// (1024-based). An empty string returns 0. Negative values error.
func parseSize(s string) (int64, error) {

// LoadUI reads the [ui] table from an config.toml file and returns
// a UIConfig. A missing file is an error; a missing [ui] section
// returns DefaultUIConfig().
func LoadUI(path string) (UIConfig, error) {
```

Two unexported helpers and one exported parser get indistinguishable doc weight (two sentences of similar length). Complexity differs by an order of magnitude.

### providers.go:80 + cache.go:26 + loader.go:52 — C4 (T7)

```go
// LookupProvider returns the Provider for key and true if known.
// DefaultCacheConfig returns the defaults applied when [cache] is
// missing from config.toml. Currently 2GB body-cache cap and 2GB
// attachment-cache cap.
// ErrFirstRun is returned by Load when the default config path
// did not exist and a fresh template was written.
```

Every symbol in the loader/registry surface opens with `Symbol does/returns X` regardless of how much the contract requires explanation.

### account.go:27–61 — C4 (T17)

```go
// PasswordCmd is a shell command whose stdout becomes the
// password. Resolved at first Connect (not at config-load time)
// so secret-manager prompts surface near the action that needs
// them. Mutually exclusive with Password.
PasswordCmd string

// Email is the user's address. May be empty for backends that
// auto-discover (JMAP session). Used as SASL username for IMAP.
Email string
```

Hedged narrative: "surface near the action that needs them" is ADR voice. One terse line suffices: `// deferred to first Connect; mutually exclusive with Password.`

### ui.go:29–43 — C4 (T17 / T31)

```go
// TrashRetentionDays is the per-session sweep cutoff for Trash. 0 disables (default).
// Clamped to [0, 365] on parse.
TrashRetentionDays int

// SpamRetentionDays is the per-session sweep cutoff for Spam. 0 disables (default).
// Clamped to [0, 365] on parse.
SpamRetentionDays int
```

Word-for-word identical except the noun.

---

## C5 — Naming tells

### providers.go:81 + cache.go:29 + writer.go:110 — C5 (T18)

```go
func LookupProvider(key string) (Provider, bool) {
func DefaultCacheConfig() CacheConfig {
func ExistingFolderKeys(data []byte) (map[string]bool, error) {
```

`LookupProvider` — `Providers[key]` is idiomatic. `DefaultCacheConfig` — `CacheDefaults()` or unexported `defaultCache`. `ExistingFolderKeys` — `FolderKeys(data)` suffices; "Existing" is a pre-modifier the signature already disambiguates.

### path.go:11–14 — C5 (T18)

```go
// ExpandHome turns a leading "~" into the user's home directory.
// "~" alone resolves to $HOME; "~/x" resolves to $HOME/x. Other
// paths pass through. The empty string also passes through (callers
// supply their own default).
func ExpandHome(p string) (string, error) {
```

The pure-passthrough contract for non-tilde paths is itself evidence the name over-describes — "Expand" implies transformation; passthrough isn't expansion.

---

## C6 — Test boilerplate

### ui_test.go (TestLoadUI) — C6 (T9)

```go
{name: "missing [ui] section uses defaults", ...},
{name: "empty [ui] section uses defaults", ...},
```

Sentence-form predicate `name:` fields. Compare `"default when missing"` in `TestLoadUI_Icons` (correctly noun-phrase) with `"missing [ui] section uses defaults"` here.

### ui_test.go:190–213 (TestLoadUI_UndoSeconds) — C6 (T9)

```go
{"below floor clamps to 2", "[ui]\nundo_seconds = 0\n", 2},
{"above ceiling clamps to 30", "[ui]\nundo_seconds = 99\n", 30},
{"negative clamps to 2", "[ui]\nundo_seconds = -5\n", 2},
```

Verb "clamps" encodes assertion. Noun forms: `"below floor"`, `"above ceiling"`, `"negative"`.

### accounts_test.go and others — C6 (T24)

```go
t.Fatalf("expected error containing %q, got nil", tt.wantErr)
t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
```

Identical assertion phrasing across `TestParseAccounts`, `TestResolveEnv`, `TestLoadUI`, `TestLoadUIMissingFile` — four test functions in three files use the template verbatim.

---

## C7 — Error phrasing

### accounts.go + cache.go + ui.go (loader pairs) — C7 (T10b)

Six errors across three files follow `"<gerund> <noun> config: %w"`:

```go
fmt.Errorf("reading accounts config: %w", err)
fmt.Errorf("parsing accounts config: %w", err)
fmt.Errorf("reading cache config: %w", err)
fmt.Errorf("parsing cache config: %w", err)
fmt.Errorf("reading ui config: %w", err)
fmt.Errorf("parsing ui config: %w", err)
```

Cross-file chorus reads as fill-in-the-blank.

### Same six errors — C7 (T13)

Callers (`cmd/poplar/`) check `errors.Is(err, ErrFirstRun)` and `errors.Is(err, ErrOldAccountsToml)` only — never `fs.PathError` or TOML parse errors. The `%w` exposes those as package API for no caller benefit. Should be `%v`.

---

## C8 — Structural symmetry

### internal/config/ — C8 (T19)

10 source files for a single responsibility (config parsing):

```
account.go      — AccountConfig struct only
accounts.go     — parsing for AccountConfig
cache.go        — CacheConfig struct + parsing
ui.go           — UIConfig + FolderConfig + parsing
loader.go       — Resolve, Load, sentinel errors
providers.go    — Provider + Providers + LookupProvider
template.go     — Template()
writer.go       — folder-section render + ExistingFolderKeys
diderror.go     — suggestProvider + levenshtein
path.go         — ExpandHome (8 lines)
```

`account.go` + `accounts.go` split with zero coupling reason. `path.go` holds one tiny function. `diderror.go` is named after the UX pattern. `template.go` + `writer.go` could collapse. Reflexive `<thing>.go` per concept skeleton.
