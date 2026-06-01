package godocsync_test

import (
	"testing"

	"github.com/sollniss/analysisbench"
	"github.com/sollniss/godocsync"
)

func BenchmarkAnalyzer(b *testing.B) {
	testdata := analysisbench.TestData()
	analysisbench.Run(b, testdata, godocsync.Analyzer, "basic", "multifile")
}

func BenchmarkAnalyzerPerPackage(b *testing.B) {
	testdata := analysisbench.TestData()
	analysisbench.RunPerPackage(b, testdata, godocsync.Analyzer, "basic", "multifile")
}
