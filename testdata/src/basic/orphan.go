package basic

func _() {
	// #example NonExistent.Method // want `#example references unknown symbol "NonExistent.Method"`
	x := 42
	_ = x
	// #endexample
}
