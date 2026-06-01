package godocsync

import (
	"go/ast"
	"go/format"
	"go/token"
	"strconv"
	"strings"
)

// commentContent extracts the text between comment delimiters.
//   - For "// text" it returns ("text", false, true).
//   - For "/* text */" it returns ("text", true, true).
//   - Returns ("", false, false) if the format is unrecognized.
func commentContent(text string) (content string, isBlock bool, ok bool) {
	if len(text) >= 2 && text[0] == '/' && text[1] == '/' {
		return text[2:], false, true
	}
	if len(text) >= 4 &&
		text[0] == '/' &&
		text[1] == '*' &&
		text[len(text)-2] == '*' &&
		text[len(text)-1] == '/' {
		return text[2 : len(text)-2], true, true
	}
	return "", false, false
}

// markerKind distinguishes open, close, and non-marker comments.
type markerKind int

const (
	markerNone  markerKind = iota
	markerOpen             // #example <key> [<ordinal>]
	markerClose            // #endexample
)

// classifyMarker determines if a comment is an #example or #endexample marker.
// For open markers, returns (markerOpen, key, ordinal, rawOrdinal).
// ordinal is -1 if an invalid ordinal was provided, 1 if none specified.
// For close markers, returns (markerClose, "", 0, "").
// Supports both // line comments and /* */ block comments.
func classifyMarker(text string) (kind markerKind, key string, ordinal int, rawOrdinal string) {
	s, isBlock, valid := commentContent(text)
	if !valid {
		return markerNone, "", 0, ""
	}
	s = trimLeftSpace(s)

	// Check #endexample (must be checked before #example since both start with '#e').
	if len(s) >= 11 && s[:11] == "#endexample" {
		rest := trimLeftSpace(s[11:])
		if isBlock {
			if len(rest) == 0 {
				return markerClose, "", 0, ""
			}
			return markerNone, "", 0, ""
		}
		// Line comment: must be empty or a trailing // comment.
		if len(rest) == 0 || (len(rest) >= 2 && rest[0] == '/' && rest[1] == '/') {
			return markerClose, "", 0, ""
		}
		return markerNone, "", 0, ""
	}

	// Check #example.
	if len(s) < 8 || s[:8] != "#example" {
		return markerNone, "", 0, ""
	}
	s = s[8:]
	// Must be followed by whitespace (not end of string, not another letter).
	if len(s) == 0 || (s[0] != ' ' && s[0] != '\t') {
		return markerNone, "", 0, ""
	}
	s = trimLeftSpace(s)
	// Extract key (non-whitespace run).
	keyEnd := 0
	for keyEnd < len(s) && s[keyEnd] != ' ' && s[keyEnd] != '\t' {
		keyEnd++
	}
	if keyEnd == 0 {
		return markerNone, "", 0, ""
	}
	key = s[:keyEnd]
	s = trimLeftSpace(s[keyEnd:])
	// Optional ordinal (digits).
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		numEnd := 0
		for numEnd < len(s) && s[numEnd] >= '0' && s[numEnd] <= '9' {
			numEnd++
		}
		ordStr := s[:numEnd]
		s = trimLeftSpace(s[numEnd:])
		// Remaining must be empty or (for line comments) a trailing // comment.
		if len(s) > 0 && !isBlock && (len(s) < 2 || s[0] != '/' || s[1] != '/') {
			return markerNone, "", 0, ""
		}
		if len(s) > 0 && isBlock {
			return markerNone, "", 0, ""
		}
		ord, err := strconv.Atoi(ordStr)
		if err != nil || ord < 1 {
			return markerOpen, key, -1, ordStr
		}
		return markerOpen, key, ord, ""
	}
	// No ordinal. Remaining must be empty or (for line comments) a trailing // comment.
	if len(s) > 0 && !isBlock && (len(s) < 2 || s[0] != '/' || s[1] != '/') {
		return markerNone, "", 0, ""
	}
	if len(s) > 0 && isBlock {
		return markerNone, "", 0, ""
	}
	return markerOpen, key, 1, ""
}

