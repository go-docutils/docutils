package rst

import (
	"strconv"
	"strings"
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

func bulletContentColumn(line string) int {
	j := 1
	for j < len(line) && line[j] == ' ' {
		j++
	}
	return j
}

func isEnumLine(s string) bool {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) || s[i] != '.' {
		return false
	}
	rest := s[i+1:]
	return rest == "" || rest[0] == ' '
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
// enumerator-looking line "correct" back to plain text instead.
func isEnumListStart(lines []string, i int) bool {
	if !isEnumLine(lines[i]) {
		return false
	}
	if i+1 >= len(lines) {
		return true
	}
	next := lines[i+1]
	if isBlankStr(next) || leadingSpaces(next) > 0 {
		return true
	}
	ordinal, ok := enumOrdinal(lines[i])
	if !ok {
		return false
	}
	return strings.HasPrefix(next, strconv.Itoa(ordinal+1)+".")
}

// enumOrdinal extracts the leading arabic ordinal from an enumerator line
// (this project only supports arabic + "." — see the package doc comment).
func enumOrdinal(s string) (int, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, false
	}
	return n, true
}

func enumContentColumn(line string) int {
	i := strings.IndexByte(line, '.')
	j := i + 1
	for j < len(line) && line[j] == ' ' {
		j++
	}
	return j
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
		if indent := leadingSpaces(lines[k]); indent > 0 && indent < markerCol {
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
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
