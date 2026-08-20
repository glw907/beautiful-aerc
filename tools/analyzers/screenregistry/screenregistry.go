// Package screenregistry reports a type under internal/ui/... whose
// method set implements ui.Screen but that no init ever hands to
// ui.Register: UX-1's acceptance criterion ("a reflection test fails
// on any screen type in internal/ui/... that is unregistered"),
// enforced by static analysis instead of a runtime test, since a
// runtime test can only see the registrations that ran in its own
// package's own test binary and a subpackage's screens never would.
//
// Candidate types come from go/types' own Implements check against
// ui.Screen's method set, resolved locally when the analyzed package
// is ui itself and through its imports otherwise, so a type reaches
// Screen's shape by promotion (an embedded screen) exactly as the
// Go compiler would see it, not by matching declared method syntax.
//
// Register is generic over the screen type: Register[MailScreen]
// (entry). A registration is read from the call's own type argument
// (go/types' Instances info), resolved through TypesInfo.ObjectOf
// regardless of the import alias, or the absence of one, that
// reached Register at the call site.
package screenregistry

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check every internal/ui/... Screen-shaped type is registered

Reports a named type under internal/ui/... whose method set
implements ui.Screen (Init, Update, View, Entry: go/types Implements,
so a promoted method from an embedded screen counts) but that no
Register[T] call in the same package names as its type argument.`

// Analyzer reports an unregistered Screen-shaped type under
// internal/ui/....
var Analyzer = &analysis.Analyzer{
	Name: "screenregistry",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	role, _, ok := pkgrole.Of(pass.Pkg.Path())
	if !ok || role != "ui" {
		return nil, nil
	}

	screenIface := findScreenInterface(pass.Pkg)
	if screenIface == nil {
		return nil, nil
	}

	candidates := candidateScreenTypes(pass.Pkg, screenIface)
	if len(candidates) == 0 {
		return nil, nil
	}

	registerObj := findRegister(pass.Pkg)
	registered := registeredTypeObjects(pass, registerObj)

	for _, tn := range candidates {
		if !registered[tn] {
			pass.Reportf(tn.Pos(), "type %s implements ui.Screen but is never registered via Register", tn.Name())
		}
	}
	return nil, nil
}

// findScreenInterface returns the underlying interface of the
// exported "Screen" type name, looked up in pkg itself (pkg is the
// ui root) or, failing that, in whichever direct import of pkg has
// the "ui" role (pkg is a subpackage of ui).
func findScreenInterface(pkg *types.Package) *types.Interface {
	if iface := interfaceNamed(pkg, "Screen"); iface != nil {
		return iface
	}
	for _, imp := range pkg.Imports() {
		if role, _, ok := pkgrole.Of(imp.Path()); ok && role == "ui" {
			if iface := interfaceNamed(imp, "Screen"); iface != nil {
				return iface
			}
		}
	}
	return nil
}

// findRegister returns the "Register" func object, resolved the same
// way findScreenInterface resolves "Screen".
func findRegister(pkg *types.Package) types.Object {
	if obj := pkg.Scope().Lookup("Register"); obj != nil {
		return obj
	}
	for _, imp := range pkg.Imports() {
		if role, _, ok := pkgrole.Of(imp.Path()); ok && role == "ui" {
			if obj := imp.Scope().Lookup("Register"); obj != nil {
				return obj
			}
		}
	}
	return nil
}

func interfaceNamed(pkg *types.Package, name string) *types.Interface {
	tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok {
		return nil
	}
	iface, ok := tn.Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	return iface
}

// candidateScreenTypes returns every named, non-interface type
// declared at pkg's package scope whose method set, or its pointer's
// method set, implements screenIface.
func candidateScreenTypes(pkg *types.Package, screenIface *types.Interface) []*types.TypeName {
	var out []*types.TypeName
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isIface := named.Underlying().(*types.Interface); isIface {
			continue
		}
		if types.Implements(named, screenIface) || types.Implements(types.NewPointer(named), screenIface) {
			out = append(out, tn)
		}
	}
	return out
}

// registeredTypeObjects scans pass's files for a call to registerObj
// with an explicit type argument, Register[T](entry), however
// Register itself was imported, and returns the set of T's
// *types.TypeName objects. It returns an empty set for a nil
// registerObj, so a package that cannot see Register at all (the ui
// root has none, an impossible case in practice) reports every
// candidate rather than panicking.
func registeredTypeObjects(pass *analysis.Pass, registerObj types.Object) map[*types.TypeName]bool {
	reg := make(map[*types.TypeName]bool)
	if registerObj == nil {
		return reg
	}
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident := registerCallIdent(call.Fun)
			if ident == nil || pass.TypesInfo.ObjectOf(ident) != registerObj {
				return true
			}
			inst, ok := pass.TypesInfo.Instances[ident]
			if !ok || inst.TypeArgs == nil || inst.TypeArgs.Len() == 0 {
				return true
			}
			if tn, ok := typeNameOf(inst.TypeArgs.At(0)); ok {
				reg[tn] = true
			}
			return true
		})
	}
	return reg
}

// registerCallIdent returns the identifier a generic call's Fun
// expression instantiates: Register[T] or pkg.Register[T], single or
// multiple type arguments, or nil for anything else.
func registerCallIdent(fun ast.Expr) *ast.Ident {
	var target ast.Expr
	switch e := fun.(type) {
	case *ast.IndexExpr:
		target = e.X
	case *ast.IndexListExpr:
		target = e.X
	default:
		return nil
	}
	switch t := target.(type) {
	case *ast.Ident:
		return t
	case *ast.SelectorExpr:
		return t.Sel
	}
	return nil
}

// typeNameOf returns t's *types.TypeName, unwrapping one level of
// pointer first so Register[*MailScreen] resolves to the same
// TypeName Register[MailScreen] does: the pointer-normalization
// Register itself performs at runtime (M13), mirrored here so the
// analyzer's registered set matches what actually gets registered.
func typeNameOf(t types.Type) (*types.TypeName, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}
	return named.Obj(), true
}
