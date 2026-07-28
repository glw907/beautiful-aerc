// Package importboundary reports poplar package imports that cross
// the boundaries fixed by the build-machine design (technical
// design section 2): the UI layer never reaches the sync seam, the
// markdown editor stays free of poplar imports, and the pure-logic
// packages stay free of store handles and I/O.
package importboundary

import (
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check the poplar package dependency boundary

Reports:
  - internal/ui importing internal/backend, internal/sync, or
    internal/outbox
  - internal/catkin importing any package from the poplar module
  - internal/render, internal/when, internal/search, or
    internal/calendar importing internal/store or an I/O package
    (internal/backend, internal/sync, internal/outbox,
    internal/keyring, internal/platform)`

// Analyzer reports poplar package imports that cross the boundaries
// fixed by the build-machine design.
var Analyzer = &analysis.Analyzer{
	Name: "importboundary",
	Doc:  doc,
	Run:  run,
}

// ioRoles names the roles render, when, search, and calendar may
// not import: internal/store (a store handle) and the packages
// that speak a wire protocol or touch hardware.
var ioRoles = []string{"store", "backend", "sync", "outbox", "keyring", "platform"}

// forbidden maps a role to the roles it must not import.
var forbidden = map[string][]string{
	"ui":       {"backend", "sync", "outbox"},
	"render":   ioRoles,
	"when":     ioRoles,
	"search":   ioRoles,
	"calendar": ioRoles,
}

func run(pass *analysis.Pass) (any, error) {
	role, moduleRoot, ok := pkgrole.Of(pass.Pkg.Path())
	if !ok {
		return nil, nil
	}

	for _, f := range pass.Files {
		for _, imp := range f.Imports {
			path := pkgrole.ImportPath(imp)
			if reason, bad := violates(role, moduleRoot, path); bad {
				pass.Reportf(imp.Pos(), "internal/%s must not import %s: %s", role, path, reason)
			}
		}
	}
	return nil, nil
}

// violates reports whether a package with the given role may not
// import path, and why.
func violates(role, moduleRoot, path string) (reason string, bad bool) {
	if role == "catkin" {
		if pkgrole.InModule(path, moduleRoot) {
			return "catkin imports nothing from poplar", true
		}
		return "", false
	}

	impRole, impModuleRoot, ok := pkgrole.Of(path)
	if !ok || impModuleRoot != moduleRoot {
		return "", false
	}
	if slices.Contains(forbidden[role], impRole) {
		return "internal/" + role + " does not reach internal/" + impRole, true
	}
	return "", false
}
