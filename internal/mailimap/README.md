# internal/mailimap

Generic IMAP backend implementing `mail.Backend` over IMAP4rev1
via `emersion/go-imap`. UIDPLUS required; MOVE / SPECIAL-USE / IDLE
used opportunistically. Two physical connections per backend
(command + idle).

See `docs/superpowers/specs/2026-05-01-imap-backend-design.md`.

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
