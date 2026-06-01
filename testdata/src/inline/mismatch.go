package inline

// MixedA demonstrates /* */ open with // close.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
func MixedA() {} // want MixedA:"example"

// MixedB demonstrates // open with /* */ close.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
func MixedB() {} // want MixedB:"example"

func _() {
	// Inline open, line close:
	_ = /* #example MixedA */ MyString("mixed-a")
	// #endexample

	// Line open, inline close:
	// #example MixedB
	_ = MyString("mixed-b")
	/* #endexample */
}
