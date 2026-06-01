package basic

// First:
//
//	oldCode() // want "code block is out of sync with example code"
//	oldCode()
//
// Second:
//
//	oldCode() // want "code block is out of sync with example code"
//	oldCode()
//
// Extra line.
func OrdinalAll() {} // want OrdinalAll:"example"

func _() {
	// #example OrdinalAll 1
	newCode()
	newCode()
	// #endexample

	// #example OrdinalAll 2
	newCode()
	newCode()
	// #endexample
}

// First (unchecked):
//
//	untouched()
//
// Second:
//
//	oldCode() // want "code block is out of sync with example code"
//	oldCode()
//
// Extra line.
func OrdinalPartial() {} // want OrdinalPartial:"example"

func _() {
	// #example OrdinalPartial 2
	newCode()
	newCode()
	// #endexample
}

// First:
//
//	newCode()
//	newCode()
//
// Second:
//
//	newCode()
//	newCode()
//
// Extra line.
func OrdinalInSync() {} // want OrdinalInSync:"example"

func _() {
	// #example OrdinalInSync 1
	newCode()
	newCode()
	// #endexample

	// #example OrdinalInSync 2
	newCode()
	newCode()
	// #endexample
}

// Only block:
//
//	newCode()
//	newCode()
//
// Extra line.
func OrdinalMissing() {} // want OrdinalMissing:"example"

func _() {
	// #example OrdinalMissing 1
	newCode()
	newCode()
	// #endexample

	// #example OrdinalMissing 2 // want "OrdinalMissing has no corresponding code block"
	newCode()
	newCode()
	// #endexample
}
