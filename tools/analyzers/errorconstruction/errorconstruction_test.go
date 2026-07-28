package errorconstruction_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/errorconstruction"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, errorconstruction.Analyzer,
		"a/internal/store", "a/internal/uerr")
}
