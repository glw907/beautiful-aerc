// Package importboundary reports poplar package imports that cross
// the boundaries fixed by the build-machine design (technical
// design section 2): the UI layer never reaches the sync seam, the
// markdown editor stays free of poplar imports, and the pure-logic
// packages stay free of store handles and I/O.
package importboundary

import (
	"slices"
	"strings"

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
    internal/keyring, internal/platform)
  - any package in the module, other than internal/backend/jmapsource,
    or jmap and its own subpackages, importing the module's jmap
    package or a subpackage of it, whether or not the importer's own
    path falls under internal/ or cmd/`

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

// jmapImporter is the one package that may import the module's jmap
// package, the wire protocol transport internal/backend/jmapsource
// speaks against Fastmail. jmap sits directly under the module root
// rather than under internal/ or cmd/, so pkgrole has no role for it
// and the forbidden-role table above cannot express this rule; it is
// checked by full package path instead.
const jmapImporter = "internal/backend/jmapsource"

func run(pass *analysis.Pass) (any, error) {
	// An external test package's path carries a "_test" suffix pkgrole
	// leaves in place on the role-only check below (it trims only the
	// role segment, not the whole path), and go/packages' own
	// synthesized test-binary main for a package with an Example
	// function (jmap/example_test.go) carries a ".test" suffix
	// instead, reported from a GOCACHE path rather than real source.
	// Both are trimmed here so a package's own black-box tests and its
	// own compiled test binary are compared against jmapImporter (or
	// exempted as jmap testing itself, in violatesJMAPCarveOut) under
	// the same identity as the package itself, rather than read as a
	// different, unclassifiable importer.
	pkgPath := strings.TrimSuffix(strings.TrimSuffix(pass.Pkg.Path(), "_test"), ".test")

	// role/moduleRoot/roleOK gate the role-to-role checks alone.
	// violatesJMAPCarveOut below does not: it derives its own
	// moduleRoot from the jmap import itself, so it runs for every
	// package in the module, including one under scripts/ or any
	// other path pkgrole.Of cannot classify. A package outside
	// internal/ and cmd/ was exactly how this rule's own gap
	// surfaced: an earlier version of run returned here, before ever
	// reaching the jmap check below, whenever roleOK was false.
	role, moduleRoot, roleOK := pkgrole.Of(pass.Pkg.Path())

	for _, f := range pass.Files {
		for _, imp := range f.Imports {
			path := pkgrole.ImportPath(imp)
			if roleOK {
				if reason, bad := violates(role, moduleRoot, path); bad {
					pass.Reportf(imp.Pos(), "internal/%s must not import %s: %s", role, path, reason)
					continue
				}
			}
			if reason, bad := violatesJMAPCarveOut(pass, pkgPath, path); bad {
				pass.Reportf(imp.Pos(), "%s must not import %s: %s", pkgPath, path, reason)
			}
		}
	}
	return nil, nil
}

// violatesJMAPCarveOut reports whether path names the module's jmap
// package or a subpackage of it, and importer (pkgPath) is neither
// jmapImporter nor jmap itself. moduleRoot comes from jmapModuleRootFor
// rather than from importer's own path, so the rule fires for every
// importer in the module and not only ones pkgrole can classify, and
// covers a subpackage a later task adds under jmap/ (jmap/eventsource,
// say) rather than matching only the exact top-level path.
//
// The jmap-itself exemption covers jmap's own external test package
// (package jmap_test, example_test.go's idiom), which legitimately
// imports jmap to demonstrate calling it as a consumer would, and any
// subpackage of jmap importing jmap or a sibling subpackage: both are
// the library wiring or testing itself, not an external consumer.
// jmap/doc.go's "imports no poplar package" is jmap's promise to the
// rest of the module, not a promise to itself.
func violatesJMAPCarveOut(pass *analysis.Pass, importer, path string) (reason string, bad bool) {
	moduleRoot, ok := jmapModuleRootFor(pass, path)
	if !ok || !pkgrole.InModule(importer, moduleRoot) {
		return "", false
	}
	if importer == moduleRoot+"/"+jmapImporter {
		return "", false
	}
	if pkgrole.InModule(importer, moduleRoot+"/jmap") {
		return "", false
	}
	return "only " + jmapImporter + " may import jmap", true
}

// jmapModuleRootFor reports the module root to check path against.
// pass.Module.Path, when the driver supplies one, is authoritative:
// a real go vet or poplarcheck invocation carries it, and using it
// instead of path's own structure closes a collision jmapModuleRoot
// alone cannot: an import of some unrelated github.com/jmap/anything
// would otherwise derive a moduleRoot of "github.com", which every
// poplar package satisfies pkgrole.InModule against.
//
// analysistest's own testdata loads as a GOPATH tree with no go.mod,
// which this package's own test exercises: the driver there hands
// back a non-nil pass.Module with an empty Path rather than a nil
// pass.Module, so the check is against Path being empty, not against
// Module being nil, and jmapModuleRootFor falls back to
// jmapModuleRoot for that case the same as a nil Module.
func jmapModuleRootFor(pass *analysis.Pass, path string) (moduleRoot string, ok bool) {
	if pass.Module != nil && pass.Module.Path != "" {
		root := pass.Module.Path + "/jmap"
		if path == root || strings.HasPrefix(path, root+"/") {
			return pass.Module.Path, true
		}
		return "", false
	}
	return jmapModuleRoot(path)
}

// jmapModuleRoot reports the module root implied by path, when path
// names package jmap or a subpackage of it: everything before path's
// first "jmap" path segment. It returns ok false for a path with no
// such segment, including one that merely ends in "jmap" as part of
// a longer word (myjmap) rather than as its own segment. It is a
// heuristic over path's own structure, not module-aware, so
// jmapModuleRootFor uses it only when the driver supplies no
// pass.Module to check against instead.
func jmapModuleRoot(path string) (moduleRoot string, ok bool) {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "jmap" {
			return strings.Join(segments[:i], "/"), true
		}
	}
	return "", false
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
