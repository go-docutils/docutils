package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Line blocks ("| one verse line" per line, for addresses/poetry) and
// doctest blocks (interactive-Python-session snippets starting ">>>"),
// modeled on docutils' Body.line_block/line_block_line and Body.doctest
// (states.py).
//
// SCOPE (v1): a line block is captured as a FLAT sequence of `line`
// children — docutils additionally groups consecutive lines into nested
// `line_block` wrappers by their relative indentation after the "| "
// marker (nest_line_block_lines/nest_line_block_segment), used for
// indented sub-stanzas; that nesting is not implemented, so every line
// ends up a direct child regardless of how far it's indented past the
// marker. A doctest block's content is kept verbatim (including the
// ">>>" prompts), same as docutils.

func isLineBlockLine(s string) bool {
	if s == "" || s[0] != '|' {
		return false
	}
	return len(s) == 1 || s[1] == ' '
}

func (p *parser) parseLineBlock(lines []string, i int) (*doctree.Element, int) {
	lb := doctree.NewElement(doctree.TagLineBlock)
	for i < len(lines) && isLineBlockLine(lines[i]) {
		content := strings.TrimLeft(lines[i][1:], " ")
		lb.Append(doctree.NewElement(doctree.TagLine, parseInline(content)...))
		i++
	}
	return lb, i
}

func isDoctestLine(s string) bool {
	return strings.HasPrefix(s, ">>>") && (len(s) == 3 || s[3] == ' ')
}

func parseDoctestBlock(lines []string, i int) (*doctree.Element, int) {
	j := i
	for j < len(lines) && !isBlankStr(lines[j]) && leadingSpaces(lines[j]) == 0 {
		j++
	}
	text := strings.Join(lines[i:j], "\n")
	return doctree.NewElement(doctree.TagDoctestBlock, &doctree.Text{Data: text}), j
}