// trimLeftSpace trims leading spaces and tabs (not all unicode whitespace).
func trimLeftSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

// symbolKey uniquely identifies an exported declaration within a package.
// receiver is empty for package-level funcs/types, the unstarred type name for methods.
// name is the func/method/type name.
type symbolKey struct {
	receiver string
	name     string
}

func (sk symbolKey) String() string {
	if sk.receiver == "" {
		return sk.name
	}
	return sk.receiver + "." + sk.name
}

// piece is one captured region from a test file.
type piece struct {
	key     symbolKey
	ordinal int       // which example on this symbol (1-based)
	file    string    // source filename
	line    int       // source line number
	text    string    // captured content (newline-delimited, formatted via go/format)
	pos     token.Pos // position of the #example marker
}

// extractMarkers scans all files for #example / #endexample regions and returns
// the resulting pieces and per-file exclusion regions. readFile is used to access
// source bytes for inline (/* */) markers.
func extractMarkers(
	fset *token.FileSet,
	files []*ast.File,
	readFile func(string) ([]byte, error),
) ([]piece, map[*ast.File][]posRange, []markerError) {
	var pieces []piece
	var errs []markerError
	regions := make(map[*ast.File][]posRange)

	for _, f := range files {
		ps, rs, es := extractFileMarkers(fset, f, readFile)
		pieces = append(pieces, ps...)
		if len(rs) > 0 {
			regions[f] = rs
		}
		errs = append(errs, es...)
	}

	return pieces, regions, errs
}

// markerError represents a problem with a marker that should be reported as a diagnostic.
type markerError struct {
	Pos     token.Pos
	Message string
}

type openMarker struct {
	comment *ast.Comment
	key     string
	ordinal int
	inline  bool // true for /* */ block comment markers
}

