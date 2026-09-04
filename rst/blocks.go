package rst

import (
	"strings"
	"unicode/utf8"
)

func isBlankStr(s string) bool { return strings.TrimSpace(s) == "" }

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func isPunctChar(r rune) bool {
	return (r >= 0x21 && r <= 0x2F) || (r >= 0x3A && r <= 0x40) ||
		(r >= 0x5B && r <= 0x60) || (r >= 0x7B && r <= 0x7E)
}

// isUniformLine reports whether s (right-trimmed) consists of one
// non-alphanumeric ASCII character repeated, docutils' 'line' pattern
// (title underline/overline or transition marker).
func isUniformLine(s string) (rune, bool) {
	t := strings.TrimRight(s, " ")
	if t == "" {
		return 0, false
	}
	r := []rune(t)
	first := r[0]
	if !isPunctChar(first) {
		return 0, false
	}
	for _, c := range r[1:] {
		if c != first {
			return 0, false
		}
	}
	return first, true
}

func isBulletChar(r rune) bool {
	switch r {
	case '-', '+', '*', '•', '‣', '⁃':
		return true
	}
	return false
}

func isBulletLine(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)
	if !isBulletChar(r[0]) {
		return false
	}
	return len(r) == 1 || r[1] == ' '
}

// bulletContentColumn returns the BYTE offset where a bullet list item's
// own content starts — the bullet marker itself (isBulletChar's own
// UNICODE bullets, "•"/"‣"/"⁃", are multi-byte in UTF-8, not just the
// ASCII "-"/"+"/"*") plus any spaces after it. Byte offset, not a rune
// count, since every caller slices the line with Go's own byte-indexed
// string slicing.
func bulletContentColumn(line string) int {
	_, size := utf8.DecodeRuneInString(line)
	j := size
	for j < len(line) && line[j] == ' ' {
		j++
	}
	return j
}

// isEnumLine reports whether s looks like SOME enumerator marker's shape
// — any of docutils' five sequences (arabic, loweralpha, upperalpha,
// lowerroman, upperroman) in any of its three formats ("N.", "(N)",
// "N)"), or the auto-enumerator "#" — with no lookahead-based list-item
// confirmation at all (see enum.go, isEnumeratedListItem/matchEnumStart
// for that); used only as a cheap EXCLUSION check by callers deciding
// whether a line could possibly be read as an enumerator, not as a
// standalone list-start decision.
func isEnumLine(s string) bool {
	_, _, _, ok := matchEnumeratorMarker(s)
	return ok
}

// isEnumListStart reports whether lines[i] both looks like an enumerator
// AND its second line confirms it's a genuine list item — real docutils'
// Body.enumerator calls is_enumerated_list_item to peek ahead before
// committing to starting a list (states.py, read directly): valid when the
// next line is blank, indented (the item's own continuation), absent
// (EOF), or itself starts with the NEXT ordinal's own enumerator (an
// immediately-following sibling item with no blank line between, e.g. a
// tightly nested "1. a\n2. b"). Anything else — most commonly a section-
// title underline ("1. Numbered Title\n===...===") — makes the
// enumerator-looking line "correct" back to plain text instead. See
// enum.go's matchEnumStart for the full (format, sequence, ordinal,
// contentCol) this wraps — every caller here only needs the bool.
func isEnumListStart(lines []string, i int) bool {
	_, _, _, _, _, ok := matchEnumStart(lines, i)
	return ok
}

// gatherListItemLines collects the lines belonging to one list item or
// field body, starting with firstLine (the marker line's remainder).
// markerCol is where the marker line's own content starts (e.g. after
// "- " or ":name: "). The body's actual indent, though, is taken from
// the FIRST continuation line when that is indented less than
// markerCol — a common, valid style for field lists in particular
// (":date: 2026-08-30\n  continuation" indents the continuation by 2,
// not by len(":date: ")=7) — mirroring docutils' get_first_known_indented,
// which detects the block's indent dynamically rather than assuming it
// matches the marker column. A continuation indented AT LEAST markerCol
// keeps markerCol as the dedent baseline, so deeper indentation still
// surfaces as nested content (e.g. a block quote) inside the item.
// Returns the item's lines (local coordinate system) and the index of
// the first line past the item.
func gatherListItemLines(lines []string, i, markerCol int, firstLine string) ([]string, int) {
	contentCol := markerCol
	for k := i + 1; k < len(lines); k++ {
		if isBlankStr(lines[k]) {
			continue
		}
		// A marker with content of its own on the SAME line (firstLine
		// != "") fixes the content column at markerCol; the only
		// adjustment needed is shrinking it for a shallower
		// continuation. A BARE marker (no trailing content at all — "1."
		// alone, nothing after) has no such anchor: real docutils takes
		// the content column straight from wherever the first indented
		// line actually starts, narrower OR wider than markerCol alike
		// (states.py's own list_item, via get_indented) — without this,
		// "1.\n   foo\n" (marker width 2, content indented 3) wrongly
		// treated "foo" as a nested block quote inside the item instead
		// of the item's own paragraph, caught by the corpus once
		// alpha/roman enumerators made this shape common enough to
		// surface (a bare "A.\n   text\n" reads identically).
		if indent := leadingSpaces(lines[k]); indent > 0 && (indent < markerCol || (firstLine == "" && indent > markerCol)) {
			contentCol = indent
		}
		break
	}

	itemLines := []string{firstLine}
	j := i + 1
	for j < len(lines) {
		if isBlankStr(lines[j]) {
			itemLines = append(itemLines, "")
			j++
			continue
		}
		if leadingSpaces(lines[j]) < contentCol {
			break
		}
		rest := lines[j]
		if len(rest) > contentCol {
			rest = rest[contentCol:]
		} else {
			rest = ""
		}
		itemLines = append(itemLines, rest)
		j++
	}
	for len(itemLines) > 0 && isBlankStr(itemLines[len(itemLines)-1]) {
		itemLines = itemLines[:len(itemLines)-1]
	}
	return itemLines, j
}

// consumeIndentedBlock collects the indented block starting at lines[i]
// (which must already be indented), stopping at the first non-blank line
// with indentation less than `indent`. Returns the block dedented by
// `indent` columns and the index past it.
func consumeIndentedBlock(lines []string, i, indent int) ([]string, int) {
	var block []string
	j := i
	for j < len(lines) {
		if isBlankStr(lines[j]) {
			block = append(block, "")
			j++
			continue
		}
		if leadingSpaces(lines[j]) < indent {
			break
		}
		rest := lines[j]
		if len(rest) > indent {
			rest = rest[indent:]
		} else {
			rest = ""
		}
		block = append(block, rest)
		j++
	}
	for len(block) > 0 && isBlankStr(block[len(block)-1]) {
		block = block[:len(block)-1]
	}
	return block, j
}

func splitLines(source string) []string {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")
	// Drop only ONE trailing empty element: the artifact of a final newline.
	// This is exactly Python's str.splitlines(), which docutils' string2lines
	// is built on. Genuine trailing blank lines must survive, because the
	// state machine consumes them and reports them in diagnostics: docutils
	// puts "Citation content expected." on line 2 for ".. [c]\n\n" and on
	// line 3 for ".. [c]\n\n\n" — the last line consumed, blank or not.
	// Stripping them all used to cancel an off-by-one elsewhere, so those
	// diagnostics came out right for the wrong reason.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
