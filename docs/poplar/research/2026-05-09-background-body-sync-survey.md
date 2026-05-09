# Background Body-Sync — Source Survey 2026-05-09

Survey of how mainstream email clients implement background mail
body sync — throttling, resumption, ordering, user-activity
gating, connection-state interaction, new-mail priority. Gathered
for Pass 13 design (background body sync substrate beneath the
search work in #38).

Open-source clients only — Apple Mail and Outlook were skipped
because their behavior is not externally inspectable. Source
files cited where visible.

## Per-client findings

### Thunderbird

Source: `mailnews/imap/src/nsAutoSyncManager.cpp`,
`nsAutoSyncState.cpp`.

- **Throttle.** `nsAutoSyncManager` runs on a
  `TYPE_REPEATING_SLACK` timer keyed by `kTimerIntervalInMs`.
  `GetNextGroupOfMessages()` enforces `kDefaultGroupSize = 2 MB`
  per batch. A `mUpdateInProgress` flag serializes folder
  updates so no two folders on the same server download
  simultaneously. `kFirstPassMessageSize` defers large messages
  until smaller ones in the batch finish — a two-pass split.
- **Resume.** `nsAutoSyncState` is a state machine
  (`stReadyToDownload`, `stDownloadInProgress`, `stCompletedIdle`,
  `stUpdateNeeded`). UID watermarks via `GetLastSyncTime()` /
  `GetLastUpdateTime()` skip recently-updated folders (10 min
  status / 1 hour discovery defaults). `TryCurrentGroupAgain`
  retries failed groups up to `kGroupRetryCount`.
- **Order.** Folder priority: Inbox → Drafts → regular →
  Trash. Within a folder: newest-first, then smallest-first
  among same-age. Large messages deferred to second pass.
- **User-activity.** Uses `nsIUserIdleService` with
  `kIdleTimeInSec`. `Pause()` on sleep / hibernation / offline;
  `Resume()` on wake. Listens for `mail:appIdle`; ignores idle
  events until `mail-startup-done` fires.
- **Connection.** Observes
  `NS_IOSERVICE_GOING_OFFLINE_TOPIC`; `WeAreOffline()` blocks
  idle processing. Auto-resumes on reconnect.
- **New mail.** `OnFolderHasPendingMsgs()` inserts at queue
  front (`InsertObjectAt(obj, 0)`). Inbox is also always
  sorted first.

### K-9 Mail / Thunderbird Android

Source: `backend/imap/src/main/java/com/fsck/k9/backend/imap/ImapSync.kt`,
`UidReverseComparator`.

- **Throttle.** No explicit timer. Per-folder sync runs in
  three sequential phases: headers for the visible window,
  full body fetch for "small" messages
  (`< maximumAutoDownloadMessageSize`), partial text-parts-only
  fetch for "large" messages. Per-account thread serializes
  folders; no inter-message delay.
- **Resume.** Uses `EXTRA_HIGHEST_KNOWN_UID` persisted in
  folder metadata. `isOldMessage()` compares each UID against
  the watermark to distinguish new-since-last-sync from
  backfill. UIDVALIDITY change → full cache clear and
  watermark reset. Mid-folder interrupts restart from the top
  of the visible window on the next poll.
- **Order.** `UidReverseComparator` sorts unsynced messages
  descending by UID — newest-first. Visible window limited to
  N most-recent server messages; no attempt to backfill older
  history beyond `defaultVisibleLimit`.
- **User-activity.** None in the sync path.
  `MessagingController` defers to Android's WorkManager /
  JobScheduler; battery / doze policy is the de facto signal.
- **Connection.** Failed sync is logged, `setLastChecked`
  stamped, next scheduled poll retries. No event-driven
  reconnect inside the sync path.
- **New mail.** No queue jumping. IMAP IDLE
  (`ImapFolderPusher`) triggers an immediate per-folder sync,
  effectively prioritizing the affected folder over the poll
  schedule.

### Geary

Source: `imap-engine-email-prefetcher.vala`,
`imap-engine-minimal-folder.vala`. Bug
[#130](https://gitlab.gnome.org/GNOME/geary/-/issues/130) for
behavior under load.

- **Throttle.** Dedicated `imap-engine-email-prefetcher`
  component (`do_prefetch_email_async`) runs autonomously.
  Fetches UID ranges in batches without an explicit
  client-side rate limit; the server's `[THROTTLED]` tag is
  the actual gate. Bug #130 documents 20+ GB runaway sync
  when the prefetch-window setting was ignored.
- **Resume.** UID range tracking. The prefetcher records
  high-water UID and continues from there. The time-based
  prefetch window (`prefetch = 2 weeks`) historically wasn't
  respected — Geary fetched the entire mailbox regardless.
- **Order.** Most-recent first within batch, then steps
  backward through history.
- **User-activity.** None visible. Background prefetching
  proceeds independently of UI focus.
- **Connection.** Sessions decoupled from folder open;
  engine creates sessions when server reachable, with
  implicit retry on reconnect.
- **New mail.** IMAP IDLE triggers a `normalize_folder` pass
  that processes new UIDs before continuing prefetch. New
  mail jumps ahead of backfill in the normalize pass.

### Evolution

Source: `camel-imapx-server.c`,
`camel-imapx-conn-manager.c`, `CamelIMAPXSettings`.

- **Throttle.** Connection-pool design:
  `camel-imapx-conn-manager` allows configurable concurrent
  IMAP connections per store, enabling parallel folder access
  and body fetching. Large messages fetched via partial FETCH
  byte ranges (`use-multi-fetch`).
- **Resume.** Local cache (`~/.cache/evolution`). On
  reconnect, IMAPX re-queries via CONDSTORE / HIGHESTMODSEQ
  where available, full UID FETCH otherwise. IDLE push for
  incremental updates. No explicit crash-recovery watermark
  documented.
- **Order.** Configurable via `fetch-order` property.
- **User-activity.** None. IDLE-driven, not poll-driven —
  no polling cycle to gate. Folder selection caches the
  folder's messages on UI action.
- **Connection.** `camel-imapx-conn-manager` reconnects
  transparently. IDLE re-established via the same manager.
- **New mail.** IDLE push immediately triggers a folder
  update. No explicit queue jumping; push updates are
  synchronous and inherently prioritized.

### aerc

Source: `worker/imap/fetch.go`, `worker/jmap/fetch.go`,
`worker/jmap/push.go`.

- **Throttle.** No background prefetch. Strictly demand-
  driven: bodies fetched only on user request
  (`handleFetchMessageBodyPart`, `handleFetchFullMessages`).
- **Resume.** Disk blob cache (`~/.cache/aerc/<account>/blobs/`)
  via levelDB survives restarts. JMAP worker replays state
  changes on reconnect via `Email/changes` with persisted
  `emailState` token. IMAP has no comparable delta — re-fetches
  on demand.
- **Order.** N/A — user-driven only.
- **User-activity.** N/A — no background work to pause.
- **Connection.** JMAP SSE (`push.EventSource`) on `Listen()`
  error sleeps 5s and retries. IMAP IDLE via go-imap
  background goroutine. Both reconnect automatically.
- **New mail.** JMAP `handleChange` on server push fetches
  delta via batched `Email/changes` / `Email/get`, marks new
  with `RecentFlag`, posts `MessageInfo` immediately. IMAP
  IDLE EXISTS untagged response triggers re-fetch. No queue
  to jump.

### Fastmail offline desktop client (2024)

Source: Advent 2024 blog posts (architecture, mail storage,
sync). Closed source — the blog is the spec.

- **Throttle.** Three-stage paging: IDs as placeholders →
  metadata/headers in batches → bodies, but only proactively
  for "pinned/recent" messages; older bodies fetched on demand.
  No specific batch sizes or timer intervals published.
- **Resume.** JMAP `modseq`-based state tokens (`/changes`).
  On restart, replays from last persisted state token rather
  than re-downloading. Offline writes tracked as a log of
  patches; reconnect syncs the log with last-write-wins
  patch-merge.
- **Order.** Recency-first for metadata. Older bodies
  fetched only on explicit demand.
- **User-activity.** Not described. Sync delegated to a
  shared / service worker; inherits browser background-tab
  throttling.
- **Connection.** JMAP `/changes` + server push (SSE or
  WebSocket) handle reconnect automatically. Browser
  online/offline APIs for queue replay.
- **New mail.** Server push delivers new mail via the JMAP
  push channel; cache processes the change immediately,
  updating UI. No queue jumping needed because bodies are
  demand-fetched anyway.

## Comparison table

| | Throttle | Resume | Order | User-activity | Reconnect | New-mail priority |
|---|---|---|---|---|---|---|
| Thunderbird | 2 MB / batch, idle-gated timer, per-server serialization | UID watermark + state machine + retry | Newest-first, smallest-first; Inbox priority | `nsIUserIdleService` — explicit pause/resume | Notification → auto-resume | Queue-front insertion |
| K-9 | None client-side; per-account thread serializes | `EXTRA_HIGHEST_KNOWN_UID` watermark; restart-from-top on interrupt | Newest-first | None; Android doze policy | Retry on next poll | IMAP IDLE triggers folder sync |
| Geary | None client-side; server `[THROTTLED]` is gate | UID range watermark | Newest-first within batch | None | Session recreated | IDLE → normalize before backfill |
| Evolution | Connection pool; chunked large-msg fetch | CONDSTORE / HIGHESTMODSEQ or full UID scan | Configurable (`fetch-order`) | None (push-driven) | Connection manager auto-reconnects | IDLE push triggers immediate update |
| aerc | None — demand-only | Disk blob cache; JMAP state token | User-driven | N/A | JMAP 5s retry; IMAP IDLE | Push delivers immediately |
| Fastmail | Staged: IDs → headers → bodies (recent only proactive) | JMAP `modseq` state tokens | Recency-first; older on demand | Browser tab throttling | JMAP `/changes` on reconnect | Server push (SSE/WebSocket) |

## Synthesis

**Newest-first is unanimous.** Every client that does background
body sync fetches in descending UID order. K-9, Geary,
Thunderbird, Fastmail all converge here. Evolution exposes a
toggle but defaults to newest. The pattern is universal because
it matches user attention.

**Watermark-resumed is unanimous.** Every client persists some
form of high-water mark — UID, JMAP `modseq`, levelDB blob index
— so an interrupted sync resumes without re-fetching. K-9's
`EXTRA_HIGHEST_KNOWN_UID` is the simplest. Poplar's
implicit-via-SQL approach (`LEFT JOIN bodies WHERE bytes IS
NULL`) is functionally equivalent without a separate column,
and self-heals on cache eviction.

**Thunderbird stands alone on user-activity gating.**
`nsIUserIdleService` → explicit pause/resume is the only
in-process awareness of human attention in the survey. Geary,
K-9, Evolution have nothing equivalent. For a TUI client where
user keystrokes share the IMAP cmd connection with backfill —
not the case for the GUI peers — Thunderbird's pattern transfers
more directly to poplar than to any of the others.

**Throttle shapes diverge sharply.** Thunderbird's 2 MB
per-batch with timer-slack is the principled implementation.
K-9 has no client-side rate limit. Geary has no client-side
limit and got bug-reported for runaway sync. aerc and Fastmail
sidestep the problem by demand-fetching. The Thunderbird shape
(batch ceiling + idle gate + connection serialization) is the
strongest reference for a poplar-shaped backfill.

**IDLE-priority new mail is universal.** Every client that has
push (everyone except aerc-IMAP) routes new mail outside the
backfill schedule via IDLE / SSE / WebSocket. Poplar already has
this via `pumpUpdatesCmd`; new mail naturally jumps the queue
because newest-first ordering plus push integration converge.

## Implications for Pass 13

- **Adopt the Thunderbird-shape throttle:** batch ceiling
  (~2 MB) + timer-slack between batches + idle-gating via a
  `lastActivity` timestamp threaded from `tea.KeyMsg` events.
- **Skip the watermark column.** SQL `LEFT JOIN bodies WHERE
  bytes IS NULL ORDER BY sent_at DESC` is the work queue;
  state lives in the cache itself.
- **Don't replicate Geary's bug.** Always honor server
  back-pressure (`[THROTTLED]`, JMAP rate-limit responses)
  with exponential back-off, cap 60s — same shape as the
  outbox drainer.
- **New mail jumps for free.** Newest-first ordering plus the
  existing `pumpUpdatesCmd` push integration means any newly-
  arrived UID is the next one the worker fetches.

## Sources

- [Thunderbird Source Docs — Folders](https://source-docs.thunderbird.net/en/latest/backend/folders.html)
- [Thunderbird Android — MessagingController (DeepWiki)](https://deepwiki.com/thunderbird/thunderbird-android/4.1-message-synchronization)
- [K-9 ImapSync.kt (GitHub)](https://github.com/thunderbird/thunderbird-android/blob/main/backend/imap/src/main/java/com/fsck/k9/backend/imap/ImapSync.kt)
- [Geary — downloads all emails despite prefetch limit (#130)](https://gitlab.gnome.org/GNOME/geary/-/issues/130)
- [Geary imap-db-folder.vala (GNOME GitLab)](https://gitlab.gnome.org/GNOME/geary/blob/main/src/engine/imap-db/imap-db-folder.vala)
- [Evolution IMAPX blog post (Chenthill, 2010)](https://chenthill.wordpress.com/2010/01/11/evolution-with-improved-imap-support-imapx/)
- [Evolution CamelDS.IMAPX wiki](https://wiki.gnome.org/Apps/Evolution/CamelDS.IMAPX)
- [aerc fetch.go (IMAP worker)](https://git.platypush.tech/blacklight/aerc/src/commit/4bc43d2741fa4904e51fc5da71d15b804c556c43/worker/imap/fetch.go)
- [aerc (rjarry) — GitHub mirror](https://github.com/rjarry/aerc)
- [Fastmail — Building offline: general architecture](https://www.fastmail.com/blog/offline-architecture/)
- [Fastmail — Building offline: mail storage](https://www.fastmail.com/blog/offline-mail-storage/)
- [Fastmail — Building offline: syncing changes back to the server](https://www.fastmail.com/blog/offline-sync/)
