package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file covers reST's "explicit markup" (lines starting with `.. `):
// comments, directives, and hyperlink targets, modeled on docutils'
// Body.explicit_construct dispatch (states.py) which tries, in order,
// footnote, citation, hyperlink_target, substitution_def, directive, and
// falls back to comment.
//
// SCOPE (v1): only hyperlink_target and directive are recognized
// specifically; footnote/citation/substitution_def are NOT (a `..[1]` or
// `..|sub|` construct falls through to being read as a plain comment,
// same as docutils' own fallback for an unmatched explicit construct).
// Directives are captured structurally ONLY — name, a single-line
// argument string, and the raw (unparsed, not split into options vs.
// content) body text — never dispatched to per-directive semantics
// (there is no directive registry). Directive/comment body indentation
// is assumed to be exactly 3 columns (the ".. " prefix width), which
// covers the overwhelming convention in real-world reST but not an
// arbitrarily-indented body. Hyperlink targets are direct (name → URI)
// only; indirect targets (name → another reference) and anonymous
// targets (`.. __: uri` / `__ uri`) are not recognized. An unresolved
// reference (no matching target) is left as a bare `reference` node
// with no `refuri` attribute; real docutils instead runs an error
// transform that rewrites it to a `problematic` node and appends a
// system-message section to the document — not implemented here.

func isExplicitMarkupLine(s string) bool {
	if len(s) < 2 || s[0] != '.' || s[1] != '.' {
		return false
	}
	return len(s) == 2 || s[2] == ' '
}

// gatherExplicitBody collects the body of an explicit-markup construct:
// an optional leading blank line, then a block indented >= 3 columns
// (dedented), continuing across blank lines until a genuine dedent.
func gatherExplicitBody(lines []string, i int) ([]string, int) {
	j := i + 1
	for j < len(lines) && isBlankStr(lines[j]) {
		j++
	}
	if j >= len(lines) || leadingSpaces(lines[j]) < 3 {
		return nil, i + 1
	}
	return consumeIndentedBlock(lines, j, 3)
}

func parseExplicitMarkup(lines []string, i int) (doctree.Node, int) {
	line := lines[i]
	if len(line) == 2 {
		return doctree.NewElement(doctree.TagComment), i + 1
	}
	rest := line[3:]

	if len(rest) > 1 && rest[0] == '_' && rest[1] != ' ' {
		return parseHyperlinkTarget(lines, i, rest[1:])
	}
	if name, args, ok := matchDirectiveName(rest); ok {
		return parseDirective(lines, i, name, args)
	}
	return parseComment(lines, i, rest)
}

func isDirectiveNameChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '+' || b == '.'
}

// matchDirectiveName recognizes "name:: arguments" at the start of rest.
func matchDirectiveName(rest string) (name, args string, ok bool) {
	j := 0
	for j < len(rest) && isDirectiveNameChar(rest[j]) {
		j++
	}
	if j == 0 {
		return "", "", false
	}
	k := j
	if k < len(rest) && rest[k] == ' ' {
		k++
	}
	if !strings.HasPrefix(rest[k:], "::") {
		return "", "", false
	}
	return rest[:j], strings.TrimSpace(rest[k+2:]), true
}

func parseDirective(lines []string, i int, name, args string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	el := doctree.NewElement(doctree.TagDirective)
	el.SetAttr("name", name)
	if args != "" {
		el.SetAttr("arguments", args)
	}
	if len(body) > 0 {
		el.Append(&doctree.Text{Data: strings.Join(body, "\n")})
	}
	return el, next
}

func parseComment(lines []string, i int, rest string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	text := strings.TrimSpace(rest)
	if len(body) > 0 {
		if text != "" {
			text += "\n"
		}
		text += strings.Join(body, "\n")
	}
	if text == "" {
		return doctree.NewElement(doctree.TagComment), next
	}
	return doctree.NewElement(doctree.TagComment, &doctree.Text{Data: text}), next
}

// parseHyperlinkTarget recognizes ".. _name: uri", where uri may
// continue on subsequent body lines (concatenated with no added
// whitespace, matching how docutils reconstructs a wrapped URI).
func parseHyperlinkTarget(lines []string, i int, rest string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	name, uri := rest, ""
	if idx := strings.IndexByte(rest, ':'); idx >= 0 {
		name = rest[:idx]
		uri = rest[idx+1:]
	}
	name = strings.TrimSpace(name)
	uri = strings.TrimSpace(uri)
	for _, l := range body {
		uri += strings.TrimSpace(l)
	}
	el := doctree.NewElement(doctree.TagTarget)
	el.SetAttr("name", normalizeName(name))
	el.SetAttr("refuri", uri)
	return el, next
}

// normalizeName mirrors docutils.nodes.fully_normalize_name: case- and
// whitespace-insensitive matching between a reference and its target.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// resolveTargets walks the tree once to collect every hyperlink
// target's URI by normalized name, then walks it again setting refuri
// on every reference whose refname matches — docutils' target
// resolution transform, drastically simplified (no indirect targets, no
// duplicate/ambiguous-name diagnostics).
func resolveTargets(doc *doctree.Element) {
	targets := map[string]string{}
	collectTargets(doc, targets)
	linkReferences(doc, targets)
}

func collectTargets(n doctree.Node, targets map[string]string) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagTarget {
		if name := el.Attr("name"); name != "" {
			targets[name] = el.Attr("refuri")
		}
	}
	for _, c := range el.Children {
		collectTargets(c, targets)
	}
}

func linkReferences(n doctree.Node, targets map[string]string) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagReference {
		if ref := el.Attr("refname"); ref != "" {
			if uri, found := targets[normalizeName(ref)]; found {
				el.SetAttr("refuri", uri)
			}
		}
	}
	for _, c := range el.Children {
		linkReferences(c, targets)
	}
}
