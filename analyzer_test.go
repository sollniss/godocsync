package godocsync_test

import (
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sollniss/godocsync"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/packages"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, godocsync.Analyzer, "basic")
}

func TestInline(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, godocsync.Analyzer, "inline")
}

func TestMultiFile(t *testing.T) {
	testdata := analysistest.TestData()
	pkgDir := filepath.Join(testdata, "src", "multifile")

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes |
			packages.NeedImports | packages.NeedDeps,
		Tests: true,
		Dir:   pkgDir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Find the base package (with internal test files) and external test package.
	var basePkg *packages.Package
	var extTestPkg *packages.Package
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.Name, "_test") {
			extTestPkg = pkg
		} else if len(pkg.Syntax) > 1 {
			// The test-augmented package has source + internal test files
			basePkg = pkg
		}
	}
	if basePkg == nil {
		// Fallback: use any non-_test package
		for _, pkg := range pkgs {
			if !strings.HasSuffix(pkg.Name, "_test") {
				basePkg = pkg
				break
			}
		}
	}
	if basePkg == nil {
		t.Fatal("could not find base package")
	}
	if extTestPkg == nil {
		t.Fatal("could not find external test package")
	}

	// Phase 1: Run analyzer on the base package to export facts and collect diagnostics.
	type factEntry struct {
		typ  reflect.Type
		fact analysis.Fact
	}
	factStore := make(map[types.Object][]factEntry)

	var baseDiags []analysis.Diagnostic
	basePass := &analysis.Pass{
		Analyzer:   godocsync.Analyzer,
		Fset:       basePkg.Fset,
		Files:      basePkg.Syntax,
		Pkg:        basePkg.Types,
		TypesInfo:  basePkg.TypesInfo,
		TypesSizes: basePkg.TypesSizes,
		Report: func(d analysis.Diagnostic) {
			baseDiags = append(baseDiags, d)
		},
		ExportObjectFact: func(obj types.Object, fact analysis.Fact) {
			factStore[obj] = append(factStore[obj], factEntry{
				typ:  reflect.TypeOf(fact),
				fact: fact,
			})
		},
		ImportObjectFact: func(obj types.Object, fact analysis.Fact) bool {
			return false
		},
	}

	if _, err := basePass.Analyzer.Run(basePass); err != nil {
		t.Fatal(err)
	}

	// Phase 2: Run analyzer on the external test package, importing facts.
	var extDiags []analysis.Diagnostic
	testPass := &analysis.Pass{
		Analyzer:   godocsync.Analyzer,
		Fset:       extTestPkg.Fset,
		Files:      extTestPkg.Syntax,
		Pkg:        extTestPkg.Types,
		TypesInfo:  extTestPkg.TypesInfo,
		TypesSizes: extTestPkg.TypesSizes,
		Report: func(d analysis.Diagnostic) {
			extDiags = append(extDiags, d)
		},
		ExportObjectFact: func(obj types.Object, fact analysis.Fact) {},
		ImportObjectFact: func(obj types.Object, fact analysis.Fact) bool {
			entries, ok := factStore[obj]
			if !ok {
				return false
			}
			wantType := reflect.TypeOf(fact)
			for _, e := range entries {
				if e.typ == wantType {
					reflect.ValueOf(fact).Elem().Set(reflect.ValueOf(e.fact).Elem())
					return true
				}
			}
			return false
		},
	}

	if _, err := testPass.Analyzer.Run(testPass); err != nil {
		t.Fatal(err)
	}

	// Collect all diagnostics.
	allDiags := append(baseDiags, extDiags...)

	// Verify diagnostics were reported.
	if len(allDiags) != 3 {
		for i, d := range allDiags {
			t.Logf("diag[%d]: %s at %s", i, d.Message, basePass.Fset.Position(d.Pos))
		}
		t.Fatalf("expected 3 diagnostics, got %d", len(allDiags))
	}

	// All diagnostics should be "code block is out of sync" on multi_file.go.
	for _, d := range allDiags {
		if !strings.Contains(d.Message, "code block is out of sync") {
			t.Fatalf("unexpected message: %s", d.Message)
		}
		pos := basePass.Fset.Position(d.Pos)
		if filepath.Base(pos.Filename) != "multi_file.go" {
			t.Fatalf("expected diagnostic in multi_file.go, got %s", pos.Filename)
		}
	}

	// Apply all suggested fixes to multi_file.go.
	srcFile := filepath.Join(pkgDir, "multi_file.go")
	src, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatal(err)
	}

	// Collect all text edits from all diagnostics.
	var allEdits []analysis.TextEdit
	for _, d := range allDiags {
		if len(d.SuggestedFixes) != 1 {
			t.Fatalf("expected 1 suggested fix, got %d for: %s", len(d.SuggestedFixes), d.Message)
		}
		allEdits = append(allEdits, d.SuggestedFixes[0].TextEdits...)
	}

	// Sort edits by position (descending) so we can apply in reverse order.
	sort.Slice(allEdits, func(i, j int) bool {
		return allEdits[i].Pos > allEdits[j].Pos
	})

	result := append([]byte(nil), src...)
	for _, edit := range allEdits {
		start := basePass.Fset.Position(edit.Pos).Offset
		end := basePass.Fset.Position(edit.End).Offset
		tmp := make([]byte, 0, len(result)-(end-start)+len(edit.NewText))
		tmp = append(tmp, result[:start]...)
		tmp = append(tmp, edit.NewText...)
		tmp = append(tmp, result[end:]...)
		result = tmp
	}

	goldenPath := filepath.Join(pkgDir, "multi_file.go.golden")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(result) != string(golden) {
		t.Errorf("fix result does not match golden:\n--- got ---\n%s\n--- want ---\n%s", result, golden)
	}
}

