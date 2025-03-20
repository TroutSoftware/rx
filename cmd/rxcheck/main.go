package main

import (
	"github.com/TroutSoftware/rx/cmd/rxcheck/internal/get"
	"github.com/TroutSoftware/rx/cmd/rxcheck/internal/mutate"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(get.Analyzer, mutate.Analyzer)
}