func extractFileMarkers(
	fset *token.FileSet,
	f *ast.File,
	readFile func(string) ([]byte, error),
) ([]piece, []posRange, []markerError) {
	// Collect all marker comments in source order.
	type markerComment struct {
		c          *ast.Comment
		kind       markerKind
		inline     bool // true for /* */ block comment markers
		key        string
		ordinal    int
		rawOrdinal string
	}

	var markers []markerComment
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			// Fast-path: skip comments that can't possibly be markers.
			if !strings.Contains(c.Text, "#") {
				continue
			}
			kind, key, ordinal, rawOrd := classifyMarker(c.Text)
			if kind == markerNone {
				continue
			}
			inline := len(c.Text) >= 4 && c.Text[1] == '*'
			markers = append(markers, markerComment{
				c:          c,
				kind:       kind,
				inline:     inline,
				key:        key,
				ordinal:    ordinal,
				rawOrdinal: rawOrd,
			})
		}
	}

	if len(markers) == 0 {
		return nil, nil, nil
	}

	// Arena builder: all formatted text is written here, then sliced into
	// per-piece substrings at the end. This avoids one allocation per marker pair.
	var arena strings.Builder
	type textSpan struct{ start, end int }

	var pieces []piece
	var spans []textSpan
	var regions []posRange
	var errs []markerError
	var current *openMarker

	// Region tracking (independent of current pointer logic).
	var regionOpenPos token.Pos
	var inRegion bool

	for _, mc := range markers {
		if mc.kind == markerOpen {
			// Track region for exclusion (independent of ordinal validity).
			regionOpenPos = mc.c.Pos()
			inRegion = true

			if current != nil {
				errs = append(errs, markerError{
					Pos: mc.c.Pos(),
					Message: "#example opened without closing previous #endexample (opened at line " +
						strconv.Itoa(fset.Position(current.comment.Pos()).Line) + ")",
				})
				current = nil
			}
			if mc.ordinal < 0 {
				errs = append(errs, markerError{
					Pos:     mc.c.Pos(),
					Message: "invalid ordinal " + strconv.Quote(mc.rawOrdinal),
				})
				continue
			}
			current = &openMarker{comment: mc.c, key: mc.key, ordinal: mc.ordinal, inline: mc.inline}
			continue
		}

		// Close marker.
		if inRegion {
			regions = append(regions, posRange{start: regionOpenPos, end: mc.c.End()})
			inRegion = false
		}

		if current == nil {
			errs = append(errs, markerError{
				Pos:     mc.c.Pos(),
				Message: "#endexample without matching #example",
			})
			continue
		}

		// Matched pair found.
		start := current.comment.End()
		end := mc.c.Pos()
		spanStart := arena.Len()
		if current.inline {
			// Inline markers: extract raw source text between them.
			appendInlineText(&arena, fset, f, start, end, readFile)
		} else {
			// Line markers: find AST nodes between them.
			stmts := findStatementsBetween(f, start, end)
			if len(stmts) > 0 {
				appendFormattedStatements(&arena, fset, stmts, start)
			} else {
				decls := findDeclsBetween(f.Decls, start, end)
				appendFormattedDecls(&arena, fset, decls, start)
			}
		}

		sk := parseMarkerKey(current.key)
		p := fset.Position(current.comment.Pos())
		pieces = append(pieces, piece{
			key:     sk,
			ordinal: current.ordinal,
			file:    p.Filename,
			line:    p.Line,
			pos:     current.comment.Pos(),
		})
		spans = append(spans, textSpan{start: spanStart, end: arena.Len()})
		current = nil
	}

	if current != nil {
		errs = append(errs, markerError{
			Pos:     current.comment.Pos(),
			Message: "#example " + strconv.Quote(current.key) + " never closed",
		})
	}

	// Finalize arena: one allocation for the complete string,
	// then each piece gets a zero-copy substring.
	if len(spans) > 0 {
		arenaStr := arena.String()
		for i, sp := range spans {
			pieces[i].text = arenaStr[sp.start:sp.end]
		}
	}

	return pieces, regions, errs
}

// findStatementsBetween finds the innermost BlockStmt enclosing [start, end]
// and returns its direct children fully contained in that range.
// It first locates the enclosing FuncDecl to avoid walking unrelated subtrees.
func findStatementsBetween(f *ast.File, start, end token.Pos) []ast.Stmt {
	// Find the function declaration whose body contains [start, end].
	var body *ast.BlockStmt
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Body.Lbrace < start && end < fn.Body.Rbrace {
			body = fn.Body
			break
		}
	}
	if body == nil {
		return nil
	}

	// Walk only the enclosing function body to find the innermost block.
	enclosing := body
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// Prune subtrees that cannot contain [start, end].
		if n.End() <= start || n.Pos() >= end {
			return false
		}
		if block, ok := n.(*ast.BlockStmt); ok && block.Lbrace < start && end < block.Rbrace {
			enclosing = block
		}
		return true
	})

	result := make([]ast.Stmt, 0, 4)
	for _, stmt := range enclosing.List {
		if stmt.Pos() >= start && stmt.End() <= end {
			result = append(result, stmt)
		}
	}
	return result
}

// appendFormattedNodes formats AST nodes using go/format and appends the result
// to dst as newline-delimited text.
// Blank lines between nodes are preserved based on their source positions.
// stripDoc is called before formatting each node to remove any doc comment
// that is actually the #example marker comment (identified by ending at or before start).
func appendFormattedNodes[T ast.Node](
	dst *strings.Builder,
	fset *token.FileSet,
	nodes []T,
	start token.Pos,
	stripDoc func(T, token.Pos),
) {
	for i, node := range nodes {
		stripDoc(node, start)
		if i > 0 {
			// Newline to end previous node's last line.
			dst.WriteByte('\n')
			// Preserve blank lines between nodes based on position gap.
			endLine := fset.Position(nodes[i-1].End()).Line
			nextLine := fset.Position(node.Pos()).Line
			for range nextLine - endLine - 1 {
				dst.WriteByte('\n')
			}
		}
		format.Node(dst, fset, node)
	}
}

