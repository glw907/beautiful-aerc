# Mail Client Search UX — Matrix Survey 2026-05-09

Prior-art survey of search functionality across the major-client
matrix, gathered for Pass 13.1 design (BACKLOG #38). Sources cited
inline.

Clients surveyed: Thunderbird, Apple Mail, Outlook, mutt/neomutt,
aerc, K-9 Mail, Geary, Evolution, alpine/pine. Plus Gmail and
Fastmail web for the operator-string reference.

Skipped: FairEmail, Spark, Nylas, Edison (niche or discontinued).

## Comparison table

| Client | Entry | Operator language | Scope | Result render | Index vs server | Persistence | Folder-local quick filter |
|---|---|---|---|---|---|---|---|
| Thunderbird | Quick Filter Bar (`F8`) + Global Search box (toolbar) | Quick Filter: buttons + free text scoped to fields. Global Search: plain terms + quotes; no native operators (addons add them) | Quick Filter: current folder. Global Search: all folders/accounts | Quick Filter narrows in place. Global Search opens a faceted results tab | Gloda local SQLite FTS index for Global; Quick Filter is in-memory | Saved Searches as virtual folders; results tab ephemeral | Yes (Quick Filter Bar) |
| Apple Mail | Unified search bar | Token UI (From, To, Subject, Date, Flagged, Has Attachment, Mailbox); no string syntax | Default current mailbox; token to switch to All Mailboxes | Narrows list in place; token breadcrumb row | Local Core Spotlight index | Smart Mailboxes are persistent; search ephemeral | Same bar, scope on current folder |
| Outlook | Unified search bar (`Ctrl+E` / `Alt+Q`) | `from:`, `to:`, `subject:`, `hasattachment:yes`, `received:>date`, `received:date..date`, `body:`, `cc:`, `size:` | Dropdown: Current Folder → Mailbox → All Mailboxes → All Outlook Items | Replaces message list; ribbon offers post-filters | Local Windows Search / Spotlight; Web uses Exchange server | Ephemeral; no native saved searches | Same bar, scope defaults to current folder |
| mutt / neomutt | `l` (limit), `/` (next-match), `\` (tag) | `~f`, `~t`, `~s`, `~b`, `~d`, `~r`, `~F`, `~N`, `~R`, `~A`, `~m`, `~n`, boolean `!`/`|`; POSIX regex | Folder-local only; cross-folder via notmuch/mairix | Live-narrows index pane; `l all` resets | In-memory; IMAP backend pushes `~b` to IMAP SEARCH if configured | Ephemeral; macros simulate persistence | `l` is the entire surface |
| aerc | `:search <terms>` / `:filter <terms>` | `-f`, `-t`, `-c`, `-d`, `-x`, `-X`, `-H header:value`, `-b`, `-a`; free terms = subject; `-r`/`-u`/`-e` for read state | Current folder only | `:search` highlights, `n`/`N` navigate; `:filter` hides non-matches | Backend-dependent: IMAP SEARCH or notmuch | Ephemeral | `:filter` vs `:search`; no separate cross-folder |
| K-9 Mail | Magnifier icon; account-level unified search | Free text; flag filters via UI | Local-first; "Search on server" appears after local results | Flat list replacing message list | Local SQLite cache; IMAP SEARCH for server extension | Ephemeral | None — same box at folder and account level |
| Geary | Search bar (`Ctrl+F`) | `from:name`, `is:read`, `is:unread`, `is:starred`; mostly free text | Cross-account, cross-folder by default | Flat "Search" pseudo-folder in sidebar | Local SQLite FTS5 index | Ephemeral | None — search is always cross-folder |
| Evolution | Toolbar search bar; Search menu for advanced/saved | UI-driven field filters; "Free Form Expression" with `f:`, `t:`, `s:` shortcuts; boolean AND/OR/NOT | Current folder by default; "All Accounts" via menu | Narrows list in place; vFolders are persistent saved searches | Local Camel index for headers; auto-escalates to IMAP SEARCH for body | Ephemeral search; vFolders persistent | Same bar, scope defaults to current folder |
| alpine / pine | `;` (Select) in folder index; `W` (WhereIs) for cursor jump | Field-by-field prompts (date, sender, subject, CC, body, status, line text); no `op:value` strings | Folder-local only | `;` marks messages; aggregate commands act on the set | IMAP SEARCH for body when connected | Ephemeral selection set | `;`/`W` is the entire surface |

### Web reference (operator completeness benchmark)

| | Gmail | Fastmail |
|---|---|---|
| Operators | `from:`, `to:`, `cc:`, `bcc:`, `subject:`, `label:`, `in:`, `is:`, `has:`, `filename:`, `before:`, `after:`, `older_than:`, `newer_than:`, `size:`, `larger:`, `smaller:`, `deliveredto:`, `list:` | `from:`, `to:`, `cc:`, `subject:`, `in:`, `flag:`, `has:`, `filetype:`, `attached:`, `fromin:`, `toin:`, `before:`, `after:`, `size:` |
| Server | Server-side only | Server-side (JMAP `Email/query`) |
| Saved search | Yes (filters) | Yes (sidebar) |

## Synthesis

**Convergence.** Every client distinguishes a fast folder-local
narrowing operation (mutt's `l`, Thunderbird's Quick Filter,
Outlook's current-folder scope, aerc's `:filter`, alpine's `;`)
from a slower cross-folder full-text operation, even when both
share a single widget. Local indexing is universal for GUI
clients (Gloda, Spotlight, Camel, SQLite-FTS); the TUI clients
that support body search (mutt, aerc) delegate to IMAP SEARCH
rather than indexing locally. Ephemeral results dominate. True
persistence (Thunderbird Saved Searches, Evolution vFolders,
Fastmail saved searches) is everywhere a minority path, buried
behind two or more actions.

**Divergence.** Operator language splits cleanly: web clients
(Gmail, Fastmail) and Outlook commit to `key:value` strings the
user types; GUI desktop clients (Apple Mail, Evolution, Geary)
favor token builders or field dropdowns producing no learnable
syntax; TUI clients (mutt, aerc, alpine) use command paradigms
(`~f`, `-f`, `;`) that compose well but are opaque without the
man page. Geary is the only client that makes all search
cross-folder by default and has no folder-local quick-filter at
all — a deliberate simplification that frustrated power users in
practice.

**Implications for poplar.** Modifier-free, single-pane,
vim-first constraints rule out token-builder UIs and dedicated
search panes. mutt's `l` and aerc's `:filter` are the natural
TUI ancestors, but both are folder-local only with no
cross-folder surface. Poplar's JMAP backend makes cross-folder
search cheap (one `Email/query`, no per-folder loop), an
opportunity none of the TUI ancestors exploit. The poplar-shaped
gap: a `key:value` operator string in a TUI command bar with
sensible scope-switching. aerc's flag soup comes closest but is
positional rather than composable; none of the surveyed TUIs
offer scope-switching (folder → account → all accounts) in a
single flow. The cache layer makes that scope-switch free for
us.

## Sources

- [Global Search | Thunderbird Help](https://support.mozilla.org/en-US/kb/global-search)
- [Quick Filter Toolbar | Thunderbird Help](https://support.mozilla.org/en-US/kb/quick-filter-toolbar)
- [Using Saved Searches | Thunderbird Help](https://support.mozilla.org/en-US/kb/using-saved-searches)
- [Search for emails in Mail on Mac | Apple Support](https://support.apple.com/guide/mail/search-for-emails-mlhlp1003/mac)
- [Mail Search on macOS Catalina | houdah blog](https://blog.houdah.com/2019/11/mail-search-on-macos-catalina/)
- [How to search in Outlook | Microsoft Support](https://support.microsoft.com/en-us/office/how-to-search-in-outlook-d824d1e9-a255-4c8a-8553-276fb895a8da)
- [Advanced Usage | NeoMutt](https://neomutt.org/guide/advancedusage)
- [Effective search with Mutt | Ruslan Osipov](https://rosipov.com/blog/effective-search-with-mutt/)
- [aerc-search(1) | Arch manual pages](https://man.archlinux.org/man/aerc-search.1.en)
- [Search on Server | K-9 Mail GitHub Issue #791](https://github.com/k9mail/k-9/issues/791)
- [Apps/Geary/FullTextSearchStrategy | GNOME Wiki Archive](https://wiki.gnome.org/Apps/Geary/FullTextSearchStrategy)
- [Searching Mail | Evolution GNOME Help](https://help.gnome.org/users/evolution/stable/mail-searching.html.en)
- [Using Search folders | Evolution GNOME Help](https://help.gnome.org/users/evolution/stable/mail-search-folders.html.en)
- [alpine(1) | Debian Manpages](https://manpages.debian.org/testing/alpine/alpine.1.en.html)
- [Refine searches in Gmail | Gmail Help](https://support.google.com/mail/answer/7190)
- [Searching your mail | Fastmail Help](https://www.fastmail.help/hc/en-us/articles/360060591213)
