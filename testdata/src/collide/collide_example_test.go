package collide_test

// The import of collide/other comes first, in its own group, so that a match
// by package name finds the wrong package.
import (
	other "collide/other"

	"collide"
	"testing"
)

func TestExample(t *testing.T) {
	other.Other()

	// #example Thing
	collide.Thing()
	// #endexample
}
