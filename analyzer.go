package godocsync

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ExampleFact is exported for every exported symbol whose doc comment exists.
// If Blocks is non-empty, the doc contains code blocks. If Blocks is empty,
// the doc exists but has no code blocks.
//
// It is also exported as a package fact for the package doc comment so that
// external test packages (pkg_test) can sync "#example package" markers. In
// that case File names the file holding the doc comment: a package has no
// types.Object (and thus no Pos) to recover the file from. File is empty for
// object facts.
type ExampleFact struct {
	File   string
	Blocks []BlockInfo
}

// BlockInfo stores a single code block's text and byte offsets within its file.
type BlockInfo struct {
	Text        string
	StartOffset int // byte offset of first comment in block
	EndOffset   int // byte offset past newline of last comment in block
	DiagOffset  int // byte offset of first code comment (for diagnostic placement)
}

func (*ExampleFact) AFact() {}

func (*ExampleFact) String() string { return "example" }

var Analyzer = &analysis.Analyzer{
	Name:      "godocsync",
	Doc:       "checks that code blocks in doc comments match #example markers in test files",
	Run:       run,
	FactTypes: []analysis.Fact{(*ExampleFact)(nil)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Replace the framework's restricted ReadFile with os.ReadFile so that
	// cross-package suggested fixes (e.g. pkg_test fixing docs in pkg) can
	// be applied by singlechecker's -fix flag.
	pass.ReadFile = os.ReadFile

	pkgName := pass.Pkg.Name()
	isExternalTest := strings.HasSuffix(pkgName, "_test")

	// Extract markers from files in this pass (also computes exclusion regions).
	pieces, regions, markerErrs := extractMarkers(pass.Fset, pass.Files, pass.ReadFile)

	for _, me := range markerErrs {
		pass.Report(analysis.Diagnostic{
			Pos:     me.Pos,
			Message: me.Message,
		})
	}

	// For non-external-test packages, export ExampleFacts for documented symbols.
	var localFacts map[types.Object]*ExampleFact
	var localPkgFact *ExampleFact
	if !isExternalTest {
		localFacts = exportExampleFacts(pass, regions)
		localPkgFact = exportPackageDocFact(pass)
	}

	if len(pieces) == 0 {
		return nil, nil
	}

	examples := buildExamples(pieces)

	if isExternalTest {
		srcPkg := findSelfPackage(pass)
		if srcPkg == nil {
			return nil, nil
		}
		resolveAndReport(pass, examples, func(key symbolKey) (types.Object, *ExampleFact, bool) {
			obj := lookupSymbolIn(srcPkg, key)
			if obj == nil {
				return nil, nil, false
			}
			if !obj.Exported() {
				return nil, nil, true // skip unexported
			}
			var fact ExampleFact
			if !pass.ImportObjectFact(obj, &fact) {
				return obj, nil, false
			}
			return obj, &fact, false
		}, func(block *exampleBlock) {
			var fact ExampleFact
			if !pass.ImportPackageFact(srcPkg, &fact) {
				checkPackageDoc(pass, nil, block)
				return
			}
			checkPackageDoc(pass, &fact, block)
		})
	} else {
		resolveAndReport(pass, examples, func(key symbolKey) (types.Object, *ExampleFact, bool) {
			obj := pass.Pkg.Scope().Lookup(key.name)
			if key.receiver != "" {
				obj = lookupMethod(pass.Pkg, key)
			}
			if obj == nil {
				return nil, nil, false
			}
			return obj, localFacts[obj], false
		}, func(block *exampleBlock) {
			checkPackageDoc(pass, localPkgFact, block)
		})
	}

	return nil, nil
}

// resolveAndReport iterates over examples, resolves each symbol via the
// provided resolver, and reports diagnostics for mismatches.
// handlePackage, if non-nil, handles the special "package" key.
// The resolver returns (obj, fact, skip):
//   - (nil, nil, false): symbol not found
//   - (obj, nil, false): symbol found but has no doc comment
//   - (obj, fact, false): symbol found with fact
//   - (_, _, true): skip this symbol silently
func resolveAndReport(
	pass *analysis.Pass,
	examples map[symbolKey]*exampleBlock,
	resolve func(symbolKey) (types.Object, *ExampleFact, bool),
	handlePackage func(*exampleBlock),
) {
	for key, block := range examples {
		if key.name == "package" && key.receiver == "" {
			if handlePackage != nil {
				handlePackage(block)
			}
			continue
		}

		obj, fact, skip := resolve(key)
		if skip {
			continue
		}
		if obj == nil {
			pass.Report(analysis.Diagnostic{
				Pos:     block.firstPos(),
				Message: "#example references unknown symbol " + strconv.Quote(key.String()),
			})
			continue
		}
		if fact == nil {
			pass.Report(analysis.Diagnostic{
				Pos:     block.firstPos(),
				Message: key.String() + " has no comment",
			})
			continue
		}
		if len(fact.Blocks) == 0 {
			reportMissingBlocks(pass, block, key)
			continue
		}
		checkFactSync(pass, pass.Fset.File(obj.Pos()), fact, block, key)
	}
}

// exportExampleFacts exports an ExampleFact for every symbol whose doc comment exists.
//   - Symbols with code blocks get a populated Blocks slice.
//   - Symbols with a doc but no code blocks get an empty Blocks slice.
//   - Symbols without any doc comment are skipped (no fact exported).
func exportExampleFacts(
	pass *analysis.Pass,
	regions map[*ast.File][]posRange,
) map[types.Object]*ExampleFact {
	facts := make(map[types.Object]*ExampleFact)
	for _, f := range pass.Files {
		filename := pass.Fset.Position(f.Pos()).Filename
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}

		excluded := regions[f]
		ri := 0 // linear scan index into excluded regions
		for _, decl := range f.Decls {
			pos := decl.Pos()
			// Advance past regions that end before this declaration.
			for ri < len(excluded) && excluded[ri].end < pos {
				ri++
			}
			if ri < len(excluded) && pos >= excluded[ri].start {
				continue
			}
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name == nil {
					continue
				}
				obj := pass.TypesInfo.ObjectOf(d.Name)
				if obj == nil {
					continue
				}
				if fact := buildExampleFact(pass.Fset, d.Doc); fact != nil {
					pass.ExportObjectFact(obj, fact)
					facts[obj] = fact
				}

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name == nil {
							continue
						}
						obj := pass.TypesInfo.ObjectOf(s.Name)
						if obj == nil {
							continue
						}
						doc := s.Doc
						if doc == nil {
							doc = d.Doc
						}
						if fact := buildExampleFact(pass.Fset, doc); fact != nil {
							pass.ExportObjectFact(obj, fact)
							facts[obj] = fact
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if name == nil {
								continue
							}
							obj := pass.TypesInfo.ObjectOf(name)
							if obj == nil {
								continue
							}
							doc := s.Doc
							if doc == nil {
								doc = d.Doc
							}
							if fact := buildExampleFact(pass.Fset, doc); fact != nil {
								pass.ExportObjectFact(obj, fact)
								facts[obj] = fact
							}
						}
					}
				}
			}
		}
	}
	return facts
}

