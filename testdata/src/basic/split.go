package basic

// SplitExample does something.
//
//	oldCode() // want "code block is out of sync with example code"
func SplitExample() {} // want SplitExample:"example"

func _() {
	var i int
	// #example SplitExample
	i++
	// #endexample
	i--
	// #example SplitExample
	i++
	// #endexample
	_ = i
}
