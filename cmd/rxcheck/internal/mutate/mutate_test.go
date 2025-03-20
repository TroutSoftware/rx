package mutate

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCheckMutate(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer)
}