// buildExampleFact returns an ExampleFact for a doc comment.
//   - Returns nil if doc is nil (no fact should be exported).
//   - Returns a fact with empty Blocks if the doc has no code blocks.
func buildExampleFact(fset *token.FileSet, doc *ast.CommentGroup) *ExampleFact {
	if doc == nil {
		return nil
	}

	comments := doc.List
	blocks := findCodeBlocks(comments)

	fact := &ExampleFact{}
	if len(blocks) == 0 {
		return fact
	}

	tf := fset.File(doc.Pos())
	if tf == nil {
		return fact
	}

	for _, region := range blocks {
		fact.Blocks = append(fact.Blocks, BlockInfo{
			Text:        blockText(comments, region),
			StartOffset: tf.Offset(comments[region.Start].Pos()),
			EndOffset:   tf.Offset(commentEndOfLine(tf, comments[region.End])),
			DiagOffset:  tf.Offset(comments[firstCodeComment(comments, region)].Pos()),
		})
	}
	return fact
}

// exportPackageDocFact exports an ExampleFact as a package fact if the package
// has a doc comment. Only non-test files are considered: a comment above the
// package clause of a _test.go file is not the package doc. Returns nil when
// no package doc exists (no fact is exported), mirroring buildExampleFact.
func exportPackageDocFact(pass *analysis.Pass) *ExampleFact {
	for _, f := range pass.Files {
		if f.Doc == nil {
			continue
		}
		filename := pass.Fset.Position(f.Pos()).Filename
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		fact := buildExampleFact(pass.Fset, f.Doc)
		if fact == nil {
			continue
		}
		fact.File = filename
		pass.ExportPackageFact(fact)
		return fact
	}
	return nil
}

// checkPackageDoc handles syncing code blocks in the package-level doc comment.
// fact is nil when the package has no doc comment.
func checkPackageDoc(pass *analysis.Pass, fact *ExampleFact, block *exampleBlock) {
	key := symbolKey{name: "package"}
	if fact == nil {
		pass.Report(analysis.Diagnostic{
			Pos:     block.firstPos(),
			Message: key.String() + " has no comment",
		})
		return
	}
	if len(fact.Blocks) == 0 {
		reportMissingBlocks(pass, block, key)
		return
	}
	checkFactSync(pass, tokenFileByName(pass.Fset, fact.File), fact, block, key)
}

