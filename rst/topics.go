package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.body's
// BasePseudoSection/Topic/Sidebar (body.py, read directly): "topic"
// (a REQUIRED title argument) and "sidebar" (an OPTIONAL title
// argument, plus its own ":subtitle:" option, valid only alongside a
// title) share the same :class:/:name: options and content handling as
// the admonitions (see admonitions.go's parseDirectiveBlock) — the real
// difference is a NESTING restriction neither admonition needs: a
// topic/sidebar is only valid directly inside <document>/<section>
// (topic ALSO directly inside <sidebar>) — anywhere else (a list item, a
// block quote, another topic, ...) is an ERROR. real docutils checks
// this against the CURRENT container (state_machine.node); this
// project's own `parent` argument to parseDirective already IS that
// same current container, so no separate context-tracking is needed.

// runTopicOrSidebar implements Topic.run/Sidebar.run together — tag is
// doctree.TagTopic or doctree.TagSidebar.
func (p *parser) runTopicOrSidebar(tag string, lines []string, i, next int, args string, body []string, parent *doctree.Element) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")

	if parent.Tag != doctree.TagDocument && parent.Tag != doctree.TagSection && parent.Tag != doctree.TagSidebar {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"The \""+tag+"\" directive may not be used within topics or body elements.", lineno, blockText)}
	}
	if tag == doctree.TagSidebar && parent.Tag == doctree.TagSidebar {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`The "sidebar" directive may not be used within a sidebar element.`, lineno, blockText)}
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

	if tag == doctree.TagTopic && argument == "" {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Error in \""+tag+"\" directive:\n1 argument(s) required, 0 supplied.", lineno, blockText)}
	}
	if tag == doctree.TagSidebar {
		if _, hasSubtitle := options["subtitle"]; hasSubtitle && argument == "" {
			return []doctree.Node{sectionMessage("3", "ERROR",
				`The "subtitle" option may not be used without a title.`, lineno, blockText)}
		}
	}
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "`+tag+`" directive; none found.`, lineno, blockText)}
	}

	el := doctree.NewElement(tag)
	if v, ok := options["class"]; ok {
		el.SetAttr("class", strings.Join(classOption(v), " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", makeID(name))
	}
	var titleMsgs []*doctree.Element
	if argument != "" {
		var title *doctree.Element
		title, titleMsgs = p.parseTableTitle(argument, lineno)
		el.Append(title)
		if tag == doctree.TagSidebar {
			if sub, ok := options["subtitle"]; ok {
				subEl, subMsgs := p.parseTableTitle(sub, lineno)
				subEl.Tag = doctree.TagSubtitle
				el.Append(subEl)
				titleMsgs = append(titleMsgs, subMsgs...)
			}
		}
	}
	p.parseBlockLines(content, el)

	out := []doctree.Node{el}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	return out
}
