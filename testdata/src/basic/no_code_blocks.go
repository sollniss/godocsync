package basic

// NoCodeBlocks does something useful.
func NoCodeBlocks() {} // want NoCodeBlocks:"example"

func _() {
	// #example NoCodeBlocks // want "NoCodeBlocks has no corresponding code block"
	var _ struct{}
	// #endexample
}
