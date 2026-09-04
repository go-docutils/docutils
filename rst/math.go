package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.body.MathBlock (read
// directly): ".. math::" wraps TeX math source in one or more
// <math_block> nodes. No arguments are declared (has_content=True plus a
// :class:/:name: option_spec), so same-line text after "::" folds into
// the directive's own first content line exactly like a generic
// admonition's does — which is WHY ".. math:: y = f(x)" followed by an
// indented "1+1=2" produces TWO blocks rather than an argument and a
// body: both end up in content, separated by the blank line between
// them.
//
// The distinctive part: content is joined and then SPLIT ON BLANK LINES,
// each non-empty chunk becoming its own <math_block> sibling ("content =
// '\n'.join(self.content).split('\n\n')", read directly) — a single
// directive can emit several nodes, the same one-directive-many-siblings
// shape the table/list-table directives already have. Content is kept
// verbatim (never inline-parsed): it's TeX source, not reST, so a
// backslash or asterisk in it is math syntax rather than markup.
func (p *parser) runMathDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
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
			`Content block expected for the "math" directive; none found.`, lineno, blockText)}
	}

	class := ""
	if v, ok := options["class"]; ok {
		class = strings.Join(classOption(v), " ")
	}
	name, id := "", ""
	if v, ok := options["name"]; ok && v != "" {
		name = normalizeName(v)
		id = makeID(name)
	}

	var out []doctree.Node
	for _, block := range strings.Split(strings.Join(content, "\n"), "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		el := doctree.NewElement(doctree.TagMathBlock, &doctree.Text{Data: block})
		if class != "" {
			el.SetAttr("class", class)
		}
		if name != "" {
			// add_name is called on EVERY block real docutils produces
			// here, not just the first (MathBlock.run, read directly) —
			// so a :name: alongside a blank-line-split body really does
			// repeat the same name/id across siblings. No corpus fixture
			// combines the two, and real docutils would diagnose the
			// duplicate id itself through machinery this project doesn't
			// have (see resolveTargets' own doc comment on duplicate
			// names) — matched as-is rather than silently deviating.
			el.SetAttr("name", name)
			el.SetAttr("id", id)
		}
		out = append(out, el)
	}
	return out
}
