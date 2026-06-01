// Package pkg contains compilable code snippets used in the README.
// The #region markers are consumed by mdsync to keep the README in sync.
package pkg

// #region basic
// Greet returns a greeting.
//
// Example:
//
//	Greet("world")
func Greet(name string) string {
	return "Hello, " + name + "!"
}

func _() {
	// #example Greet
	Greet("world") // Hello World!
	// #endexample
}

// #endregion

// #region method
// Run executes the task.
//
// Example:
//
//	t := Task{Name: "build"}
//	t.Run()
func (Task) Run() {}

func _() {
	// #example Task.Run
	t := Task{Name: "build"}
	t.Run()
	// #endexample
}

// #endregion

// #region ordinals
// Process does two things.
//
// Parsing:
//
//	Parse(input)
//
// Rendering:
//
//	Render(parsed)
func Process() {}

func _() {
	input := ""
	parsed := result{}

	// #example Process 1
	Parse(input)
	// #endexample

	// #example Process 2
	Render(parsed)
	// #endexample
}

// #endregion

func _() {
	// #region split
	setup()
	// #example Foo
	step1()
	// #endexample
	cleanup()
	// #example Foo
	step2()
	// #endexample
	// #endregion
}

func _() {
	// #region inline
	_ = /* #example MyString */ MyString("hello") /* #endexample */
	// #endregion
}

// #region toplevel
// NewHelper creates a helper.
//
// Example:
//
//	type helper struct{}
//
//	func (helper) Do() {}
func NewHelper() {}

// #example NewHelper
type helper struct{}

func (helper) Do() {}

// #endexample
// #endregion

// Stubs

type (
	Task   struct{ Name string }
	result struct{}
)

// MyString is a custom string type.
//
// Example:
//
//	MyString("hello")
type MyString string

// Foo does something.
//
// Example:
//
//	step1()
//	step2()
func Foo() {}

func setup()   {}
func step1()   {}
func step2()   {}
func cleanup() {}

func Parse(_ string)       {}
func Render(_ interface{}) {}
func Do()                  {}

// DoSomething does something.
//
// Example:
//
//	pkg.DoSomething()
func DoSomething() {}
