// Package rst is a reStructuredText parser producing a doctree.Element
// document tree, modeled on docutils.parsers.rst.states.
//
// SCOPE (v1 — see [[go-docutils-org]] for the plan): sections (over/under
// lined titles), paragraphs, transitions, bullet lists, enumerated lists
// (arabic + '.' suffix only), block quotes, and the inline markup in
// inline.go. NOT yet ported: field lists, option lists, definition
// lists, line blocks, tables, footnotes/citations, hyperlink targets,
// substitution definitions, directives, comments, literal blocks,
// doctest blocks. Title-style consistency and enumerator-sequence
// validation are not enforced (docutils errors on inconsistent styles or
// skipped levels; this parser silently assigns a level instead).
package rst

import "github.com/go-docutils/docutils/doctree"

type titleStyle struct {
	char     rune
	overline bool
}

type parser struct {
	titleStyles []titleStyle
}

// Parse parses reStructuredText source into a document tree.
func Parse(source string) *doctree.Element {
	p := &parser{}
	doc := doctree.NewElement(doctree.TagDocument)
	p.parseDocument(splitLines(source), doc)
	return doc
}

// parseDocument is the top-level (and section-body) loop: it recognizes
// everything parseBlockLines does, plus section titles.
func (p *parser) parseDocument(lines []string, doc *doctree.Element) {
	current := doc
	var stack []*doctree.Element // open sections, stack[0] = top-level

	i := 0
	for i < len(lines) {
		if isBlankStr(lines[i]) {
			i++
			continue
		}
		if leadingSpaces(lines[i]) > 0 {
			bq, next := p.parseBlockQuote(lines, i)
			current.Append(bq)
			i = next
			continue
		}
		if isBulletLine(lines[i]) {
			list, next := p.parseBulletList(lines, i)
			current.Append(list)
			i = next
			continue
		}
		if isEnumLine(lines[i]) {
			list, next := p.parseEnumeratedList(lines, i)
			current.Append(list)
			i = next
			continue
		}
		if title, style, consumed, ok := matchTitle(lines, i); ok {
			sec := doctree.NewElement(doctree.TagSection)
			sec.Append(doctree.NewElement(doctree.TagTitle, parseInline(title)...))
			level := p.levelForStyle(style)
			for len(stack) >= level {
				stack = stack[:len(stack)-1]
			}
			parent := doc
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			parent.Append(sec)
			stack = append(stack, sec)
			current = sec
			i += consumed
			continue
		}
		if isTransitionLine(lines[i]) {
			current.Append(doctree.NewElement(doctree.TagTransition))
			i++
			continue
		}
		para, next := consumeParagraph(lines, i)
		current.Append(para)
		i = next
	}
}

// parseBlockLines parses body elements that may NOT contain sections
// (docutils: nested_parse sets match_titles=False for any node besides
// document/section — i.e. inside list items and block quotes).
func (p *parser) parseBlockLines(lines []string, parent *doctree.Element) {
	i := 0
	for i < len(lines) {
		if isBlankStr(lines[i]) {
			i++
			continue
		}
		if leadingSpaces(lines[i]) > 0 {
			bq, next := p.parseBlockQuote(lines, i)
			parent.Append(bq)
			i = next
			continue
		}
		if isBulletLine(lines[i]) {
			list, next := p.parseBulletList(lines, i)
			parent.Append(list)
			i = next
			continue
		}
		if isEnumLine(lines[i]) {
			list, next := p.parseEnumeratedList(lines, i)
			parent.Append(list)
			i = next
			continue
		}
		if isTransitionLine(lines[i]) {
			parent.Append(doctree.NewElement(doctree.TagTransition))
			i++
			continue
		}
		para, next := consumeParagraph(lines, i)
		parent.Append(para)
		i = next
	}
}

func (p *parser) levelForStyle(s titleStyle) int {
	for idx, existing := range p.titleStyles {
		if existing == s {
			return idx + 1
		}
	}
	p.titleStyles = append(p.titleStyles, s)
	return len(p.titleStyles)
}