// tokenFileByName finds the token.File for filename in fset, or nil.
// A package doc has no Pos to recover its file from, so the package fact
// carries the filename and the file is looked up here. This relies on the
// file set being shared with the package that exported the fact, the same
// assumption the object-fact path makes via obj.Pos().
func tokenFileByName(fset *token.FileSet, filename string) *token.File {
	var found *token.File
	fset.Iterate(func(tf *token.File) bool {
		if tf.Name() == filename {
			found = tf
			return false
		}
		return true
	})
	return found
}

// reportMissingBlocks reports a "no corresponding code block" diagnostic for
// every non-nil ordinal in block.
func reportMissingBlocks(pass *analysis.Pass, block *exampleBlock, key symbolKey) {
	for i, text := range block.ordinals {
		if block.ordinalPositions[i] == 0 || len(text) == 0 {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     block.ordinalPositions[i],
			Message: key.String() + " has no corresponding code block",
		})
	}
}

// checkFactSync checks sync between code blocks (described by fact data) and
// marker content. Used for both same-package and cross-package analysis.
func checkFactSync(
	pass *analysis.Pass,
	tf *token.File,
	fact *ExampleFact,
	block *exampleBlock,
	key symbolKey,
) {
	if tf == nil {
		return
	}

	nBlocks := len(fact.Blocks)

	for i, text := range block.ordinals {
		if block.ordinalPositions[i] == 0 || len(text) == 0 {
			continue
		}
		if i >= nBlocks {
			pass.Report(analysis.Diagnostic{
				Pos:     block.ordinalPositions[i],
				Message: key.String() + " has no corresponding code block",
			})
			continue
		}
		if ordinalMatchesFact(text, fact.Blocks[i].Text) {
			continue
		}
		// Point at the marker of this ordinal, not at the first marker of the
		// symbol: each ordinal has its own region.
		markerPos := pass.Fset.Position(block.ordinalPositions[i])
		sourceRef := filepath.Base(markerPos.Filename) + ":" + strconv.Itoa(markerPos.Line)
		rendered := renderOrdinal(text)
		diagPos := tf.Pos(fact.Blocks[i].DiagOffset)
		pass.Report(analysis.Diagnostic{
			Pos:     diagPos,
			Message: "code block is out of sync with example code (source: " + sourceRef + ")",
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "update code block",
					TextEdits: []analysis.TextEdit{{
						Pos:     tf.Pos(fact.Blocks[i].StartOffset),
						End:     tf.Pos(fact.Blocks[i].EndOffset),
						NewText: []byte(rendered),
					}},
				},
			},
		})
	}
}

// lookupSymbolIn resolves a symbolKey in the given package.
func lookupSymbolIn(pkg *types.Package, key symbolKey) types.Object {
	if key.receiver == "" {
		return pkg.Scope().Lookup(key.name)
	}
	return lookupMethod(pkg, key)
}

// lookupMethod resolves a method symbolKey (receiver.name) in the given package.
func lookupMethod(pkg *types.Package, key symbolKey) types.Object {
	typObj := pkg.Scope().Lookup(key.receiver)
	if typObj == nil {
		return nil
	}
	typName, ok := typObj.(*types.TypeName)
	if !ok {
		return nil
	}
	named, ok := typName.Type().(*types.Named)
	if !ok {
		return nil
	}
	obj, _, _ := types.LookupFieldOrMethod(named, true, pkg, key.name)
	return obj
}

// findSelfPackage finds the source package imported by an external test package.
func findSelfPackage(pass *analysis.Pass) *types.Package {
	baseName := strings.TrimSuffix(pass.Pkg.Name(), "_test")
	for _, imp := range pass.Pkg.Imports() {
		if imp.Name() == baseName {
			return imp
		}
	}
	return nil
}

// commentEndOfLine returns the token.Pos at the start of the line AFTER the
// given comment. This correctly handles both \n and \r\n line endings by
// using the token.File's line table rather than arithmetic on comment.End().
//
// Background: ast.Comment.End() is computed as Pos()+len(Text), but the
// scanner strips \r from comment Text. On files with \r\n endings, End()
// points at the \r byte, making End()+1 incorrect. Using LineStart avoids
// this issue entirely.
func commentEndOfLine(tf *token.File, c *ast.Comment) token.Pos {
	line := tf.Line(c.End())
	if line < tf.LineCount() {
		return tf.LineStart(line + 1)
	}
	// Last line of file (no trailing newline).
	return tf.Pos(tf.Size())
}
