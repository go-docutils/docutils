package rst

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// runContentsDirective ports directives.parts.Contents: a <topic> whose
// only child (besides an optional <title>) is the <pending> that a later
// transform replaces with the real table of contents. The parse-time
// shape is all this package produces, the same scope boundary
// .. class:: and .. sectnum:: already sit on.
//
// The rules, from parts.py read directly:
//
//   - classes are ["contents"], plus the :class: option, plus "local"
//     when :local: is given;
//   - the argument becomes the <title>; with NO argument the title is
//     the language's own "Contents" label -- EXCEPT under :local:, which
//     has no title at all;
//   - the name is the normalized title text (or "Contents"), and the id
//     comes from it;
//   - the pending's details are the parsed OPTIONS, so :depth: prints as
//     a bare integer, a flag as None, and :backlinks: none as None while
//     :backlinks: top prints as 'top'.
//
// Not ported: the "may not be used within topics or body elements"
// context check, which needs a parent-state notion this parser does not
// keep -- the same reason the replace/date context checks are one branch
// each rather than a general mechanism.
func (p *parser) runContentsDirective(args string, body []string, lineBase, i int) []doctree.Node {
	combined := append([]string{args}, body...)
	argument, options, _ := parseDirectiveBlock(combined, true)

	classes := []string{"contents"}
	if v, ok := options["class"]; ok {
		classes = append(classes, classOption(v)...)
	}
	_, local := options["local"]
	if local {
		classes = append(classes, "local")
	}

	el := doctree.NewElement(doctree.TagTopic)
	el.SetAttr("class", strings.Join(classes, " "))

	titleText := strings.TrimSpace(argument)
	var titleEl *doctree.Element
	switch {
	case titleText != "":
		nodes, _ := p.parseInline(titleText, msgLine(i, lineBase))
		titleEl = doctree.NewElement(doctree.TagTitle, nodes...)
	case !local:
		titleText = "Contents"
		titleEl = doctree.NewElement(doctree.TagTitle, &doctree.Text{Data: titleText})
	default:
		titleText = "Contents"
	}
	name := normalizeName(titleText)
	el.SetAttr("name", name)
	el.SetAttr("id", p.explicitTargetID("topic", name))
	if titleEl != nil {
		el.Append(titleEl)
	}

	var details []pendingDetail
	for k, v := range options {
		switch strings.ToLower(k) {
		case "depth":
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				details = append(details, pendingDetail{"depth", strconv.Itoa(n)})
			}
		case "local":
			// A flag: its VALUE is None, not the empty string.
			details = append(details, pendingDetail{"local", "None"})
		case "backlinks":
			// Contents.backlinks maps "none" onto None and leaves the
			// other two as strings.
			if strings.EqualFold(strings.TrimSpace(v), "none") {
				details = append(details, pendingDetail{"backlinks", "None"})
			} else {
				details = append(details, pendingDetail{"backlinks", pyRepr(strings.TrimSpace(v))})
			}
		case "class":
			details = append(details, pendingDetail{"class", pyReprList(classOption(v))})
		}
	}
	el.Append(pendingNode("docutils.transforms.parts.Contents", details))
	return []doctree.Node{el}
}
