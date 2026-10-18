package width

// Wide has a region with 2 lines that are wider than the -max-width of the
// test. The width of the last line includes the tab.
//
//	short()
//	aVeryLongIdentifierCall()
//	if true {
//		shortButNotThat()
//	}
func Wide() {} // want Wide:"example"

func _() {
	// #example Wide // want `example line 2 is 25 columns wide, the limit is 20: aVeryLongIdentifierCall\(\)` `example line 4 is 21 columns wide`
	short()
	aVeryLongIdentifierCall()
	if true {
		shortButNotThat()
	}
	// #endexample
}

// Narrow has a region within the limit.
//
//	short()
func Narrow() {} // want Narrow:"example"

func _() {
	// #example Narrow
	short()
	// #endexample
}

func short() {}

func shortButNotThat() {}

func aVeryLongIdentifierCall() {}
