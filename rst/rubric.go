package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.body.Rubric (read
// directly): ".. rubric:: TEXT" produces a <rubric> whose children are
// TEXT's own INLINE-parsed content (like a title, but with no wrapping
// <title> element — the <rubric> node itself IS the text container) —
// unlike topic/admonition, Rubric declares NO has_content at all, so any
// indented body beyond the (required, same-line-or-wrapped) argument is
// a real ERROR ("no content permitted"), a generic
// Body.parse_directive_block-level check (states.py, read directly) this
// project doesn't implement centrally but replicates here specifically,
// since no OTHER directive ported so far has needed it (every one either
// declares has_content=True, or simply never LOOKS at leftover content
// rather than erroring on it).
func (p *parser) runRubricDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
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
	argument, options, content := parseDirectiveBlock(combined, true)

	if argument == "" {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Error in \"rubric\" directive:\n1 argument(s) required, 0 supplied.", lineno, blockText)}
	}
	if len(content) > 0 && !allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Error in \"rubric\" directive:\nno content permitted.", lineno, blockText)}
	}

	textNodes, msgs := p.parseInline(argument, lineno)
	el := doctree.NewElement(doctree.TagRubric, textNodes...)
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
