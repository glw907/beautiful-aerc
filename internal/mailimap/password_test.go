// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

// counterCmd returns a shell command that increments a per-test counter
// file and prints the new value, so two invocations yield distinct outputs
// ("1" then "2"). Used to detect whether resolvedPassword re-runs the cmd.
func counterCmd(t *testing.T) string {
	t.Helper()
	counter := filepath.Join(t.TempDir(), "n")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(
		`n=$(cat %q); n=$((n+1)); printf %%s "$n" > %q; printf %%s "$n"`,
		counter, counter)
}

func TestResolvedPassword_XOAUTH2_BypassesCache(t *testing.T) {
	b := New(config.AccountConfig{
		Name:        "g",
		Auth:        "xoauth2",
		PasswordCmd: counterCmd(t),
	})

	first, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first == second {
		t.Errorf("xoauth2 should not cache: first=%q second=%q (want different)", first, second)
	}
}

func TestResolvedPassword_NonXOAUTH2_Caches(t *testing.T) {
	b := New(config.AccountConfig{
		Name:        "f",
		Auth:        "plain",
		PasswordCmd: counterCmd(t),
	})

	first, _ := b.resolvedPassword()
	second, _ := b.resolvedPassword()
	if first != second {
		t.Errorf("plain auth should cache: first=%q second=%q (want equal)", first, second)
	}
}
