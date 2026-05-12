package mailauth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const keyringSvc = "poplar-oauth"

// ErrKeyringUnavailable is returned when the system keyring backend cannot
// be reached (no DBus session, unsupported platform, etc.).
var ErrKeyringUnavailable = errors.New("mailauth: system keyring unavailable")

// KeyringStore persists refresh tokens in the system keyring.
type KeyringStore struct{}

// NewKeyringStore returns a KeyringStore. Call OpenStore to get a working
// backend; NewKeyringStore alone does not probe availability.
func NewKeyringStore() *KeyringStore { return &KeyringStore{} }

func (s *KeyringStore) Set(account, refresh string) error {
	err := keyring.Set(keyringSvc, account, refresh)
	if err == nil {
		return nil
	}
	if isKeyringInit(err) {
		return ErrKeyringUnavailable
	}
	return err
}

func (s *KeyringStore) Get(account string) (string, error) {
	val, err := keyring.Get(keyringSvc, account)
	if err == nil {
		return val, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if isKeyringInit(err) {
		return "", ErrKeyringUnavailable
	}
	return "", err
}

func (s *KeyringStore) Delete(account string) error {
	err := keyring.Delete(keyringSvc, account)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if isKeyringInit(err) {
		return ErrKeyringUnavailable
	}
	return err
}

func isKeyringInit(err error) bool {
	return errors.Is(err, keyring.ErrUnsupportedPlatform)
}

// OpenStore returns the requested token-store backend. When preferred
// is BackendKeyring or BackendAgeFile the requested store is returned
// without probing. The user's explicit `oauth-store` choice wins, so
// a config-time decision doesn't drift between processes if keyring
// availability flips. When preferred is empty the historical probe
// runs: write+read+delete against the system keyring; on success
// return KeyringStore, on failure fall back to an AgeFileStore rooted
// at fallbackDir. The returned error is always nil; fallback is
// intentional.
func OpenStore(slug, fallbackDir string, preferred Backend) (TokenStore, Backend, error) {
	switch preferred {
	case BackendKeyring:
		return NewKeyringStore(), BackendKeyring, nil
	case BackendAgeFile:
		return NewAgeFileStore(fallbackDir), BackendAgeFile, nil
	}
	s := NewKeyringStore()
	probe := slug + "__probe__"
	if err := s.Set(probe, "ok"); err == nil {
		if got, err := s.Get(probe); err == nil && got == "ok" {
			_ = s.Delete(probe)
			return s, BackendKeyring, nil
		}
		_ = s.Delete(probe)
	}
	return NewAgeFileStore(fallbackDir), BackendAgeFile, nil
}
