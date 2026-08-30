package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file covers reST's "explicit markup" (lines starting with `.. `):
// comments, directives, hyperlink targets, footnotes, citations, and
// substitution definitions, modeled on docutils' Body.explicit_construct
// dispatch (states.py) which tries, in order, footnote, citation,
// hyperlink_target, substitution_def, directive, and falls back to
// comment.
//
// SCOPE (v1): directives are captured structurally ONLY — name, a
// single-line argument string, and the raw (unparsed, not split into
// options vs. content) body text — never dispatched to per-directive
// semantics (there is no directive registry); a substitution definition
// is captured the same way, with its `|name|` attached as an extra
// attribute. Directive/comment/footnote/citation/substitution body
// indentation is assumed to be exactly 3 columns (the ".. " prefix
// width), which covers the overwhelming convention in real-world reST
// but not an arbitrarily-indented body. A hyperlink target may be named
// (a direct URI, or chased through however many INDIRECT hops — see
// bareIndirectTargetName), an INLINE target inside a paragraph (see
// inline.go's tryInlineTarget), or ANONYMOUS (`.. __: uri`, resolved by
// document-order position rather than by name — see resolveTargets); an
// indirect anonymous target (`.. __: othername_`) is not implemented, a
// rare compound of two already-rare constructs. An
// unresolved reference (no matching target) is left as a bare
// `reference` node with no `refuri` attribute; real docutils instead
// runs an error transform that rewrites it to a `problematic` node and
// appends a system-message section to the document — not implemented
// here. Footnote/citation numbering and symbol assignment (docutils'
// note_autofootnote/note_symbol_footnote bookkeeping) IS implemented —
// see footnotenum.go's resolveFootnoteNumbers, run at the end of Parse
// alongside resolveTargets. Citations are never auto-numbered (see
// parseFootnoteOrCitation above), so this applies only to footnotes.

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

func (p *parser) parseExplicitMarkup(lines []string, i int) (doctree.Node, int) {
	line := lines[i]
	if len(line) == 2 {
		return doctree.NewElement(doctree.TagComment), i + 1
	}
	rest := line[3:]

	if label, labelRest, ok := matchBracketLabel(rest); ok {
		return p.parseFootnoteOrCitation(lines, i, label, labelRest)
	}
	if strings.HasPrefix(rest, "__:") {
		return parseAnonymousTarget(lines, i, rest[3:])
	}
	if len(rest) > 1 && rest[0] == '_' && rest[1] != ' ' {
		return parseHyperlinkTarget(lines, i, rest[1:])
	}
	if subName, subRest, ok := matchPipeLabel(rest); ok {
		if node, next, ok := p.parseSubstitutionDef(lines, i, subName, subRest); ok {
			return node, next
		}
		// Malformed substitution definition: fall through to comment,
		// matching docutils' fallback for any unmatched explicit
		// construct (explicit_construct's final `return self.comment(...)`).
	}
	if name, args, ok := matchDirectiveName(rest); ok {
		return parseDirective(lines, i, name, args)
	}
	return parseComment(lines, i, rest)
}

// matchBracketLabel recognizes "[label] rest" at the start of s — the
// shared shape of footnote (`.. [1]`, `.. [#]`, `.. [#name]`, `.. [*]`)
// and citation (`.. [name]`) markers. The closing ']' must be followed
// by whitespace or end-of-string.
func matchBracketLabel(s string) (label, rest string, ok bool) {
	if len(s) == 0 || s[0] != '[' {
		return "", "", false
	}
	end := strings.IndexByte(s, ']')
	if end < 0 {
		return "", "", false
	}
	if end+1 < len(s) && s[end+1] != ' ' {
		return "", "", false
	}
	return s[1:end], strings.TrimSpace(s[end+1:]), true
}

