package pkg_test

import (
	"testing"

	pkg "github.com/sollniss/godocsync/examples"
)

func _() {
	// #region packagedoc
	// #example package
	pkg.Do()
	// #endexample
	// // #endregion
}

// #region crosspackage
func TestExample(t *testing.T) {
	// #example DoSomething
	pkg.DoSomething()
	// #endexample
}

// #endregion
