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
