package inline

// OrdinalAll has two code blocks, both out of sync.
//
// First:
//
//	oldCode() // want "code block is out of sync with example code"
//
// Second:
//
//	oldCode() // want "code block is out of sync with example code"
func OrdinalAll() {} // want OrdinalAll:"example"

// OrdinalMissing has one code block but ordinal 2 is also specified.
//
// Only block:
//
//	oldCode() // want "code block is out of sync with example code"
func OrdinalMissing() {} // want OrdinalMissing:"example"

// OrdinalPartial has two code blocks; only ordinal 2 is specified.
//
// First (unchecked):
//
//	untouched()
//
// Second:
//
//	oldCode() // want "code block is out of sync with example code"
func OrdinalPartial() {} // want OrdinalPartial:"example"

func _() {
	_ = /* #example OrdinalAll 1 */ MyString("first")  /* #endexample */
	_ = /* #example OrdinalAll 2 */ MyString("second") /* #endexample */

	_ = /* #example OrdinalMissing 1 */ MyString("exists") /* #endexample */
	_ = /* #example OrdinalMissing 2 */ MyString("nope")   /* #endexample */ // want `OrdinalMissing has no corresponding code block`

	_ = /* #example OrdinalPartial 2 */ MyString("updated") /* #endexample */
}

func newCode() {}
