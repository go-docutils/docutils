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
//
// Each "|"-marked line's own CONTINUATION lines — indented, not
// themselves "|"-prefixed, not blank — join the SAME <line> rather than
// spinning off as an unrelated block_quote sibling: real docutils'
// line_block_line calls get_first_known_indented(match.end(),
// until_blank=True) (states.py, read directly) — the exact same
// known-first-line/unknown-rest, true-minimum-dedent mechanism
// gatherFootnoteBody already uses for a footnote body, here with
// until_blank additionally ending the continuation at the first blank
// line (a line block, unlike a footnote body, never has an internal
// blank-separated second paragraph within one <line>). The joined text
// is inline-parsed as ONE multi-line unit, so markup can span the wrap
// (docutils' own "Individual lines in line blocks *may* wrap" example) —
// this used to only ever look at the marker line's own remainder,
// leaving any continuation as unparsed, misclassified plain text one
// level up.
//
// A line block that ends on a non-blank line which ISN'T itself a new
// "|"-marked line (an abrupt interruption, not a blank-line-terminated
// or EOF-terminated finish) gets a "Line block ends without a blank
// line." WARNING — real docutils computes its line number from the
// WHOLE block's own FIRST marker line, not the interrupting line
// itself (`line=lineno+1`, states.py's Body.line_block, read directly:
// `lineno` is captured once, before any line is even consumed), matched
// here exactly rather than the more "obviously right" interrupting-line
// number, since that's what the corpus's own foreign-judge output shows.
//
// lineBase mirrors every other line-scanning function in this package —
// see consumeParagraph's own doc comment — passed through here so each
// line's own inline-markup diagnostics carry a real absolute line
// number when available, matching real docutils' own per-line lineno
// tracking (previously always 0/unknown for every line block line,
// regardless of context).
func (p *parser) parseLineBlock(lines []string, i, lineBase int) (*doctree.Element, []*doctree.Element, int) {
	lb := doctree.NewElement(doctree.TagLineBlock)
	firstLine := i
	var items []lbLine
	var messages []*doctree.Element
	for i < len(lines) && isLineBlockLine(lines[i]) {
		lineno := 0
		if lineBase >= 0 {
			lineno = i + lineBase + 1
		}
		rest := lines[i][1:]
		indent := -1
		firstContent := ""
		if strings.TrimRight(rest, " ") != "" {
			n := 0
			for n < len(rest) && rest[n] == ' ' {
				n++
			}
			indent = n - 1
			firstContent = rest[n:]
		}
		cont, _, _, next := collectLiteralIndented(lines, i+1, true)
		content := strings.Join(append([]string{firstContent}, cont...), "\n")
		contentNodes, contentMsgs := p.parseInline(content, lineno)
		items = append(items, lbLine{doctree.NewElement(doctree.TagLine, contentNodes...), indent})
		messages = append(messages, contentMsgs...)
		i = next
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
	if !(i >= len(lines) || isBlankStr(lines[i])) {
		messages = append(messages, sectionMessage("2", "WARNING",
			"Line block ends without a blank line.", msgLine(firstLine+1, lineBase), ""))
	}
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
