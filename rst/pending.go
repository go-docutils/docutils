package rst

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// A <pending> node is what docutils leaves behind for a directive whose
// real work happens in a later TRANSFORM: ".. class::", ".. sectnum::",
// ".. target-notes::" and friends parse to a placeholder carrying the
// transform's name and its options, and the transform itself acts on the
// document afterwards.
//
// This project has no transform system and is not getting one — but that
// was never what these nodes needed. The corpus compares the PARSE tree,
// and at parse time a <pending> is simply a node with a formatted text
// child; nothing has to run. Reading nodes.pending.pformat directly (it
// is where the odd indentation below comes from) turned a group of
// mismatches filed for years under "needs a transform system" into a
// straightforward node construction.
//
// The format, from that method verbatim: a ".. internal attributes:"
// line, then ".transform:" indented 5, then ".details:" indented 5, then
// one line per detail indented 7 as "key: <repr>", with the details
// SORTED BY KEY and their values rendered the way Python's %r would.
func pendingNode(transform string, details []pendingDetail) *doctree.Element {
	lines := []string{
		".. internal attributes:",
		"     .transform: " + transform,
		"     .details:",
	}
	sort.Slice(details, func(a, b int) bool { return details[a].key < details[b].key })
	for _, d := range details {
		lines = append(lines, fmt.Sprintf("       %s: %s", d.key, d.value))
	}
	return doctree.NewElement(doctree.TagPending,
		&doctree.Text{Data: strings.Join(lines, "\n")})
}

// pendingDetail is one already-rendered detail: value carries Python repr
// form, since that is what the reference emits (a string is quoted, an
// integer is not, a list of strings prints as ['a', 'b']).
type pendingDetail struct{ key, value string }

// pyRepr renders a string the way Python's %r does for the plain cases
// these details actually contain. A value with a single quote in it would
// need Python's own quote-switching rule; no directive option here can
// produce one, and inventing the rule unverified would be worse than
// this note.
func pyRepr(s string) string { return "'" + s + "'" }

func pyReprList(items []string) string {
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = pyRepr(it)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// runClassDirective ports directives.misc.Class: the class names are
// normalized the same way every other :class: option is, and the
// directive's own name is recorded because "class" and its alias
// "rst-class" produce different details.
func runClassDirective(name, args string, body []string) []doctree.Node {
	combined := append([]string{args}, body...)
	argument, _, _ := parseDirectiveBlock(combined, true)
	var classes []string
	for _, f := range strings.Fields(argument) {
		classes = append(classes, makeID(f))
	}
	details := []pendingDetail{{"directive", pyRepr(strings.ToLower(name))}}
	if len(classes) > 0 {
		details = append(details, pendingDetail{"class", pyReprList(classes)})
	}
	return []doctree.Node{pendingNode("docutils.transforms.misc.ClassAttribute", details)}
}

// runSectnumDirective ports directives.parts.SectNum. Only the options
// actually given appear in the details; depth and start are integers, so
// they print unquoted.
func runSectnumDirective(args string, body []string) []doctree.Node {
	combined := append([]string{args}, body...)
	_, options, _ := parseDirectiveBlock(combined, false)
	var details []pendingDetail
	for key, raw := range options {
		v := strings.TrimSpace(raw)
		switch strings.ToLower(key) {
		case "depth", "start":
			if _, err := strconv.Atoi(v); err == nil {
				details = append(details, pendingDetail{strings.ToLower(key), v})
			}
		case "prefix", "suffix":
			details = append(details, pendingDetail{strings.ToLower(key), pyRepr(v)})
		}
	}
	return []doctree.Node{pendingNode("docutils.transforms.parts.SectNum", details)}
}

// runTargetNotesDirective ports directives.references.TargetNotes, whose
// only option is :class:.
func runTargetNotesDirective(args string, body []string) []doctree.Node {
	combined := append([]string{args}, body...)
	_, options, _ := parseDirectiveBlock(combined, false)
	var details []pendingDetail
	if v, ok := options["class"]; ok {
		var classes []string
		for _, f := range strings.Fields(v) {
			classes = append(classes, makeID(f))
		}
		if len(classes) > 0 {
			details = append(details, pendingDetail{"class", pyReprList(classes)})
		}
	}
	return []doctree.Node{pendingNode("docutils.transforms.references.TargetNotes", details)}
}
