package basic

// Func does something.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
//
// Extra line.
func Indent() {} // want Indent:"example"

func _() {
	if true {
		// #example Indent
		newCode()
		// #endexample
	}

	// #example Indent
	newCode()
	// #endexample

	// #example Indent
	if true {
		newCode()
	}
	// #endexample
}
