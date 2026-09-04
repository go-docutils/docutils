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
// doctree.TagTopic or doctree.TagSidebar. lineBase/blankFinish are the
// same threading convention every other explicit-markup handler in this
// package already has (see parseComment/parseFootnoteOrCitation): lineBase
// for msgLine's own absolute-line computation, blankFinish for the
// "Explicit markup ends without a blank line; unexpected unindent."
// warning — both were MISSING here specifically (topics/sidebars predate
// the lineBase convention, v0.27.0/v0.28.0), the "worth a dedicated
// round" gap this file's own README already flagged as recurring
// (v0.23.0's code_parsing][0], confirmed to also affect nested topic/
// sidebar ERROR messages in v0.28.0) — closed this round by threading
// lineBase all the way from parseExplicitMarkup through parseDirective
// into here, AND (new this round) computing a REAL lineBase for this
// function's own recursive p.parseBlockLines(content, ...) call instead
// of parseBlockLines' usual "-1, no known correspondence" default: since
// content is a pure SUFFIX of combined below (this directive always
// requires hasArgument=true, so parseDirectiveBlock's fold-back branch
// never runs — see its own doc comment), combined[idx] corresponds
// EXACTLY to lines[i+idx] for every idx (a straightforward consequence
// of combined's own construction: one line in, one line out, no
// reordering or skipping beyond the front/back trims that only ever
// affect combined's own edges, never an interior index) — so content's
// own absolute line-0 offset is simply i + (len(combined)-len(content)).
func (p *parser) runTopicOrSidebar(tag string, lines []string, i, lineBase, next int, args string, body []string, blankFinish bool, parent *doctree.Element) []doctree.Node {
	lineno := msgLine(i, lineBase)
	blockText := strings.Join(lines[i:next], "\n")

	// The SAME "ends without a blank line" warning every other explicit
	// construct already carries (see stoppedOnExplicitMarkup's own
	// established pattern) — real docutils' Body.explicit_markup wraps
	// EVERY explicit construct in it uniformly, not just the rejected-
	// nesting case below, so it's collected once here and appended to
	// whatever this function ultimately returns, on every path.
	var warnings []doctree.Node
	stoppedOnExplicitMarkup := next < len(lines) && isExplicitMarkupLine(lines[next])
	if !blankFinish && !stoppedOnExplicitMarkup {
		warnings = append(warnings, sectionMessage("2", "WARNING",
			"Explicit markup ends without a blank line; unexpected unindent.", msgLine(next, lineBase), ""))
	}

	if parent.Tag != doctree.TagDocument && parent.Tag != doctree.TagSection && parent.Tag != doctree.TagSidebar {
		return append([]doctree.Node{sectionMessage("3", "ERROR",
			"The \""+tag+"\" directive may not be used within topics or body elements.", lineno, blockText)}, warnings...)
	}
	if tag == doctree.TagSidebar && parent.Tag == doctree.TagSidebar {
		return append([]doctree.Node{sectionMessage("3", "ERROR",
			`The "sidebar" directive may not be used within a sidebar element.`, lineno, blockText)}, warnings...)
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
	contentLineBase := -1
	if lineBase >= 0 {
		contentLineBase = i + (len(combined) - len(content)) + lineBase
	}

	if tag == doctree.TagTopic && argument == "" {
		return append([]doctree.Node{sectionMessage("3", "ERROR",
			"Error in \""+tag+"\" directive:\n1 argument(s) required, 0 supplied.", lineno, blockText)}, warnings...)
	}
	if tag == doctree.TagSidebar {
		if _, hasSubtitle := options["subtitle"]; hasSubtitle && argument == "" {
			return append([]doctree.Node{sectionMessage("3", "ERROR",
				`The "subtitle" option may not be used without a title.`, lineno, blockText)}, warnings...)
		}
	}
	if len(content) == 0 || allBlank(content) {
		return append([]doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "`+tag+`" directive; none found.`, lineno, blockText)}, warnings...)
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
	p.parseBlockLines(content, el, contentLineBase)

	out := []doctree.Node{el}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	out = append(out, warnings...)
	return out
}
