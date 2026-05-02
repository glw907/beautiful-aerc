# Poplar Configuration — v1 Design

**Date:** 2026-05-02
**Status:** approved design, awaiting implementation plan.

This work was originally planned as Pass 10 (Config polish) but
the user has elected to land it before Pass 8.1 — the provider
registry and account-config touchpoints overlap heavily, and the
config rename is cheaper to do before adding more presets.

## Goal

Land the cleanest, most discoverable configuration story for poplar
1.0 before anyone has muscle memory. The user-facing surface is a
single self-documenting TOML file that ships with sensible defaults
and exhaustive inline help — first-time users should never need to
leave the file to configure poplar.

A first-run setup wizard is **post-1.0** and explicitly out of scope.

## Scope

In:

- Rename `accounts.toml` → `config.toml`
- Rename top-level key `backend` → `provider`
- Provider preset list expansion (11 entries + 2 fallbacks)
- Single `email` field (no `domain` + `username` split)
- Password handling: `password = "$VAR"` or `password-cmd = "..."`
- Self-documenting template generated on first run
- OS-specific notes (Linux, macOS, Windows) inline in the template
- Validation tiers (syntax, schema, connectivity)
- Friendly pre-launch error messages
- `poplar config check` subcommand
- macOS path discipline (use `~/.config/`, not Apple's
  Application Support)

Out:

- Setup wizard (post-1.0)
- SMTP / outgoing-mail preset fields (Pass 9 lands compose; SMTP
  defaults will be added then)
- Per-account UI overrides (existing global `[ui]` block stays)
- ProtonMail Bridge auto-launch (manual install, manual start)
- OAuth interactive setup flow (Pass 8.1 wires Gmail/Outlook
  refresh-token handling separately; this spec only specifies
  the config surface)

## Architectural decisions (pinned)

- **One config file.** Both `[[account]]` blocks and `[ui]` table
  live in `config.toml`. (Already true; only the filename
  changes.)
- **Provider-first config model.** Users pick a `provider` preset;
  fallbacks `imap` and `jmap` cover unlisted servers.
- **No backwards compatibility shims.** Pre-1.0 churn is free; the
  rename is a hard cutover. Old `accounts.toml` files trigger an
  error pointing at the new location.

## TOML shape

### File location

| OS                  | Path                                  |
|---------------------|---------------------------------------|
| Linux (Ubuntu/Mint) | `~/.config/poplar/config.toml`        |
| macOS               | `~/.config/poplar/config.toml`        |
| Windows             | `%APPDATA%\poplar\config.toml`        |

poplar overrides Go's `os.UserConfigDir()` on macOS to use
`~/.config/poplar/`. Apple's `~/Library/Application Support/` is
reserved for GUI apps; CLI tools (`pass`, `nvim`, `tmux`, `git`)
universally use `~/.config/`.

### Top-level structures

```toml
[[account]]   # one block per mail account; multiple allowed
[ui]          # one block of UI preferences for all accounts
```

### Account fields

| Field           | Required                  | Default          |
|-----------------|---------------------------|------------------|
| `provider`      | yes                       | —                |
| `email`         | yes                       | —                |
| `password`      | yes (or `password-cmd`)   | —                |
| `password-cmd`  | yes (or `password`)       | —                |
| `name`          | no                        | value of `email` |
| `host`          | only for `imap`/`jmap`    | preset host      |
| `port`          | no                        | 993 (IMAP)       |
| `starttls`      | no                        | false            |
| `insecure-tls`  | no                        | false            |
| `auth`          | no                        | "plain" (IMAP)   |
| `copy-to`       | no                        | none             |
| `folders-sort`  | no                        | server order     |

`password` and `password-cmd` are mutually exclusive — setting both
is a config error.

### Provider preset list (v1)

| Preset        | Backend | Notes                                |
|---------------|---------|--------------------------------------|
| `fastmail`    | jmap    | Fastmail's modern API                |
| `gmail`       | imap    | OAuth (XOAUTH2)                      |
| `icloud`      | imap    | App-specific password                |
| `yahoo`       | imap    | App-specific password                |
| `zoho`        | imap    | App-specific password                |
| `outlook`     | imap    | OAuth (Microsoft)                    |
| `mailbox-org` | imap    | App password                         |
| `posteo`      | imap    | Privacy-focused EU provider          |
| `runbox`      | imap    | Privacy-focused                      |
| `gmx`         | imap    | EU consumer                          |
| `protonmail`  | imap    | Local ProtonMail Bridge; insecure-tls|

Plus two fallbacks: `imap` (any IMAP server with explicit
host/port) and `jmap` (any JMAP server with explicit `source`
URL).

### `Provider` struct (Go)

The `internal/config.Provider` struct gains one field; otherwise
unchanged from Pass 8:

```go
type Provider struct {
    Name        string
    Backend     string  // "imap" or "jmap"
    Host        string  // IMAP only
    Port        int
    StartTLS    bool
    InsecureTLS bool    // NEW — true only for protonmail
    URL         string  // JMAP only
    AuthHint    string  // "app-password" | "bearer" | "xoauth2"
    HelpURL     string
    GmailQuirks bool    // X-GM-EXT, Trash precondition
}
```

