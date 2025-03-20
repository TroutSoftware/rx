package get

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestValidElem(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer)
}
