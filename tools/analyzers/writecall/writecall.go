// Package writecall reports code outside ADR-0003's writer cast
// that reaches internal/store's write surface, the
// package-boundary half of ADR-0003 (technical design section 2).
// The store, the engines that hold a Writer, and the command that
// starts one may write. Every other package mutates by enqueueing
// an intent.
//
// The guarded surface comes from the store's own types rather than
// from a name pattern: a store symbol is a write entry point when
// it accepts, returns, or is a method on a write capability, which
// is the store's Writer, a *sql.DB over the store file, or a
// *sql.Tx. A store function that hides its capability behind a
// path is named in writeByName instead, and the write-surface test
// fails when internal/store grows an export neither rule covers.
//
// The analyzer does not verify that a call site reaches a write
// against a live connection, and it does not forbid Exec on a read
// connection. That half is not statically decidable in
// go/analysis, so the store's read API owns it instead, by
// returning a named read-only handle type with no Exec method.
package writecall

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check store writes stay inside ADR-0003's writer cast

Reports a reference from outside internal/store, internal/sync,
internal/outbox, and cmd/poplar to internal/store's write surface:
the Writer type and its methods, the openers that yield a write
connection, and the transaction mutators. Everywhere else mutates
by enqueueing an intent.`

// Analyzer reports internal/store's write surface reached from
// outside the writer cast.
var Analyzer = &analysis.Analyzer{
	Name: "writecall",
	Doc:  doc,
	Run:  run,
}

// writerCast holds the roles ADR-0003 puts on the writing side of
// the store boundary: the store itself, the two engines that hold a
// Writer and run transactions through it, and the command that
// opens the store and starts the writer.
var writerCast = map[string]bool{
	"store":  true,
	"sync":   true,
	"outbox": true,
	"poplar": true,
}

// writeByName holds the store exports whose write capability their
// signature does not carry. Recover takes a path and rebuilds the
// database file behind it.
var writeByName = map[string]bool{
	"Recover": true,
}

const writerType = "Writer"

func run(pass *analysis.Pass) (any, error) {
	role, _, ok := pkgrole.Of(pass.Pkg.Path())
	if !ok || writerCast[role] {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[sel.Sel]
			if obj == nil || !inStore(obj.Pkg()) || !writeEntryPoint(obj) {
				return true
			}
			pass.Reportf(sel.Pos(),
				"internal/%s reaches store.%s, part of internal/store's write surface; mutate through an intent (ADR-0003)",
				role, obj.Name())
			return true
		})
	}
	return nil, nil
}

// writeEntryPoint reports whether obj, a member of internal/store,
// is part of the store's write surface.
func writeEntryPoint(obj types.Object) bool {
	switch obj := obj.(type) {
	case *types.TypeName:
		return obj.Name() == writerType
	case *types.Func:
		return writeByName[obj.Name()] || carriesWriteCapability(obj.Signature())
	}
	return false
}

// carriesWriteCapability reports whether sig hands a write
// capability across the store boundary, in either direction, or
// binds one as a receiver.
func carriesWriteCapability(sig *types.Signature) bool {
	if recv := sig.Recv(); recv != nil && isWriteCapability(recv.Type()) {
		return true
	}
	for _, tuple := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := range tuple.Len() {
			if isWriteCapability(tuple.At(i).Type()) {
				return true
			}
		}
	}
	return false
}

// isWriteCapability reports whether t is one of the three handles a
// write travels on: the store's Writer, a database/sql connection,
// or a transaction.
func isWriteCapability(t types.Type) bool {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() != nil && obj.Pkg().Path() == "database/sql" {
		return obj.Name() == "DB" || obj.Name() == "Tx"
	}
	return inStore(obj.Pkg()) && obj.Name() == writerType
}

// inStore reports whether pkg is internal/store or one of its
// subpackages.
func inStore(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	role, _, ok := pkgrole.Of(pkg.Path())
	return ok && role == "store"
}
