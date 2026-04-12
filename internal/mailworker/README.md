# mailworker

IMAP and JMAP workers forked from aerc
(`git.sr.ht/~rjarry/aerc`) on 2026-04-09.

## Why a fork

Aerc does not maintain a stable library API — internal packages
change without notice. A clean fork with cherry-pick upstream
tracking is more stable than chasing `go get -u` breakage. See
`docs/poplar/decisions/0002-clean-fork-over-direct-import.md`.

## Subpackages

| Package | Purpose |
|---------|---------|
| `worker/` | Worker event loop, IMAP and JMAP implementations |
| `worker/imap/` | Gmail IMAP worker |
| `worker/jmap/` | Fastmail JMAP worker |
| `models/` | Shared aerc data types (MessageInfo, Folder, etc.) |
| `auth/` | OAuth bearer-token handling |
| `keepalive/` | TCP keepalive tuning |
| `xdg/` | XDG base directory paths |
| `log/` | Structured logging used by the workers |
| `parse/` | RFC 2822 header parsing |
| `rfc822/` | Message body structure parsing |
| `lib/` | Worker-internal utilities |

Aerc's original `lib/` grab-bag was split into `auth`, `keepalive`,
`xdg`, `log`, and `parse` during the fork. See
`docs/poplar/decisions/0008-split-aerc-lib-into-focused-packages.md`.

## Upstream tracking

When cherry-picking upstream protocol fixes, preserve the aerc
commit hash in the commit message so the provenance stays visible
in `git log`. Every `.go` file in this tree already carries a
top-of-file `// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License`
comment; keep that comment when editing.

## LICENSE

Aerc is MIT-licensed. The original LICENSE file from aerc is
preserved alongside this README.
