package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/sollniss/godocsync"
)

func main() {
	singlechecker.Main(godocsync.Analyzer)
}