Account-level `insecure-tls` still overrides the preset value when
explicitly set.

## Self-documenting template

On first launch with no `config.toml`, poplar:

1. Creates the parent directory if needed.
2. Writes the template to `config.toml`.
3. Prints to stderr:
   ```
   created ~/.config/poplar/config.toml — edit it and run poplar again
   ```
4. Exits with status 78 (`EX_CONFIG`).

The template is **emitted by code** (a `templateConfig() string`
function in `internal/config/`), not checked in as a static file.
This keeps it under code review, lets us interpolate the version,
and lets `poplar config init --force` regenerate it for users who
want a clean reference.

### Template structure

```
HEADER
   File purpose, location per OS, restart-after-edit note,
   TOML structure overview.

ACCOUNTS  ════════════════════
   Provider presets (one-line table)
   Fallback presets (imap / jmap)

   Setup notes (subsection)
      App-password providers
      OAuth providers (gmail, outlook)
      ProtonMail (Bridge install per OS)

   Secrets (subsection)
      $VAR pattern
      password-cmd pattern (recommended)
      Examples grouped by OS:
        Linux (Ubuntu/Mint/Debian)
        macOS
        Windows (PowerShell SecretManagement)
      First-time secret-manager pointer per OS

   Hosted-provider example (uncommented; the user's starting point)

   Optional account fields (commented; defaults shown)

   Self-hosted IMAP example (commented)

   ProtonMail via Bridge example (commented)

UI  ════════════════════
   theme, undo_seconds, retention_days options
```

### Voice and formatting

