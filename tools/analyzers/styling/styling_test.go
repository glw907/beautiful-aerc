package styling_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/styling"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	results := analysistest.Run(t, testdata, styling.Analyzer,
		"a/internal/ui", "a/internal/theme", "a/internal/catkin")

	for _, r := range results {
		if r.Pass.Pkg.Path() != "a/internal/ui" {
			continue
		}
		if got := r.Result.(int); got != 1 {
			t.Errorf("escape count for a/internal/ui = %d, want 1 (the allow-unicode café line reports no diagnostic but still counts)", got)
		}
	}
}
