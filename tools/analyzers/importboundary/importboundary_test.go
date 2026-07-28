package importboundary_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/importboundary"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, importboundary.Analyzer,
		"a/internal/ui", "a/internal/catkin", "a/internal/render",
		"a/internal/when", "a/internal/search", "a/internal/calendar")
}
