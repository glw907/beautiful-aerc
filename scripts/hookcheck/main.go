// hookcheck flags a production reference to internal/uerr's
// RedirectForTest, the test hook that swaps uerr's log destination for
// the whole process. It has to be exported so internal/uerr/uerrtest
// can reach it from another package, which also puts it within reach
// of every other package; one production call would divert every ER-1
// error line for the life of the process, the exact failure the uerr
// seam exists to prevent.
//
// Walks every .go file below the working directory that is neither a
// _test.go file nor inside internal/uerr, prints file:line: UH1 for
// each RedirectForTest reference, and exits 1. internal/uerr/uerrtest
// is inside that tree by design: it is ADR-0014's test-only sibling of
// internal/uerr, and every function it exports takes a *testing.T.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// hook is the identifier no production file may name.
const hook = "RedirectForTest"

// uerrDir is the one package tree allowed to reference hook, in
// slash-separated form regardless of the host's separator.
const uerrDir = "internal/uerr"

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	hits := 0
	fset := token.NewFileSet()
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(path)
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") && name != "." {
				return fs.SkipDir
			}
			if slashed == uerrDir || strings.HasSuffix(slashed, "/"+uerrDir) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hookcheck: %s: %v\n", path, err)
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok || ident.Name != hook {
				return true
			}
			pos := fset.Position(ident.Pos())
			fmt.Printf("%s:%d: UH1 %s referenced outside %s from a non-test file; it redirects every error line in the process\n",
				pos.Filename, pos.Line, hook, uerrDir)
			hits++
			return true
		})
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		fmt.Fprintf(os.Stderr, "hookcheck: %v\n", err)
		os.Exit(2)
	}
	if hits > 0 {
		os.Exit(1)
	}
}
