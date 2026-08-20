package screenregistry_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/screenregistry"
)

// TestAnalyzer proves the happy path (GoodScreen, SubScreen: each
// registered, neither flagged) and all four evasions the orchestrator
// named: an unregistered type in the ui root, an unregistered type
// reached only through an aliased import of ui, one reached only
// through a dot import, and one that implements Screen solely by
// embedding a registered screen's promoted methods, plus the
// subpackage case itself, since every fixture but GoodScreen and
// bad_unregistered.go's UnregisteredScreen lives under a/internal/ui/sub.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, screenregistry.Analyzer, "a/internal/ui", "a/internal/ui/sub")
}
