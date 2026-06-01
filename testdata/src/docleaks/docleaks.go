package docleaks

// Func does something.
//
//	oldCode() // want "code block is out of sync with example code"
func Func() {} // want Func:"example"

// When #example immediately precedes a type declaration,
// the marker comment is parsed as the docs comment for the type.
//
// Verify that it does not get included in the output.
//
//	oldCode() // want "code block is out of sync with example code"
func MarkerBeforeType() {} // want MarkerBeforeType:"example"

func _() {
	// #example Func
	newCode()
	// #endexample

	// #example MarkerBeforeType
	type Typ struct{}
	// #endexample
}

func newCode() {}
