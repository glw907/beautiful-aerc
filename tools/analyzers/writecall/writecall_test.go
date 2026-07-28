package writecall_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/glw907/poplar/tools/analyzers/writecall"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, writecall.Analyzer, "a/internal/ui", "a/internal/store")
}
