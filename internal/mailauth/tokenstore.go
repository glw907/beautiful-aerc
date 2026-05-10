// Package mailauth persists and retrieves OAuth refresh tokens. Two
// implementations are provided: KeyringStore (system keyring via
// zalando/go-keyring) and AgeFileStore (per-account age-encrypted file).
// OpenStore probes the keyring and picks the working backend.
package mailauth

import "errors"

// Backend names the token-store implementation in use.
type Backend string

const (
	BackendKeyring Backend = "keyring"
	BackendAgeFile Backend = "age-file"
)

// ErrNotStored is returned by Get when no token exists for the account.
var ErrNotStored = errors.New("mailauth: token not stored")

// TokenStore persists and retrieves OAuth refresh tokens.
type TokenStore interface {
	Set(account, refresh string) error
	Get(account string) (string, error) // returns "", ErrNotStored when missing
	Delete(account string) error
}
