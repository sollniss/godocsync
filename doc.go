package godocsync

import (
	"go/ast"
	"strings"
)

// codeBlockRegion identifies a code block within a CommentGroup.List slice
// by start and end indices (both inclusive).
type codeBlockRegion struct {
	Start int // index of first comment in block (typically the blank separator)
	End   int // index of last comment in block (inclusive)
}

// findCodeBlocks identifies code block regions in a slice of comments.
//
// A code block region is a maximal contiguous group of blank comment lines
// and code comment lines that contains at least one code line. Trailing blank
// lines (not followed by more code in the same region) are excluded.
func findCodeBlocks(comments []*ast.Comment) []codeBlockRegion {
	// Most doc comments have 1 code block. 2 should cover most cases.
	blocks := make([]codeBlockRegion, 0, 2)
	i := 0
	for i < len(comments) {
		// Skip prose lines.
		if !isBlankComment(comments[i]) && !isCodeComment(comments[i]) {
			i++
			continue
		}

		// Start of a potential code block region.
		regionStart := i
		lastCode := -1
		for i < len(comments) && (isBlankComment(comments[i]) || isCodeComment(comments[i])) {
			if isCodeComment(comments[i]) {
				lastCode = i
			}
			i++
		}

		if lastCode >= 0 {
			blocks = append(blocks, codeBlockRegion{
				Start: regionStart,
				End:   lastCode,
			})
		}
	}
	return blocks
}

// blockText reconstructs the text of a code block region from comment nodes.
// Each comment's Text is followed by a newline. The result includes a trailing
// newline after the last comment.
func blockText(comments []*ast.Comment, region codeBlockRegion) string {
	// Pre-size builder: estimate based on comment count and average length.
	n := region.End - region.Start + 1
	var b strings.Builder
	b.Grow(n * 40) // reasonable estimate for comment lines
	for i := region.Start; i <= region.End; i++ {
		b.WriteString(comments[i].Text)
		b.WriteByte('\n')
	}
	return b.String()
}

// firstCodeComment returns the index (into comments) of the first code comment
// in the given region. Returns region.Start as fallback.
func firstCodeComment(comments []*ast.Comment, region codeBlockRegion) int {
	for i := region.Start; i <= region.End; i++ {
		if isCodeComment(comments[i]) {
			return i
		}
	}
	return region.Start
}

func isBlankComment(c *ast.Comment) bool {
	// Comment text always starts with "//" (guaranteed by Go scanner).
	// Check that the remainder is only spaces/tabs.
	for i := 2; i < len(c.Text); i++ {
		if c.Text[i] != ' ' && c.Text[i] != '\t' {
			return false
		}
	}
	return len(c.Text) >= 2
}

func isCodeComment(c *ast.Comment) bool {
	// Must start with "//" and have at least one more character.
	if len(c.Text) < 3 || c.Text[0] != '/' || c.Text[1] != '/' {
		return false
	}
	// Code lines start with "//\t" or "// \t" (tab after optional single space).
	return c.Text[2] == '\t' || (c.Text[2] == ' ' && len(c.Text) > 3 && c.Text[3] == '\t')
}
