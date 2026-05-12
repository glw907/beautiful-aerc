package mailauth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgeFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewAgeFileStore(dir)

	if err := s.Set("acct-a", "refresh-token-xyz"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("acct-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "refresh-token-xyz" {
		t.Errorf("Get = %q, want %q", got, "refresh-token-xyz")
	}
}

func TestAgeFileStoreGetMissing(t *testing.T) {
	dir := t.TempDir()
	s := NewAgeFileStore(dir)

	got, err := s.Get("nope")
	if err != nil {
		t.Fatalf("Get missing: %v", err)
	}
	if got != "" {
		t.Errorf("Get missing = %q, want empty", got)
	}
}

func TestAgeFileStoreFileMode(t *testing.T) {
	dir := t.TempDir()
	s := NewAgeFileStore(dir)

	if err := s.Set("acct-a", "token"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	for _, name := range []string{"acct-a.age", "acct-a.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("Stat %s: %v", name, err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", name, mode)
		}
	}
}

func TestAgeFileStoreDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewAgeFileStore(dir)

	if err := s.Set("acct-a", "token"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete("acct-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := s.Get("acct-a")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != "" {
		t.Errorf("Get after delete = %q, want empty", got)
	}
}

func TestKeyringStoreRoundTripIfAvailable(t *testing.T) {
	s := NewKeyringStore()
	if err := s.Set("keyring-test-acct", "tok"); err != nil {
		if err == ErrKeyringUnavailable {
			t.Skip("system keyring unavailable")
		}
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete("keyring-test-acct") })

	got, err := s.Get("keyring-test-acct")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "tok" {
		t.Errorf("Get = %q, want %q", got, "tok")
	}
	if err := s.Delete("keyring-test-acct"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestOpenStorePrefersKeyring(t *testing.T) {
	store, _, err := OpenStore("probe-acct", t.TempDir(), "")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Set("probe-acct", "val"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete("probe-acct") })

	got, err := store.Get("probe-acct")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "val" {
		t.Errorf("Get = %q, want %q", got, "val")
	}
	_ = store.Delete("probe-acct")
}

func TestAgeFileStoreAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewAgeFileStore(dir)

	if err := s.Set("acct-a", "first"); err != nil {
		t.Fatalf("Set first: %v", err)
	}
	if err := s.Set("acct-a", "second"); err != nil {
		t.Fatalf("Set second: %v", err)
	}

	// No leftover .tmp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}

	// Second value wins.
	got, err := s.Get("acct-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "second" {
		t.Errorf("Get = %q, want %q", got, "second")
	}
}
