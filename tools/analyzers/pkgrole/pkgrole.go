// Package pkgrole classifies a poplar import path by the package it
// names under internal/ or cmd/, giving the import-boundary,
// write-call, styling, and error-construction analyzers a shared
// vocabulary for the dependency rules in the build-machine design.
package pkgrole

import (
	"go/ast"
	"strconv"
	"strings"
)

// ImportPath returns imp's import path, unquoted.
func ImportPath(imp *ast.ImportSpec) string {
	path, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		// The parser guarantees a valid quoted string here.
		panic(err)
	}
	return path
}

// Of returns the role named by path's internal/ or cmd/ segment
// (for example "ui" for ".../internal/ui/screens") and the module
// root preceding that segment. Of reports ok false for a
// third-party path with no internal/ or cmd/ segment.
func Of(path string) (role, moduleRoot string, ok bool) {
	for _, marker := range [...]string{"/internal/", "/cmd/"} {
		i := strings.Index(path, marker)
		if i < 0 {
			continue
		}
		rest := path[i+len(marker):]
		if j := strings.Index(rest, "/"); j >= 0 {
			rest = rest[:j]
		}
		return rest, path[:i], true
	}
	return "", "", false
}

// InModule reports whether path names a package inside the module
// rooted at moduleRoot: moduleRoot itself, or anything under it.
func InModule(path, moduleRoot string) bool {
	return path == moduleRoot || strings.HasPrefix(path, moduleRoot+"/")
}