func (p *parser) parseBlockQuote(lines []string, i int) (*doctree.Element, int) {
	indent := leadingSpaces(lines[i])
	block, next := consumeIndentedBlock(lines, i, indent)
	bq := doctree.NewElement(doctree.TagBlockQuote)
	p.parseBlockLines(block, bq)
	return bq, next
}

func (p *parser) parseBulletList(lines []string, i int) (*doctree.Element, int) {
	bulletChar := []rune(lines[i])[0]
	list := doctree.NewElement(doctree.TagBulletList)
	list.SetAttr("bullet", string(bulletChar))
	for i < len(lines) && isBulletLine(lines[i]) && []rune(lines[i])[0] == bulletChar {
		col := bulletContentColumn(lines[i])
		first := ""
		if len(lines[i]) > col {
			first = lines[i][col:]
		}
		itemLines, next := gatherListItemLines(lines, i, col, first)
		item := doctree.NewElement(doctree.TagListItem)
		p.parseBlockLines(itemLines, item)
		list.Append(item)
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	return list, i
}

func (p *parser) parseEnumeratedList(lines []string, i int) (*doctree.Element, int) {
	list := doctree.NewElement(doctree.TagEnumeratedList)
	for i < len(lines) && isEnumLine(lines[i]) {
		col := enumContentColumn(lines[i])
		first := ""
		if len(lines[i]) > col {
			first = lines[i][col:]
		}
		itemLines, next := gatherListItemLines(lines, i, col, first)
		item := doctree.NewElement(doctree.TagListItem)
		p.parseBlockLines(itemLines, item)
		list.Append(item)
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	return list, i
}

// matchTitle checks for a section title starting at lines[i]: an
// overlined form (uniform line / text / matching uniform line) or an
// underlined-only form (text / uniform line, underline at least 4
// columns or at least as long as the title — docutils.states.Text.
// underline: a shorter underline is "corrected" back to plain text).
func matchTitle(lines []string, i int) (title string, style titleStyle, consumed int, ok bool) {
	if char, isLine := isUniformLine(lines[i]); isLine {
		if i+2 < len(lines) {
			text := lines[i+1]
			if !isBlankStr(text) && leadingSpaces(text) == 0 {
				if char2, isLine2 := isUniformLine(lines[i+2]); isLine2 && char2 == char {
					return trimTrailingSpace(text), titleStyle{char: char, overline: true}, 3, true
				}
			}
		}
	}
	if !isBlankStr(lines[i]) && leadingSpaces(lines[i]) == 0 &&
		!isBulletLine(lines[i]) && !isEnumLine(lines[i]) {
		if i+1 < len(lines) {
			if char, isLine := isUniformLine(lines[i+1]); isLine {
				t := trimTrailingSpace(lines[i])
				u := trimTrailingSpace(lines[i+1])
				if len([]rune(u)) >= 4 || len([]rune(u)) >= len([]rune(t)) {
					return t, titleStyle{char: char, overline: false}, 2, true
				}
			}
		}
	}
	return "", titleStyle{}, 0, false
}

func isTransitionLine(s string) bool {
	char, ok := isUniformLine(s)
	_ = char
	if !ok {
		return false
	}
	return len([]rune(trimTrailingSpace(s))) >= 4
}

func trimTrailingSpace(s string) string {
	n := len(s)
	for n > 0 && s[n-1] == ' ' {
		n--
	}
	return s[:n]
}

// consumeParagraph collects consecutive plain-text lines into a
// paragraph, stopping at a blank line, an indentation change, or the
// start of another recognized construct.
func consumeParagraph(lines []string, i int) (*doctree.Element, int) {
	var text []string
	j := i
	for j < len(lines) {
		if isBlankStr(lines[j]) {
			break
		}
		if leadingSpaces(lines[j]) > 0 {
			break
		}
		if j > i {
			if isBulletLine(lines[j]) || isEnumLine(lines[j]) {
				break
			}
			if _, isLine := isUniformLine(lines[j]); isLine {
				break
			}
		}
		text = append(text, lines[j])
		j++
	}
	joined := ""
	for k, l := range text {
		if k > 0 {
			joined += "\n"
		}
		joined += l
	}
	para := doctree.NewElement(doctree.TagParagraph, parseInline(joined)...)
	return para, j
}