// appendFormattedStatements formats statements and appends the result to dst.
func appendFormattedStatements(dst *strings.Builder, fset *token.FileSet, stmts []ast.Stmt, start token.Pos) {
	appendFormattedNodes(dst, fset, stmts, start, func(stmt ast.Stmt, start token.Pos) {
		// Strip doc comments that are actually the #example marker comment.
		// The parser attaches the marker as Doc when it immediately precedes
		// a type/var/const declaration.
		if ds, ok := stmt.(*ast.DeclStmt); ok {
			if gd, ok := ds.Decl.(*ast.GenDecl); ok && gd.Doc != nil && gd.Doc.End() <= start {
				gd.Doc = nil
			}
		}
	})
}

// findDeclsBetween finds top-level declarations fully contained between start and end positions.
func findDeclsBetween(decls []ast.Decl, start, end token.Pos) []ast.Decl {
	var result []ast.Decl
	for _, decl := range decls {
		if decl.Pos() >= start && decl.End() <= end {
			result = append(result, decl)
		}
	}
	return result
}

// appendFormattedDecls formats top-level declarations and appends the result to dst.
func appendFormattedDecls(dst *strings.Builder, fset *token.FileSet, decls []ast.Decl, start token.Pos) {
	appendFormattedNodes(dst, fset, decls, start, func(decl ast.Decl, start token.Pos) {
		// Strip doc comments that are actually the #example marker comment.
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Doc != nil && d.Doc.End() <= start {
				d.Doc = nil
			}
		case *ast.FuncDecl:
			if d.Doc != nil && d.Doc.End() <= start {
				d.Doc = nil
			}
		}
	})
}

// appendInlineText extracts raw source text between two positions
// (used for inline /* */ markers), strips common leading indentation,
// and appends the result to dst as newline-delimited text.
func appendInlineText(
	dst *strings.Builder,
	fset *token.FileSet,
	f *ast.File,
	start, end token.Pos,
	readFile func(string) ([]byte, error),
) {
	tf := fset.File(f.Pos())
	if tf == nil {
		return
	}
	filename := tf.Name()
	src, err := readFile(filename)
	if err != nil {
		return
	}
	startOff := tf.Offset(start)
	endOff := tf.Offset(end)
	if startOff >= endOff || endOff > len(src) {
		return
	}
	raw := string(src[startOff:endOff])
	parts := strings.Split(raw, "\n")

	// Trim trailing whitespace from each line.
	for i := range parts {
		parts[i] = strings.TrimRight(parts[i], " \t\r")
	}

	// Remove leading and trailing empty lines.
	for len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return
	}

	// Find minimum leading whitespace across non-empty lines.
	minIndent := -1
	for _, l := range parts {
		if l == "" {
			continue
		}
		indent := 0
		for indent < len(l) && (l[indent] == ' ' || l[indent] == '\t') {
			indent++
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}

	// Strip common indentation prefix and write to dst.
	for i, l := range parts {
		if i > 0 {
			dst.WriteByte('\n')
		}
		if minIndent > 0 && len(l) >= minIndent {
			dst.WriteString(l[minIndent:])
		} else {
			dst.WriteString(l)
		}
	}
}

// posRange represents a position range [start, end].
type posRange struct {
	start, end token.Pos
}

// parseMarkerKey splits a raw marker key into receiver and name.
// "Foo" → ("", "Foo")
// "Foo.Bar" → ("Foo", "Bar")
func parseMarkerKey(raw string) symbolKey {
	if receiver, name, ok := strings.Cut(raw, "."); ok {
		return symbolKey{receiver: receiver, name: name}
	}
	return symbolKey{name: raw}
}
