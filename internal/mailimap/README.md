# internal/mailimap

Generic IMAP backend implementing `mail.Backend` over IMAP4rev1
via `emersion/go-imap`. UIDPLUS required; MOVE / SPECIAL-USE / IDLE
used opportunistically. Two physical connections per backend
(command + idle).

See `docs/superpowers/specs/2026-05-01-imap-backend-design.md`.

## Gmail (`GmailQuirks = true`)

The `gmail` provider preset sets `GmailQuirks` on the
`AccountConfig`. The IMAP backend then:

- Asserts `X-GM-EXT-1` at Connect; refuses to start without it.
- Routes `Destroy(uids)` through `SELECT [Gmail]/Trash` before
  `STORE \Deleted` + `UID EXPUNGE`. Gmail's IMAP server only
  permanently deletes when the SELECTed mailbox is `[Gmail]/Trash`;
  EXPUNGE elsewhere just removes the matching label. Callers must
  pass UIDs that already live in Trash — both real callers
  (manual Empty Trash, retention sweep) operate from inside
  Disposal folders, so this is satisfied.

XOAUTH2 access tokens are short-lived (~1h). Poplar does not
refresh them internally yet; wire `password-cmd` to a refresher
(`oauth2l`, `op`, etc.). Internal token-endpoint exchange lands
with the first-run wizard.

## Tests

Unit tests use a fake `imapClient` (see `fake_test.go`) and run
under plain `go test ./internal/mailimap/...`.

Integration tests require a live IMAP server. They are guarded by
`//go:build integration` and run via `make test-imap`.

### Local Dovecot setup

```sh
docker run -d --name poplar-dovecot \
  -p 1143:143 \
  -e DOVECOT_USERS="testuser:{plain}testpass:::::" \
  dovecot/dovecot

export POPLAR_TEST_IMAP_HOST=127.0.0.1
export POPLAR_TEST_IMAP_USER=testuser@example.com
export POPLAR_TEST_IMAP_PASS=testpass

make test-imap
```

Tear down: `docker rm -f poplar-dovecot`.

(The `dovecot/dovecot` image's exact env-var contract may vary by
version; consult its README and adjust the command above. Goal:
one IMAP user with a known password reachable on `localhost:1143`
with STARTTLS available.)
