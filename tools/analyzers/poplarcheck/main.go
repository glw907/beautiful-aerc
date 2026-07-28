// Command poplarcheck runs poplar's four go/analysis passes
// (technical design section 18 item 7): import-boundary, write-call,
// styling, and error-construction. Run standalone over a package
// pattern, or as a go vet -vettool plugin. multichecker.Main detects
// the .cfg argument go vet passes and switches to the unitchecker
// protocol with no extra glue.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/glw907/poplar/tools/analyzers/errorconstruction"
	"github.com/glw907/poplar/tools/analyzers/importboundary"
	"github.com/glw907/poplar/tools/analyzers/styling"
	"github.com/glw907/poplar/tools/analyzers/writecall"
)

func main() {
	multichecker.Main(
		importboundary.Analyzer,
		writecall.Analyzer,
		styling.Analyzer,
		errorconstruction.Analyzer,
	)
}
