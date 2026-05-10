// Package mailauth persists and retrieves OAuth refresh tokens. Two
// implementations are provided: KeyringStore (system keyring via
// zalando/go-keyring) and AgeFileStore (per-account age-encrypted file).
// OpenStore probes the keyring and picks the working backend.
package mailauth

// Backend names the token-store implementation in use.
type Backend string

const (
	BackendKeyring Backend = "keyring"
	BackendAgeFile Backend = "age-file"
)

// TokenStore persists and retrieves OAuth refresh tokens.
type TokenStore interface {
	Set(account, refresh string) error
	Get(account string) (string, error) // returns ("", nil) when no token is stored
	Delete(account string) error
}
