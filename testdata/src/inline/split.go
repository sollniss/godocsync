package inline

// Example:
//
//	oldCode() // want "code block is out of sync with example code"
func Split() {} // want Split:"example"

func _() {
	_ = /* #example Split */ MyString("one") /* #endexample */
	_ = MyString("skipped")

	_ = /* #example Split */ MyString("two") /* #endexample */
}
