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
// A line block groups consecutive lines into nested `line_block` wrappers
// by their relative indentation after the "| " marker
// (nest_line_block_lines/nest_line_block_segment), used for indented
// sub-stanzas — ported faithfully in nestLineBlockSegment below, verified
// against real docutils for a flat block, a single nested segment
// (docutils' own line_block.txt example), and a doubly-nested one. A
// doctest block's content is kept verbatim (including the ">>>"
// prompts), same as docutils.

func isLineBlockLine(s string) bool {
	if s == "" || s[0] != '|' {
		return false
	}
	return len(s) == 1 || s[1] == ' '
}

// lbLine is one line-block line before nesting: its content already
// parsed, its indent depth (docutils' own convention — the number of
// spaces after the mandatory one following "|"; -1 for an empty "|" line,
// whose indent isn't known until it inherits the previous line's).
type lbLine struct {
	el     *doctree.Element
	indent int
}

// parseLineBlock returns the built <line_block> plus every line's own
// inline-markup messages, in line order — real docutils attaches these
// individually as each line is consumed (Body.line_block: "self.parent +=
// messages"; LineBlock.line_block continuation: "self.parent.parent +=
// messages" — states.py, read directly), always to the line_block's OWN
// enclosing parent, never nested inside it or inside any individual
// <line>. Since nothing else is appended to that enclosing parent while a
// line_block is still being consumed, collecting them here and having the
// caller append them, in order, right after the (now complete) line_block
// produces the identical final tree position.
func (p *parser) parseLineBlock(lines []string, i int) (*doctree.Element, []*doctree.Element, int) {
	lb := doctree.NewElement(doctree.TagLineBlock)
	var items []lbLine
	var messages []*doctree.Element
	for i < len(lines) && isLineBlockLine(lines[i]) {
		rest := lines[i][1:]
		indent := -1
		content := ""
		if strings.TrimRight(rest, " ") != "" {
			n := 0
			for n < len(rest) && rest[n] == ' ' {
				n++
			}
			indent = n - 1
			content = rest[n:]
		}
		contentNodes, contentMsgs := p.parseInline(content, 0)
		items = append(items, lbLine{doctree.NewElement(doctree.TagLine, contentNodes...), indent})
		messages = append(messages, contentMsgs...)
		i++
	}
	if len(items) > 0 && items[0].indent < 0 {
		items[0].indent = 0
	}
	for k := 1; k < len(items); k++ {
		if items[k].indent < 0 {
			items[k].indent = items[k-1].indent
		}
	}
	nestLineBlockSegment(lb, items)
	return lb, messages, i
}

// nestLineBlockSegment is docutils' nest_line_block_segment: within one
// segment, find the shallowest indent present; every line at that depth
// stays a direct child, and every run of deeper lines between two such
// lines becomes its own nested <line_block>, recursively segmented the
// same way.
func nestLineBlockSegment(parent *doctree.Element, items []lbLine) {
	if len(items) == 0 {
		return
	}
	least := items[0].indent
	for _, it := range items[1:] {
		if it.indent < least {
			least = it.indent
		}
	}
	var group []lbLine
	flush := func() {
		if len(group) == 0 {
			return
		}
		nested := doctree.NewElement(doctree.TagLineBlock)
		nestLineBlockSegment(nested, group)
		parent.Append(nested)
		group = nil
	}
	for _, it := range items {
		if it.indent > least {
			group = append(group, it)
			continue
		}
		flush()
		parent.Append(it.el)
	}
	flush()
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
