# godocsync

A Go analysis tool that keeps code blocks in doc comments in sync with actual code. Mark regions of code with `#example` / `#endexample` comments, and godocsync will report (and auto-fix) when the corresponding doc comment code block drifts out of date.

## Install

```sh
go install github.com/sollniss/godocsync/cmd/godocsync@latest
```

## Usage

```sh
godocsync ./...        # report out-of-sync code blocks
godocsync -fix ./...   # auto-fix doc comments
```

## Examples

### Basic

Mark code with `#example <Symbol>` and godocsync ensures the doc comment stays updated:

<!-- MDSYNC:go from:examples/examples.go region:basic -->

```go
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
```

If the code between the markers changes, godocsync reports that the doc comment code block is out of sync and suggests a fix.

### Methods

Use `Receiver.Method` syntax:

<!-- MDSYNC:go from:examples/examples.go region:method -->

```go
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
```

### Package doc

Use the special key `package`:

<!-- MDSYNC:go from:examples/examples_test.go region:packagedoc -->

```go
// #example package
pkg.Do()
// #endexample
```

### Multiple code blocks (ordinals)

When a doc comment has multiple code blocks, use ordinals to target each one:

<!-- MDSYNC:go from:examples/examples.go region:ordinals -->

```go
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
```

Omit the ordinal (defaults to 1) or specify only the ones you want to track. Unspecified ordinals are left alone.

### Split markers

Multiple markers for the same symbol and ordinal are concatenated in source order:

<!-- MDSYNC:go from:examples/examples.go region:split -->

```go
setup()
// #example Foo
step1()
// #endexample
cleanup()
// #example Foo
step2()
// #endexample
```

This produces a code block containing:

```
step1()
step2()
```

### Inline markers

Use `/* #example */` and `/* #endexample */` block comments to capture expressions:

<!-- MDSYNC:go from:examples/examples.go region:inline -->

```go
_ = /* #example MyString */ MyString("hello") /* #endexample */
```

This captures the raw text `MyString("hello")` as the code block content.

### Cross-package (external test files)

Markers in `_test.go` files with an external test package (`package foo_test`) work across package boundaries:

<!-- MDSYNC:go from:examples/examples_test.go region:crosspackage -->

```go
func TestExample(t *testing.T) {
  // #example DoSomething
  pkg.DoSomething()
  // #endexample
}
```

### Top-level declarations

Markers can also wrap top-level declarations (functions, types, etc.):

<!-- MDSYNC:go from:examples/examples.go region:toplevel -->

```go
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
```

Note: Go formatting enforces an empty line after a top-level declaration; these are not rendered in the documentation code block.

## How it works

1. Scans source files for `#example` / `#endexample` marker pairs
2. Formats the captured code using `go/format`
3. Compares it against code blocks in the referenced symbol's doc comment
4. Reports a diagnostic with a suggested fix when they differ

Run with `-fix` to apply all suggested fixes automatically.
