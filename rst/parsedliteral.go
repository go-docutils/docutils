package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.body.ParsedLiteral
// (read directly): ".. parsed-literal::" produces a <literal_block>
// whose content is INLINE-parsed (unlike an ordinary literal block,
// whose content stays completely raw) — the whole joined content is
// parsed as ONE inline_text call, matching real docutils exactly, so
// inline markup spanning multiple physical lines (e.g. "*emphasis\nover
// two lines*") still resolves to a single node. No argument at all
// (has_content=True, no option_spec-driven argument count), so same-line
// text folds into the content's own first line exactly like compound's
// does (runAdmonitionOrGeneric's own !hasArgument fold-back, reused here
// via the same parseDirectiveBlock(combined, false) call).
func (p *parser) runParsedLiteralDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")

	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	combined := make([]string, 0, 1+blanks+len(body))
	combined = append(combined, args)
	for k := 0; k < blanks; k++ {
		combined = append(combined, "")
	}
	combined = append(combined, body...)
	_, options, content := parseDirectiveBlock(combined, false)

	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "parsed-literal" directive; none found.`, lineno, blockText)}
	}

	text := strings.Join(content, "\n")
	textNodes, msgs := p.parseInline(text, lineno)
	el := doctree.NewElement(doctree.TagLiteralBlock, textNodes...)
	if v, ok := options["class"]; ok {
		el.SetAttr("class", strings.Join(classOption(v), " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", makeID(name))
	}

	out := []doctree.Node{el}
	for _, m := range msgs {
		out = append(out, m)
	}
	return out
}