// parseFootnoteOrCitation classifies a bracket-labeled explicit-markup
// block: "*" or a "#"-prefixed or all-digit label is a footnote
// (auto-symbol / auto-numbered / manually numbered); anything else is a
// citation, docutils' footnote vs. citation split.
func (p *parser) parseFootnoteOrCitation(lines []string, i int, label, firstLineRest string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	content := append([]string{firstLineRest}, body...)
	for len(content) > 0 && isBlankStr(content[len(content)-1]) {
		content = content[:len(content)-1]
	}

	var el *doctree.Element
	switch {
	case label == "*":
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("auto", "*")
	case len(label) > 0 && label[0] == '#':
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("auto", "1")
		if name := label[1:]; name != "" {
			el.SetAttr("name", normalizeName(name))
		}
	case isAllDigits(label):
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("name", label)
		el.Append(doctree.NewElement(doctree.TagLabel, &doctree.Text{Data: label}))
	default:
		el = doctree.NewElement(doctree.TagCitation)
		el.SetAttr("name", normalizeName(label))
		el.Append(doctree.NewElement(doctree.TagLabel, &doctree.Text{Data: label}))
	}
	if len(content) > 0 {
		p.parseBlockLines(content, el)
	}
	return el, next
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// matchPipeLabel recognizes "|name| rest" at the start of s — a
// substitution definition marker. The name may not contain "|".
func matchPipeLabel(s string) (name, rest string, ok bool) {
	if len(s) == 0 || s[0] != '|' {
		return "", "", false
	}
	end := strings.IndexByte(s[1:], '|')
	if end < 0 {
		return "", "", false
	}
	end++ // index within s, not s[1:]
	return s[1:end], strings.TrimSpace(s[end+1:]), true
}

// parseSubstitutionDef recognizes ".. |name| directive:: args", the
// only shape a substitution definition takes (its "content" is always
// an embedded directive invocation — commonly `replace::`, `image::`,
// or `unicode::` — never implemented, so this stores the same
// structural capture parseDirective would). Returns ok=false for a
// malformed definition (no embedded directive), matching docutils'
// fallback to a plain comment in that case.
func (p *parser) parseSubstitutionDef(lines []string, i int, name, directiveRest string) (doctree.Node, int, bool) {
	dirName, args, ok := matchDirectiveName(directiveRest)
	if !ok {
		return nil, 0, false
	}
	node, next := parseDirective(lines, i, dirName, args)
	el := node.(*doctree.Element)
	el.Tag = doctree.TagSubstitutionDef
	el.SetAttr("substitution", normalizeWhitespace(name))
	return el, next, true
}

// normalizeWhitespace mirrors docutils.nodes.whitespace_normalize_name:
// unlike target/reference names, a substitution name is case-sensitive.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
// whitespace, matching how docutils reconstructs a wrapped URI). When the
// value is itself a bare "othername_" reference rather than a URI, this is
// an INDIRECT target (docutils' parse_target/is_reference): "refname" is
// set instead of "refuri", and resolveTargets chases through it to find
// the final URI.
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
	if indirect, ok := bareIndirectTargetName(uri); ok {
		el.SetAttr("refname", normalizeName(indirect))
	} else {
		el.SetAttr("refuri", uri)
	}
	return el, next
}

// parseAnonymousTarget recognizes ".. __: uri" — a target with no name at
// all. Unlike a named target, it resolves by DOCUMENT-ORDER POSITION: the
// Nth anonymous reference (bare "x__", or "`x`__" — see tryBareReference/
// referenceOrPhrase) matches the Nth anonymous target, textual order,
// regardless of any text either one carries (verified against real
// docutils: this holds whether the targets appear before or after their
// references). resolveTargets implements the matching; indirect chasing
// ("__" pointing at a named reference rather than a URI) is not
// implemented here — a rare compound of two already-rare constructs.
func parseAnonymousTarget(lines []string, i int, rest string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	uri := strings.TrimSpace(rest)
	for _, l := range body {
		uri += strings.TrimSpace(l)
	}
	el := doctree.NewElement(doctree.TagTarget)
	el.SetAttr("anonymous", "true")
	el.SetAttr("refuri", uri)
	return el, next
}

