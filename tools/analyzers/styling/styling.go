// Package styling reports UX-3's positional styling boundary:
// outside internal/theme and internal/catkin, in non-test files, no
// non-ASCII literal, no ANSI escape literal, and no direct lipgloss
// call. Everything else styles through the theme's token API.
//
// The check is positional, not a taint analysis: it flags the
// literal or call site itself, not whether the value it produces
// reaches rendered output. A line carrying a `//poplar:allow-unicode
// <reason>` comment is exempt (a `/*poplar:allow-unicode <reason>*/`
// block comment works too, so the directive can share a line with
// another trailing comment). The reason is mandatory, and each use
// surfaces as its own diagnostic so a pass-end reviewer can find the
// ones a diff adds.
package styling

import (
	"go/ast"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check the poplar styling boundary (UX-3)

Reports, outside internal/theme and internal/catkin in non-test
files:
  - a string or rune literal containing a non-ASCII code point
  - a string or rune literal containing an ANSI escape byte (0x1b)
  - a call into a lipgloss package

A //poplar:allow-unicode <reason> comment on the same line exempts
the finding; the reason is required.`

// Analyzer reports styling-boundary violations and returns the
// count of `//poplar:allow-unicode` escapes it honored.
var Analyzer = &analysis.Analyzer{
	Name:       "styling",
	Doc:        doc,
	Run:        run,
	ResultType: reflect.TypeOf(0),
}

var allowUnicode = regexp.MustCompile(`^poplar:allow-unicode\s+(\S.*)$`)

func run(pass *analysis.Pass) (any, error) {
	role, _, ok := pkgrole.Of(pass.Pkg.Path())
	if ok && (role == "theme" || role == "catkin") {
		return 0, nil
	}

	escapes := 0
	for _, f := range pass.Files {
		filename := pass.Fset.Position(f.Pos()).Filename
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		reasons := lineReasons(pass.Fset, f)
		lipglossAliases := lipglossAliases(f)

		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if kind, bad := badLiteral(n); bad {
					escapes += report(pass, reasons, n.Pos(), "%s literal outside internal/theme and internal/catkin", kind)
				}
			case *ast.CallExpr:
				if callsLipgloss(n, lipglossAliases) {
					escapes += report(pass, reasons, n.Pos(), "lipgloss call outside internal/theme and internal/catkin")
				}
			}
			return true
		})
	}
	return escapes, nil
}

// badLiteral reports whether lit is a string or rune literal
// carrying a non-ASCII code point or an ANSI escape byte, and which.
func badLiteral(lit *ast.BasicLit) (kind string, bad bool) {
	if lit.Kind != token.STRING && lit.Kind != token.CHAR {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	for _, r := range value {
		if r == 0x1b {
			return "ANSI escape", true
		}
		if r > 0x7f {
			return "non-ASCII", true
		}
	}
	return "", false
}

func callsLipgloss(call *ast.CallExpr, aliases map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	return ok && aliases[pkgIdent.Name]
}

// lipglossAliases returns the local identifiers f uses for a
// lipgloss package import.
func lipglossAliases(f *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range f.Imports {
		path := pkgrole.ImportPath(imp)
		if !strings.Contains(path, "lipgloss") {
			continue
		}
		if imp.Name != nil {
			aliases[imp.Name.Name] = true
			continue
		}
		aliases[path[strings.LastIndex(path, "/")+1:]] = true
	}
	return aliases
}

// lineReasons maps a source line to the poplar:allow-unicode reason
// given on it, for the lines that carry one. The directive reads
// from either a line comment or a block comment, so it can share a
// source line with another trailing line comment.
func lineReasons(fset *token.FileSet, f *ast.File) map[int]string {
	reasons := make(map[int]string)
	for _, group := range f.Comments {
		for _, c := range group.List {
			text, ok := strings.CutPrefix(c.Text, "//")
			if !ok {
				text, ok = strings.CutPrefix(c.Text, "/*")
				text = strings.TrimSuffix(text, "*/")
			}
			if !ok {
				continue
			}
			m := allowUnicode.FindStringSubmatch(strings.TrimSpace(text))
			if m == nil {
				continue
			}
			reasons[fset.Position(c.Pos()).Line] = m[1]
		}
	}
	return reasons
}

// report emits the violation at pos, or the escape diagnostic in
// its place when the line carries an allow-unicode reason, and
// returns 1 if it counted as an escape.
func report(pass *analysis.Pass, reasons map[int]string, pos token.Pos, format string, args ...any) int {
	line := pass.Fset.Position(pos).Line
	if reason, ok := reasons[line]; ok {
		pass.Reportf(pos, "poplar:allow-unicode escape used: %s", reason)
		return 1
	}
	pass.Reportf(pos, format, args...)
	return 0
}
