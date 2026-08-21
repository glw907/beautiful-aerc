package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clockGuardDirs are the two package directories QA-7's TZ-and-locale
// clause holds by construction (review round 1, finding 2): every
// gallery render is a pure function of its arguments, never the
// process's wall clock, because neither package reads one. Paths are
// relative to this package's directory, the one `go test` itself runs
// from; internal/ui/fixtures is deliberately not scanned, since a
// fixture's clock input is exactly what a future date-bearing screen
// pins there.
var clockGuardDirs = []string{".", "../theme"}

// bannedClockCalls are the time package members that read the
// process's wall clock or local zone rather than a caller-supplied
// instant.
var bannedClockCalls = map[string]bool{"Now": true, "Local": true, "Since": true}

// TestNoWallClockReferences replaces TestGallery_PinnedAgainstTZ
// (deleted, review round 1, finding 2: t.Setenv("TZ", ...) cannot
// move an already initialized time.Local, and no source this test
// scans called it anyway, so the test could never fail). This guards
// the property that is actually true, by construction, against a
// future regression: no non-test source in internal/ui or
// internal/theme may reference time.Now, time.Local, or time.Since. A
// pass-3 carry: once the first rendered timestamp lands, the clock
// becomes a fixture input and a subprocess-TZ render test becomes
// meaningful again.
func TestNoWallClockReferences(t *testing.T) {
	for _, dir := range clockGuardDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			checkNoWallClock(t, filepath.Join(dir, name))
		}
	}
}

func checkNoWallClock(t *testing.T, path string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "time" || !bannedClockCalls[sel.Sel.Name] {
			return true
		}
		t.Errorf("%s: references time.%s, which reads the process's wall clock; take an instant as an argument instead", path, sel.Sel.Name)
		return true
	})
}