// bareIndirectTargetName reports whether uri, taken as a WHOLE, is a bare
// "othername_" reference (docutils' parse_target: the target's value ends
// in "_" AND the entire value matches its own bare-reference grammar — a
// real URI ending in "_", like ".../foo_", does NOT match, since it
// contains characters (":", "/") a simplename can't). The backtick-quoted
// phrase form ("`other name`_") is not implemented here: rare for a
// target's own value, unlike a reference's.
func bareIndirectTargetName(uri string) (string, bool) {
	if len(uri) < 2 || uri[len(uri)-1] != '_' {
		return "", false
	}
	body := uri[:len(uri)-1]
	if body == "" {
		return "", false
	}
	for _, r := range body {
		if !isSimpleNameChar(r) {
			return "", false
		}
	}
	return body, true
}

// normalizeName mirrors docutils.nodes.fully_normalize_name: case- and
// whitespace-insensitive matching between a reference and its target.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// resolveTargets walks the tree once to collect every hyperlink target —
// named ones by normalized name, split into "direct" (has its own refuri,
// or is an inline internal target with none, resolved to a same-document
// anchor) and "indirect" (its value is itself another target's name,
// chased through resolveIndirect until a direct one is reached), and
// anonymous ones (".. __: uri") into a separate document-order list — then
// walks it again setting refuri on every reference: by name for a named
// one, or by consuming the next unused anonymous target in document order
// for an anonymous one. docutils' target resolution transform, drastically
// simplified (no duplicate/ambiguous-name diagnostics).
func resolveTargets(doc *doctree.Element) {
	direct := map[string]string{}
	indirect := map[string]string{}
	var anonTargets []string
	collectTargets(doc, direct, indirect, &anonTargets)
	targets := map[string]string{}
	for name := range direct {
		targets[name] = direct[name]
	}
	for name := range indirect {
		if uri, ok := resolveIndirect(name, direct, indirect, 0); ok {
			targets[name] = uri
		}
	}
	anonIndex := 0
	linkReferences(doc, targets, anonTargets, &anonIndex)
}

func collectTargets(n doctree.Node, direct, indirect map[string]string, anonTargets *[]string) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagTarget {
		switch {
		case el.Attr("anonymous") == "true":
			*anonTargets = append(*anonTargets, el.Attr("refuri"))
		case el.Attr("name") != "":
			name := el.Attr("name")
			switch {
			case el.Attr("refuri") != "":
				direct[name] = el.Attr("refuri")
			case el.Attr("refname") != "":
				indirect[name] = el.Attr("refname")
			default:
				// An inline internal target ("_`text`") carries no URI of
				// its own — it IS the destination, a same-document anchor
				// a reference resolves to by pointing at its own id.
				direct[name] = "#" + name
			}
		}
	}
	for _, c := range el.Children {
		collectTargets(c, direct, indirect, anonTargets)
	}
}

// resolveIndirect chases an indirect target's refname chain
// ("a" -> "b" -> "c" -> a real refuri) to its final URI. depth guards
// against a cycle ("a" -> "b" -> "a"), which real docutils reports as an
// error; this simplified version just leaves it unresolved, same as any
// other name with no matching target.
func resolveIndirect(name string, direct, indirect map[string]string, depth int) (string, bool) {
	if depth > 20 {
		return "", false
	}
	if uri, ok := direct[name]; ok {
		return uri, true
	}
	if next, ok := indirect[name]; ok {
		return resolveIndirect(next, direct, indirect, depth+1)
	}
	return "", false
}

func linkReferences(n doctree.Node, targets map[string]string, anonTargets []string, anonIndex *int) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagReference {
		switch {
		case el.Attr("anonymous") == "true":
			if *anonIndex < len(anonTargets) {
				el.SetAttr("refuri", anonTargets[*anonIndex])
				*anonIndex++
			}
		case el.Attr("refname") != "":
			if uri, found := targets[normalizeName(el.Attr("refname"))]; found {
				el.SetAttr("refuri", uri)
			}
		}
	}
	for _, c := range el.Children {
		linkReferences(c, targets, anonTargets, anonIndex)
	}
}
