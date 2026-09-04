package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.body.Container (read
// directly): ".. container:: [class ...]" wraps its parsed content in a
// <container class="..."> — the argument (optionally spanning multiple
// lines, joined with a space, matching final_argument_whitespace=True)
// becomes the node's own classes via classOption (directives.class_option),
// NOT a :class: option — Container's own option_spec only recognizes
// :name:. An invalid-class-value error (real docutils raises one when
// class_option's own validation fails) isn't ported: no corpus fixture
// exercises it, and classOption never fails validation the way the real
// regex-based one can (it silently substitutes invalid characters
// instead), matching this project's existing class_option port.
func (p *parser) runContainerDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	directiveName := lines[i][3:]
	if idx := strings.Index(directiveName, "::"); idx >= 0 {
		directiveName = strings.TrimSpace(directiveName[:idx])
	}

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

	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "`+directiveName+`" directive; none found.`, lineno, blockText)}
	}

	el := doctree.NewElement(doctree.TagContainer)
	if argument != "" {
		el.SetAttr("class", strings.Join(classOption(argument), " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", makeID(name))
	}
	p.parseBlockLines(content, el)
	return []doctree.Node{el}
}
