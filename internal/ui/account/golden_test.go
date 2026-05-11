package account

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goldensDir = "../testdata/goldens"

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldensDir, name)
	if os.Getenv("UPDATE_GOLDENS") == "1" {
		if err := os.MkdirAll(goldensDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", goldensDir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDENS=1 to create)", path, err)
	}
	want := string(data)
	if got != want {
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(want, "\n")
		for i := range min(len(gotLines), len(wantLines)) {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("golden %s mismatch at line %d:\n  got  %q\n  want %q",
					name, i+1, gotLines[i], wantLines[i])
			}
		}
		t.Fatalf("golden %s mismatch: got %d lines, want %d lines",
			name, len(gotLines), len(wantLines))
	}
}

// TestGolden captures the full account.Model.View() at both
// design-polish sizes. Refactor passes must leave this output
// byte-for-byte equal.
func TestGolden(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"accounttab_80x24.txt", 80, 24},
		{"accounttab_120x40.txt", 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tab := newLoadedTab(t, tc.w, tc.h)
			got := tab.View()
			checkGolden(t, tc.name, got)
		})
	}
}
