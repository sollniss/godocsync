package inline

// MyString is a custom string type.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
type MyString string // want MyString:"example"

// Convert does something.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
func Convert() {} // want Convert:"example"

// Multi does something.
//
// Example:
//
//	oldCode() // want "code block is out of sync with example code"
func Multi() {} // want Multi:"example"

func _() {
	// Basic inline marker: cherry-pick a type conversion expression.
	_ = /* #example MyString */ MyString("hello") /* #endexample */

	// Inline marker on a function call.
	_ = /* #example Convert */ string(MyString("world")) /* #endexample */

	// Inline marker with multiple expressions (multi-line between markers).
	_ = /* #example Multi */ struct {
		X int
		Y int
	}{X: 1, Y: 2} /* #endexample */
}
