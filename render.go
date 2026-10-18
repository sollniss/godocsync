package godocsync

import (
	"cmp"
	"go/token"
	"slices"
	"strings"
)

// exampleBlock is the rendered content for a code block in a doc comment.
// ordinals is indexed by ordinal-1 (slot 0 = ordinal 1, slot 1 = ordinal 2, etc.).
// An empty entry with zero position means no marker was specified for that ordinal.
type exampleBlock struct {
	ordinals         []string    // per-ordinal newline-delimited text ("" + zero pos = not specified)
	ordinalPositions []token.Pos // parallel to ordinals (zero = not specified)
}

// firstPos returns the position of the first specified ordinal marker,
// for use in error messages when the specific ordinal doesn't matter.
func (b *exampleBlock) firstPos() token.Pos {
	for _, p := range b.ordinalPositions {
		if p != 0 {
			return p
		}
	}
	return token.NoPos
}

// buildExamples groups the flat piece list by symbolKey, merges pieces of the
// same ordinal (sorted by source position), and returns the assembled blocks.
func buildExamples(pieces []piece) map[symbolKey]*exampleBlock {
	type ordinalKey struct {
		sym     symbolKey
		ordinal int
	}

	// group by (symbolKey, ordinal)
	byOrdKey := make(map[ordinalKey][]piece)
	for i := range pieces {
		ok := ordinalKey{sym: pieces[i].key, ordinal: pieces[i].ordinal}
		byOrdKey[ok] = append(byOrdKey[ok], pieces[i])
	}

	// sort pieces within each group by (file, line), skip single-element groups
	for ok, ps := range byOrdKey {
		if len(ps) > 1 {
			slices.SortFunc(ps, func(a, b piece) int {
				if a.file != b.file {
					return cmp.Compare(a.file, b.file)
				}
				return cmp.Compare(a.line, b.line)
			})
			byOrdKey[ok] = ps
		}
	}

	// Collect unique ordinals per symbol directly from byOrdKey.
	symOrds := make(map[symbolKey][]int)
	for ok := range byOrdKey {
		symOrds[ok.sym] = append(symOrds[ok.sym], ok.ordinal)
	}

	result := make(map[symbolKey]*exampleBlock, len(symOrds))
	for sym, ords := range symOrds {
		if len(ords) > 1 {
			slices.Sort(ords)
		}

		// Allocate with size = max ordinal (absolute indexing).
		maxOrd := ords[len(ords)-1]
		block := &exampleBlock{
			ordinals:         make([]string, maxOrd),
			ordinalPositions: make([]token.Pos, maxOrd),
		}
		for _, ord := range ords {
			ok := ordinalKey{sym: sym, ordinal: ord}
			ps := byOrdKey[ok]
			if len(ps) == 1 {
				block.ordinals[ord-1] = ps[0].text
			} else {
				var b strings.Builder
				for i, p := range ps {
					if i > 0 && len(p.text) > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(p.text)
				}
				block.ordinals[ord-1] = b.String()
			}
			block.ordinalPositions[ord-1] = ps[0].pos
		}

		result[sym] = block
	}
	return result
}

// renderOrdinal returns the replacement text for a single ordinal as a code
// block with a leading blank separator line. text is newline-delimited content.
func renderOrdinal(text string) string {
	// Pre-size: "//\n" header + "//\t" per line + text + newlines.
	var b strings.Builder
	b.Grow(3 + len(text) + strings.Count(text, "\n")*3 + 4)
	b.WriteString("//\n")
	for {
		nl := strings.IndexByte(text, '\n')
		var line string
		if nl < 0 {
			line = text
		} else {
			line = text[:nl]
		}
		if isBlankLine(line) {
			b.WriteString("//\n")
		} else {
			b.WriteString("//\t")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		if nl < 0 {
			break
		}
		text = text[nl+1:]
	}
	return b.String()
}

// ordinalMatchesFact reports whether renderOrdinal(text) would produce
// a string equal to factText, without allocating the rendered string.
// text is newline-delimited content.
func ordinalMatchesFact(text string, factText string) bool {
	// Expected format: "//\n" then for each line either "//\n" (blank) or "//\t" + line + "\n"
	if len(factText) < 3 || factText[0] != '/' || factText[1] != '/' || factText[2] != '\n' {
		return false
	}
	pos := 3
	for {
		nl := strings.IndexByte(text, '\n')
		var line string
		if nl < 0 {
			line = text
		} else {
			line = text[:nl]
		}
		if isBlankLine(line) {
			if pos+3 > len(factText) ||
				factText[pos] != '/' ||
				factText[pos+1] != '/' ||
				factText[pos+2] != '\n' {
				return false
			}
			pos += 3
		} else {
			// Need "//\t" + line + "\n"
			need := 3 + len(line) + 1
			if pos+need > len(factText) {
				return false
			}
			if factText[pos] != '/' || factText[pos+1] != '/' || factText[pos+2] != '\t' {
				return false
			}
			pos += 3
			if factText[pos:pos+len(line)] != line {
				return false
			}
			pos += len(line)
			if factText[pos] != '\n' {
				return false
			}
			pos++
		}
		if nl < 0 {
			break
		}
		text = text[nl+1:]
	}
	return pos == len(factText)
}

// isBlankLine reports whether s is empty or contains only whitespace.
// This avoids the allocation of strings.TrimSpace.
func isBlankLine(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\r' && s[i] != '\n' {
			return false
		}
	}
	return true
}
