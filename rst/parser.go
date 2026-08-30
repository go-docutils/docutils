// Package rst is a reStructuredText parser producing a doctree.Element
// document tree, modeled on docutils.parsers.rst.states.
//
// SCOPE (v1 — see [[go-docutils-org]] for the plan): sections (over/under
// lined titles), paragraphs, transitions, bullet lists, enumerated lists
// (arabic + '.' suffix only), field lists, definition lists, line blocks
// (flat, see lineblock.go for the un-implemented indent-nesting), doctest
// blocks, block quotes, literal blocks, comments, directives (captured
// structurally only, see explicit.go), hyperlink targets with reference
// resolution, and the inline markup in inline.go. NOT yet ported: option
// lists (see fieldlist.go for why they're deferred), tables,
// footnotes/citations, substitution definitions, indirect/anonymous
// hyperlink targets, per-directive semantics. Title-style consistency and
// enumerator-sequence validation are not enforced (docutils errors on
// inconsistent styles or skipped levels; this parser silently assigns a
// level instead).
package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

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
	resolveTargets(doc)
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
		if _, _, ok := matchFieldMarker(lines[i]); ok {
			fl, next := p.parseFieldList(lines, i)
			current.Append(fl)
			i = next
			continue
		}
		if isDoctestLine(lines[i]) {
			db, next := parseDoctestBlock(lines, i)
			current.Append(db)
			i = next
			continue
		}
		if isLineBlockLine(lines[i]) {
			lb, next := p.parseLineBlock(lines, i)
			current.Append(lb)
			i = next
			continue
		}
		if isExplicitMarkupLine(lines[i]) {
			node, next := parseExplicitMarkup(lines, i)
			current.Append(node)
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
		if isDefinitionTermLine(lines, i) {
			dl, next := p.parseDefinitionList(lines, i)
			current.Append(dl)
			i = next
			continue
		}
		para, next, literalNext := consumeParagraph(lines, i)
		if para != nil {
			current.Append(para)
		}
		i = next
		if literalNext {
			if lb, next2, ok := tryLiteralBlock(lines, i); ok {
				current.Append(lb)
				i = next2
			}
		}
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
		if _, _, ok := matchFieldMarker(lines[i]); ok {
			fl, next := p.parseFieldList(lines, i)
			parent.Append(fl)
			i = next
			continue
		}
		if isDoctestLine(lines[i]) {
			db, next := parseDoctestBlock(lines, i)
			parent.Append(db)
			i = next
			continue
		}
		if isLineBlockLine(lines[i]) {
			lb, next := p.parseLineBlock(lines, i)
			parent.Append(lb)
			i = next
			continue
		}
		if isExplicitMarkupLine(lines[i]) {
			node, next := parseExplicitMarkup(lines, i)
			parent.Append(node)
			i = next
			continue
		}
		if isTransitionLine(lines[i]) {
			parent.Append(doctree.NewElement(doctree.TagTransition))
			i++
			continue
		}
		if isDefinitionTermLine(lines, i) {
			dl, next := p.parseDefinitionList(lines, i)
			parent.Append(dl)
			i = next
			continue
		}
		para, next, literalNext := consumeParagraph(lines, i)
		if para != nil {
			parent.Append(para)
		}
		i = next
		if literalNext {
			if lb, next2, ok := tryLiteralBlock(lines, i); ok {
				parent.Append(lb)
				i = next2
			}
		}
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
	if _, _, isField := matchFieldMarker(lines[i]); !isBlankStr(lines[i]) && leadingSpaces(lines[i]) == 0 &&
		!isBulletLine(lines[i]) && !isEnumLine(lines[i]) && !isExplicitMarkupLine(lines[i]) && !isField &&
		!isDoctestLine(lines[i]) && !isLineBlockLine(lines[i]) {
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
// start of another recognized construct. A paragraph ending in "::"
// (docutils.states.RSTState.paragraph) marks the following indented
// block as a literal block rather than a block quote: literalNext is
// true, and the trailing "::" is either dropped (if preceded by
// whitespace) or collapsed to a single ":" (if attached to a word). A
// paragraph that is exactly "::" produces no paragraph node at all
// (returns nil, matching docutils).
func consumeParagraph(lines []string, i int) (para *doctree.Element, next int, literalNext bool) {
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
			if isBulletLine(lines[j]) || isEnumLine(lines[j]) || isExplicitMarkupLine(lines[j]) {
				break
			}
			if _, _, isField := matchFieldMarker(lines[j]); isField {
				break
			}
			if isDoctestLine(lines[j]) || isLineBlockLine(lines[j]) {
				break
			}
			if _, isLine := isUniformLine(lines[j]); isLine {
				break
			}
		}
		text = append(text, lines[j])
		j++
	}
	data := strings.Join(text, "\n")
	if data == "::" {
		return nil, j, true
	}
	if strings.HasSuffix(data, "::") {
		literalNext = true
		if len(data) >= 3 && (data[len(data)-3] == ' ' || data[len(data)-3] == '\n') {
			data = strings.TrimRight(data[:len(data)-2], " ")
		} else {
			data = data[:len(data)-1]
		}
	}
	para = doctree.NewElement(doctree.TagParagraph, parseInline(data)...)
	return para, j, literalNext
}

// tryLiteralBlock consumes the indented block starting at lines[i] (if
// any) as a literal block: raw text, not further parsed.
func tryLiteralBlock(lines []string, i int) (*doctree.Element, int, bool) {
	for i < len(lines) && isBlankStr(lines[i]) {
		i++
	}
	if i >= len(lines) || leadingSpaces(lines[i]) == 0 {
		return nil, i, false
	}
	indent := leadingSpaces(lines[i])
	block, next := consumeIndentedBlock(lines, i, indent)
	lb := doctree.NewElement(doctree.TagLiteralBlock, &doctree.Text{Data: strings.Join(block, "\n")})
	return lb, next, true
}
