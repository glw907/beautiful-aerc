package mailauth

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

// AgeFileStore persists refresh tokens as per-account age-encrypted files
// under a single directory. Each account produces two files: <account>.key
// (bech32 X25519 secret, mode 0600) and <account>.age (ciphertext, mode 0600).
type AgeFileStore struct {
	dir string
}

// NewAgeFileStore returns an AgeFileStore rooted at dir.
func NewAgeFileStore(dir string) *AgeFileStore {
	return &AgeFileStore{dir: dir}
}

func validateAccount(account string) error {
	if strings.ContainsAny(account, `/\`) || strings.Contains(account, "..") {
		return fmt.Errorf("mailauth: account name contains invalid characters")
	}
	return nil
}

// Set encrypts refresh and writes it to <dir>/<account>.age, generating a
// fresh X25519 key in <dir>/<account>.key if one does not already exist.
func (s *AgeFileStore) Set(account, refresh string) error {
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("mailauth: create store dir: %w", err)
	}

	identity, err := s.loadOrCreateIdentity(account)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, identity.Recipient())
	if err != nil {
		return fmt.Errorf("mailauth: age encrypt: %w", err)
	}
	if _, err := io.WriteString(w, refresh); err != nil {
		return fmt.Errorf("mailauth: write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailauth: finalize ciphertext: %w", err)
	}

	return atomicWrite(s.dir, account+".age", buf.Bytes())
}

// Get decrypts and returns the refresh token for account. Returns ("", nil)
// when no token file exists.
func (s *AgeFileStore) Get(account string) (string, error) {
	if err := validateAccount(account); err != nil {
		return "", err
	}

	agePath := filepath.Join(s.dir, account+".age")
	if _, err := os.Stat(agePath); os.IsNotExist(err) {
		return "", nil
	}

	identity, err := s.readIdentity(account)
	if err != nil {
		return "", err
	}

	ciphertext, err := os.ReadFile(agePath)
	if err != nil {
		return "", fmt.Errorf("mailauth: read token file: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return "", fmt.Errorf("mailauth: age decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("mailauth: read plaintext: %w", err)
	}
	return string(plain), nil
}

// Delete removes the token and key files for account. Missing files are not errors.
func (s *AgeFileStore) Delete(account string) error {
	if err := validateAccount(account); err != nil {
		return err
	}
	for _, name := range []string{account + ".age", account + ".key"} {
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("mailauth: remove %s: %w", name, err)
		}
	}
	return nil
}

func (s *AgeFileStore) loadOrCreateIdentity(account string) (*age.X25519Identity, error) {
	keyPath := filepath.Join(s.dir, account+".key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return nil, fmt.Errorf("mailauth: generate identity: %w", err)
		}
		if err := atomicWrite(s.dir, account+".key", []byte(id.String())); err != nil {
			return nil, err
		}
		return id, nil
	}
	return s.readIdentity(account)
}

func (s *AgeFileStore) readIdentity(account string) (*age.X25519Identity, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, account+".key"))
	if err != nil {
		return nil, fmt.Errorf("mailauth: read key file: %w", err)
	}
	id, err := age.ParseX25519Identity(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("mailauth: parse identity: %w", err)
	}
	return id, nil
}

// atomicWrite writes data to dir/name via a temp file and rename, with mode 0600.
func atomicWrite(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, name+".tmp.*")
	if err != nil {
		return fmt.Errorf("mailauth: create temp file: %w", err)
	}
	tmpPath := f.Name()

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("mailauth: write temp file: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("mailauth: chmod temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("mailauth: close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, name)); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("mailauth: rename to %s: %w", name, err)
	}
	return nil
}
