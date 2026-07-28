package styling_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/styling"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, styling.Analyzer,
		"a/internal/ui", "a/internal/theme", "a/internal/catkin")
}