func TestAnalyzer_CRLF(t *testing.T) {
	// Source file with \r\n line endings everywhere.
	// The doc comment has a code block that is out of sync with the marker.
	source := "package crlf\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\r\n" +
		"//\toldCode() // want \"code block is out of sync with example code\"\r\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tnewCode()\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n" +
		"\r\n" +
		"func newCode() {}\r\n"

	// After the fix, the code block region (//\r\n//\toldCode()...\r\n) is replaced
	// with //\n//\tnewCode()\n. The rest of the file keeps \r\n line endings.
	golden := "package crlf\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\n" +
		"//\tnewCode()\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tnewCode()\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n" +
		"\r\n" +
		"func newCode() {}\r\n"

	dir, cleanup, err := analysistest.WriteFiles(map[string]string{
		"crlf/go.mod":         "module crlf\n\ngo 1.25\n",
		"crlf/crlf.go":        source,
		"crlf/crlf.go.golden": golden,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	analysistest.RunWithSuggestedFixes(t, dir, godocsync.Analyzer, "crlf")
}

func TestAnalyzer_LFFileWithCRLFBacktickString(t *testing.T) {
	// LF source file containing a backtick string with \r\n inside it.
	// This tests that \r in raw strings doesn't corrupt positions of subsequent
	// statements or the generated fix.
	source := "package backtick\n" +
		"\n" +
		"// Func does something.\n" +
		"//\n" +
		"//\toldCode() // want \"code block is out of sync with example code\"\n" +
		"func Func() {} // want Func:\"example\"\n" +
		"\n" +
		"func _() {\n" +
		"\t// #example Func\n" +
		"\tx := `hello\r\nworld`\n" +
		"\t_ = x\n" +
		"\t// #endexample\n" +
		"}\n"

	// After the fix, the code block is replaced with the formatted marker content.
	// format.Node strips \r from raw string values (per Go spec), producing:
	//   x := `hello
	//   world`
	// Combined with the _ = x statement.
	golden := "package backtick\n" +
		"\n" +
		"// Func does something.\n" +
		"//\n" +
		"//\tx := `hello\n" +
		"//\tworld`\n" +
		"//\t_ = x\n" +
		"func Func() {} // want Func:\"example\"\n" +
		"\n" +
		"func _() {\n" +
		"\t// #example Func\n" +
		"\tx := `hello\r\nworld`\n" +
		"\t_ = x\n" +
		"\t// #endexample\n" +
		"}\n"

	dir, cleanup, err := analysistest.WriteFiles(map[string]string{
		"backtick/go.mod":             "module backtick\n\ngo 1.25\n",
		"backtick/backtick.go":        source,
		"backtick/backtick.go.golden": golden,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	analysistest.RunWithSuggestedFixes(t, dir, godocsync.Analyzer, "backtick")
}

func TestAnalyzer_CRLFWithCRLFBacktickString(t *testing.T) {
	// CRLF source file that also has a backtick string with \r\n inside.
	// This is the worst case: line endings AND raw string content both have \r.
	source := "package crlfbt\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\r\n" +
		"//\toldCode() // want \"code block is out of sync with example code\"\r\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tx := `hello\r\nworld`\r\n" +
		"\t_ = x\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n"

	// After fix: code block replaced with rendered marker content (using \n).
	// Rest of file keeps \r\n.
	golden := "package crlfbt\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\n" +
		"//\tx := `hello\n" +
		"//\tworld`\n" +
		"//\t_ = x\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tx := `hello\r\nworld`\r\n" +
		"\t_ = x\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n"

	dir, cleanup, err := analysistest.WriteFiles(map[string]string{
		"crlfbt/go.mod":           "module crlfbt\n\ngo 1.25\n",
		"crlfbt/crlfbt.go":        source,
		"crlfbt/crlfbt.go.golden": golden,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	analysistest.RunWithSuggestedFixes(t, dir, godocsync.Analyzer, "crlfbt")
}

func TestAnalyzer_CRLFWithLFBacktickString(t *testing.T) {
	// CRLF file where a backtick string contains only \n (no \r).
	// This is the inverse case: file uses \r\n but the raw string has plain \n.
	source := "package crlflf\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\r\n" +
		"//\toldCode() // want \"code block is out of sync with example code\"\r\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tx := `hello\nworld`\r\n" +
		"\t_ = x\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n"

	// After fix: code block replaced. The backtick string has \n only, so
	// format.Node preserves it as-is (no \r to strip).
	golden := "package crlflf\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\n" +
		"//\tx := `hello\n" +
		"//\tworld`\n" +
		"//\t_ = x\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tx := `hello\nworld`\r\n" +
		"\t_ = x\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n"

	dir, cleanup, err := analysistest.WriteFiles(map[string]string{
		"crlflf/go.mod":           "module crlflf\n\ngo 1.25\n",
		"crlflf/crlflf.go":        source,
		"crlflf/crlflf.go.golden": golden,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	analysistest.RunWithSuggestedFixes(t, dir, godocsync.Analyzer, "crlflf")
}

func TestAnalyzer_CRLFWithLFBacktickBeforeMarkers(t *testing.T) {
	// CRLF file where a backtick string with \n appears BEFORE the doc comment
	// and markers. Tests that \n-only raw strings in a \r\n file don't shift
	// positions of subsequent comments.
	source := "package crlflfbefore\r\n" +
		"\r\n" +
		"var s = `first\nsecond\nthird`\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\r\n" +
		"//\toldCode() // want \"code block is out of sync with example code\"\r\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tnewCode()\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n" +
		"\r\n" +
		"func newCode() {}\r\n"

	golden := "package crlflfbefore\r\n" +
		"\r\n" +
		"var s = `first\nsecond\nthird`\r\n" +
		"\r\n" +
		"// Func does something.\r\n" +
		"//\n" +
		"//\tnewCode()\n" +
		"func Func() {} // want Func:\"example\"\r\n" +
		"\r\n" +
		"func _() {\r\n" +
		"\t// #example Func\r\n" +
		"\tnewCode()\r\n" +
		"\t// #endexample\r\n" +
		"}\r\n" +
		"\r\n" +
		"func newCode() {}\r\n"

	dir, cleanup, err := analysistest.WriteFiles(map[string]string{
		"crlflfbefore/go.mod":                 "module crlflfbefore\n\ngo 1.25\n",
		"crlflfbefore/crlflfbefore.go":        source,
		"crlflfbefore/crlflfbefore.go.golden": golden,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	analysistest.RunWithSuggestedFixes(t, dir, godocsync.Analyzer, "crlflfbefore")
}

func TestAnalyzer_MarkerDocLeak(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, testdata, godocsync.Analyzer, "docleaks")
}