- 72-column wrap (matches poplar's body cap).
- Crisp but friendly. Paragraphs of explanation where the concept
  warrants it; not just terse declarations.
- Don't over-assume technical knowledge. Define "preset,"
  "implicit TLS," "SASL," "app password" inline the first time
  they appear.
- Section banners: `# ───…` × 70 (full-width) for top-level;
  `# ──` × N matching heading length for subsections.
- Two-space indent for option names inside comment blocks;
  six-space indent for descriptions.
- Tables (one option name per row, short description per row)
  must keep description to a single line. If a description needs
  multiple sentences, move it to a labeled prose subsection
  below — never wrap continuation lines into a column-padded
  gutter.
- Examples use 4- or 8-space indent under their heading; right-
  aligned `(annotation)` columns work fine for short one-line
  commands.

## Password handling

### Two patterns

```toml
# Pattern 1 — environment variable
password = "$ENV_VAR_NAME"

# Pattern 2 — shell command (recommended)
password-cmd = "<shell command>"
```

### Behavior

- `password = "$VAR"` resolves at startup. If `$VAR` is unset,
  config validation fails with a friendly error.
- `password-cmd = "..."` runs once at first Connect on each
  Backend. The result is cached in memory for the lifetime of
  that Backend (i.e., until the process exits or the account is
  reloaded). Reconnects within the session reuse the cached
  value.
- If `password-cmd` fails during connect (non-zero exit, or
  command not found), the failure surfaces via the error banner;
  the reconnect retry loop continues exponentially.

### Per-OS guidance in the template

The template's Secrets section documents specific commands for
each OS — see formatting notes above.

## Validation

### Three tiers

1. **Syntax** — TOML parser errors (`BurntSushi/toml`) surface the
   parser's line/column verbatim.
2. **Schema** — runs in `config.LoadConfig` before the TUI
   launches. Catches: unknown provider, missing required field,
   conflicting `password` + `password-cmd`, unset env var, etc.
3. **Connectivity** — auth, DNS, TLS, server errors. Surfaces via
   the in-TUI error banner (ADR-0073). Other accounts still load.

### Pre-launch error format

```
poplar: config error in ~/.config/poplar/config.toml

  account "personal": unknown provider "yahho"

  Known providers:
    fastmail, gmail, icloud, yahoo, zoho, outlook,
    mailbox-org, posteo, runbox, gmx, protonmail

  Did you mean: yahoo?
```

Style:
- Header line names the file.
- Two-space indent for the error.
- Account name (or `account[N]` if no `name` field) for context.
- Known set listed when the error is "unknown X".
- "Did you mean" suggestion when there's a clear closest-match
  (Levenshtein distance ≤ 2).
- Blank line before final hint.

### Config-file lookup precedence

On startup, poplar resolves the config path in this order:

1. `--config <path>` CLI flag, if given.
2. `$POPLAR_CONFIG` env var, if set.
3. The OS-default path (`~/.config/poplar/config.toml` on
   Linux/macOS, `%APPDATA%\poplar\config.toml` on Windows).

If the path resolves to (3) and the file doesn't exist:

- If `~/.config/poplar/accounts.toml` exists, error: "Found …
  accounts.toml. poplar 1.0 reads … config.toml — please rename
  the file." (Pre-1.0 carryover only; can drop after 1.0 ships.)
- Otherwise, write the template and print the create-message.

If the path resolves to (1) or (2) and the file doesn't exist,
error — the user explicitly named a file, so silently creating
something would be surprising.

### Specific error cases

- **No config file** (default path) → write template, print
  create-message, exit non-zero (status 78, `EX_CONFIG`) so
  shell wrappers know the user still has work to do.
- **Template untouched (no uncommented accounts)** → "Config
  file at ~/.config/poplar/config.toml has no [[account]]
  blocks. Uncomment and edit one of the example blocks to get
  started." Exit non-zero.
- **Old `accounts.toml` file present** (and no `config.toml`) →
  "Found ~/.config/poplar/accounts.toml. poplar 1.0 reads
  ~/.config/poplar/config.toml — please rename the file." Exit
  78. (Pre-1.0 only; can drop after 1.0 ships.)
- **Unknown provider** → as above.
- **Missing required field** → "account \"personal\"
  (provider = \"imap\"): host is required for IMAP accounts.
  Use a preset, or set host = \"...\"."
- **Conflicting password fields** → "account \"personal\":
  both password and password-cmd are set. Use one."
- **Env var not set** → "account \"personal\" password:
  $FASTMAIL_TOKEN is not set in the environment."
- **password-cmd failed** (Connect time, surfaces via banner) →
  "⚠ connect: password-cmd \"op read ...\" exited with code 1:
  &lt;stderr&gt;"

### Self-signed-cert hint

When TLS verification fails AND the host looks self-hosted (RFC
1918 IP, `.local` mDNS, `127.x`), the runtime banner appends a
hint:

```
⚠ connect: tls: cannot validate cert for 192.168.1.10
            (set insecure-tls = true if self-signed)
```

For hosted providers, no hint — a TLS failure there means
something is genuinely wrong.

## `poplar config` subcommands

| Command                       | Behavior                              |
|-------------------------------|---------------------------------------|
| `poplar config init`          | Write template; refuse if file exists |
| `poplar config init --force`  | Write template; overwrite             |
| `poplar config check`         | Validate file + Connect each account; report per-account `OK` or `error: ...`; exit non-zero on any failure |
| `poplar config path`          | Print resolved config path (for scripts/CI) |

`poplar config check` is the smoke-test entry point for users
debugging dotfile changes; it doesn't launch the TUI.

## Code touchpoints

### Files modified

| File                                         | Change                                    |
|----------------------------------------------|-------------------------------------------|
| `internal/config/account.go`                 | (no struct change)                        |
| `internal/config/accounts.go`                | Read `provider` key; emit clean errors    |
| `internal/config/providers.go`               | Add 6 presets; add `InsecureTLS`          |
| `internal/config/template.go` (NEW)          | `templateConfig() string` returns the template |
| `internal/config/loader.go` (NEW)            | `LoadConfig()` orchestrates first-run + validation |
| `internal/config/loader_test.go` (NEW)       | Validation table tests                    |
| `cmd/poplar/root.go`                         | Call `LoadConfig()`; handle first-run     |
| `cmd/poplar/config_cmd.go` (NEW)             | `poplar config init/check/path`           |
| `cmd/poplar/backend.go`                      | Already dispatches on canonical backend   |

### Files renamed

The runtime config path lookup moves from
`~/.config/poplar/accounts.toml` to `~/.config/poplar/config.toml`.
Test fixtures and any references in `docs/` updated.

### Files removed

None. (`internal/config/accounts.go` keeps its name; only the
file it reads at runtime changes.)

## Testing

- **Template generation** — golden file test that
  `templateConfig()` matches a checked-in expected output, so
  format drift is caught in review.
- **First-run flow** — table test covering: (a) no config →
  template written; (b) old `accounts.toml` → error; (c) empty
  `[[account]]` array → error; (d) valid config → load.
- **Validation errors** — table tests covering each error case
  in the spec, asserting the exact message format.
- **Levenshtein "did you mean"** — unit test covering close
  matches (1-2 edit distance) and non-matches (≥3 edits return
  no suggestion).
- **`poplar config check`** — integration test that runs the
  subcommand against a fixture config and asserts exit code +
  output format.
- **OS-path discipline** — unit test that the resolved config
  path is `~/.config/poplar/config.toml` on `runtime.GOOS ==
  "darwin"`, not Application Support.

## Open follow-ups

- **OAuth flow specification** — Pass 8.1 covers the actual
  Gmail/Outlook XOAUTH2 plumbing. The config surface here just
  declares `auth = "xoauth2"` as a recognized value; the
  refresh-token handling lives in `internal/mailauth/` and
  `internal/mailimap/`.
- **SMTP/outgoing presets** — Pass 9 (compose) will extend each
  Provider with SMTP host/port/StartTLS so users don't supply
  `outgoing` separately. Out of scope here.
- **Config reload without restart** — possible post-1.0
  enhancement; v1 reads at startup only.

## Decision references

- ADR-0053 — UI config in same file as accounts (preserved).
- ADR-0075 — Direct-on-libraries mail stack (no aerc fork).
- ADR-0098 — Provider registry pattern (extended here).
- ADR-0099 — Two-connection IMAP (cached `password-cmd` ties
  into the reconnect loop).
- ADR-0073 — Error banner (used for runtime config errors).
