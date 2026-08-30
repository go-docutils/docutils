package rst

import "strings"

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

func enumContentColumn(line string) int {
	i := strings.IndexByte(line, '.')
	j := i + 1
	for j < len(line) && line[j] == ' ' {
		j++
	}
	return j
}

// gatherListItemLines collects the lines belonging to one list item
// (bullet or enumerated), starting with firstLine (the marker line's
// remainder) and pulling in subsequent lines indented at least
// contentCol, dedented to that column. Returns the item's lines (local
// coordinate system) and the index of the first line past the item.
func gatherListItemLines(lines []string, i, contentCol int, firstLine string) ([]string, int) {
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
