package rst

import (
	"fmt"
	"strconv"
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
// document-order position rather than by name — see resolveTargets),
// including an INDIRECT anonymous target (`.. __: othername_`, chased the
// same way a named indirect target is). An
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
// references). resolveTargets implements the matching. The value may
// itself be a bare "othername_" reference rather than a URI — an INDIRECT
// anonymous target (".. __: othername_") — chased the same way a named
// indirect target is (see bareIndirectTargetName); verified against real
// docutils that an anonymous reference still consumes its document-order
// slot even when the chase fails to resolve (it just ends up with no
// refuri set, this codebase's usual treatment of an unresolved reference,
// rather than docutils' own refid-to-the-target-itself fallback, which
// depends on error-reporting machinery not implemented here).
func parseAnonymousTarget(lines []string, i int, rest string) (doctree.Node, int) {
	body, next := gatherExplicitBody(lines, i)
	uri := strings.TrimSpace(rest)
	for _, l := range body {
		uri += strings.TrimSpace(l)
	}
	el := doctree.NewElement(doctree.TagTarget)
	el.SetAttr("anonymous", "true")
	if indirect, ok := bareIndirectTargetName(uri); ok {
		el.SetAttr("refname", normalizeName(indirect))
	} else {
		el.SetAttr("refuri", uri)
	}
	return el, next
}

// bareIndirectTargetName reports whether uri, taken as a WHOLE, is a bare
// "othername_" reference (docutils' parse_target: the target's value ends
// in "_" AND the entire value matches the simplename grammar —
// scanSimpleName in inline.go). A real URI ending in "_", like
// ".../foo_", does NOT match, since "/" isn't a valid simplename
// separator. The backtick-quoted phrase form ("`other name`_") is not
// implemented here: rare for a target's own value, unlike a reference's.
func bareIndirectTargetName(uri string) (string, bool) {
	if len(uri) < 2 || uri[len(uri)-1] != '_' {
		return "", false
	}
	runes := []rune(uri[:len(uri)-1])
	if len(runes) == 0 {
		return "", false
	}
	if scanSimpleName(runes, 0) != len(runes) {
		return "", false
	}
	return string(runes), true
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
// for an anonymous one. A NAMED reference (bare, backtick-quoted, or
// embedded-indirect-alias) whose name matches no target anywhere is
// rewritten to a <problematic> in place, docutils' own DanglingReferences
// transform, simplified: no duplicate/ambiguous-name diagnostics, and the
// <problematic>'s content is the reference's own VISIBLE text rather than
// real docutils' verbatim source slice (rawsource) — this parser doesn't
// track original source text on a reference node at all (see the package
// SCOPE note on ids/source attributes), so "broken_" resynthesizes as
// "broken", the same category of simplification already accepted
// elsewhere in this project (go-richdoc/rst's own Raw* reconstruction
// helpers) rather than a new one invented for this feature. An ANONYMOUS
// reference is checked differently, matching real docutils'
// AnonymousHyperlinks.apply (transforms/references.py, read directly, not
// guessed from behavior — an earlier pass at this got the direction of
// the check wrong): the count of anonymous references against the count
// of anonymous targets is a single, whole-document condition (`!=`, not
// merely "too many refs"), checked once — if they don't match EXACTLY,
// EVERY anonymous reference in the document becomes <problematic>,
// regardless of which side has the surplus, all sharing ONE message
// (docutils' own "Anonymous hyperlink mismatch" error).
func resolveTargets(doc *doctree.Element) {
	direct := map[string]string{}
	indirect := map[string]string{}
	var anonTargets []anonTarget
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
	anonURIs := make([]string, len(anonTargets))
	for i, t := range anonTargets {
		switch {
		case t.refuri != "":
			anonURIs[i] = t.refuri
		case t.refname != "":
			if uri, ok := resolveIndirect(t.refname, direct, indirect, 0); ok {
				anonURIs[i] = uri
			}
		}
	}
	anonIndex := 0
	var messages []*doctree.Element
	msgCount := 0
	var anonMismatch *doctree.Element
	if n := countAnonymousReferences(doc); n != len(anonURIs) {
		msgCount++
		anonMismatch = doctree.NewElement(doctree.TagSystemMessage,
			doctree.NewElement(doctree.TagParagraph, &doctree.Text{
				Data: fmt.Sprintf(`Anonymous hyperlink mismatch: %d references but %d targets.`, n, len(anonURIs)),
			}))
		anonMismatch.SetAttr("id", "system-message-"+strconv.Itoa(msgCount))
		messages = append(messages, anonMismatch)
	}
	linkReferences(doc, targets, anonURIs, anonMismatch, &anonIndex, &messages, &msgCount)
	if len(messages) > 0 {
		doc.Append(systemMessagesSection(messages))
	}
}

// countAnonymousReferences counts every anonymous reference (bare "x__",
// backtick-quoted "`x`__", or a substitution used as one, "|x|__" — see
// inline.go's tryMarker) anywhere in the document, needed BEFORE the main
// walk since the mismatch check below is whole-document, not incremental.
func countAnonymousReferences(n doctree.Node) int {
	el, ok := n.(*doctree.Element)
	if !ok {
		return 0
	}
	count := 0
	if el.Tag == doctree.TagReference && el.Attr("anonymous") == "true" {
		count++
	}
	for _, c := range el.Children {
		count += countAnonymousReferences(c)
	}
	return count
}

// anonTarget is one ".. __: ..." target in document order: either a direct
// URI or, for an indirect anonymous target, the name it chases.
type anonTarget struct {
	refuri, refname string
}

func collectTargets(n doctree.Node, direct, indirect map[string]string, anonTargets *[]anonTarget) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagTarget {
		switch {
		case el.Attr("anonymous") == "true":
			*anonTargets = append(*anonTargets, anonTarget{refuri: el.Attr("refuri"), refname: el.Attr("refname")})
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
// against a cycle ("a" -> "b" -> "a") — checked against the foreign judge:
// real docutils does NOT report this as an error at all, it resolves
// SILENTLY to an odd same-document self-reference (the reference ends up
// pointing at its own cycle's first target by id) that this project
// doesn't replicate; this simplified version just leaves a cycle
// unresolved, same as any other name with no matching target, which
// (since problematic-rewriting was added) now surfaces it as a dangling
// reference — a real practical difference from real docutils' silence,
// but arguably a more honest one for a document that has an actual bug in
// it.
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

// linkReferences resolves every reference under parent. It walks by
// PARENT rather than by node (unlike the rest of this file's tree walks)
// because a dangling reference needs to REPLACE itself in its parent's own
// child slice, not just gain an attribute. anonMismatch is non-nil exactly
// when resolveTargets found the anonymous reference and target counts
// don't match document-wide — in that case EVERY anonymous reference
// becomes
// <problematic>, sharing anonMismatch as their one <system_message>
// (docutils' own "Anonymous hyperlink mismatch", a whole-document
// condition, not a per-reference one — see resolveTargets); otherwise
// anonymous references resolve by position exactly as before.
func linkReferences(parent *doctree.Element, targets map[string]string, anonTargets []string, anonMismatch *doctree.Element, anonIndex *int, messages *[]*doctree.Element, msgCount *int) {
	for i, c := range parent.Children {
		el, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		if el.Tag == doctree.TagReference {
			switch {
			case el.Attr("anonymous") == "true":
				if anonMismatch != nil {
					parent.Children[i] = problematicAnonymousReference(el, anonMismatch, msgCount)
					continue // the replacement has no children of its own to recurse into
				}
				if *anonIndex < len(anonTargets) {
					if uri := anonTargets[*anonIndex]; uri != "" {
						el.SetAttr("refuri", uri)
					}
					*anonIndex++
				}
			case el.Attr("refname") != "":
				if uri, found := targets[normalizeName(el.Attr("refname"))]; found {
					el.SetAttr("refuri", uri)
				} else {
					parent.Children[i] = problematicReference(el, messages, msgCount)
					continue // the replacement has no children of its own to recurse into
				}
			}
		}
		linkReferences(el, targets, anonTargets, anonMismatch, anonIndex, messages, msgCount)
	}
}

// problematicAnonymousReference builds the <problematic> for one
// mismatched anonymous reference, pointing at the single SHARED msg — and
// grows msg's own "backref" attribute to list every problematic that
// points to it, real docutils' own "See backrefs attribute for IDs"
// convention (space-joined here rather than a real attribute list, this
// project's doctree.Element only carries string-valued attributes).
func problematicAnonymousReference(ref *doctree.Element, msg *doctree.Element, msgCount *int) *doctree.Element {
	*msgCount++
	prbID := "problematic-" + strconv.Itoa(*msgCount)
	prb := doctree.NewElement(doctree.TagProblematic, &doctree.Text{Data: doctree.AsText(ref)})
	prb.SetAttr("id", prbID)
	prb.SetAttr("refid", msg.Attr("id"))
	if existing := msg.Attr("backref"); existing == "" {
		msg.SetAttr("backref", prbID)
	} else {
		msg.SetAttr("backref", existing+" "+prbID)
	}
	return prb
}

// problematicReference builds the <problematic>/<system_message> pair for
// one dangling named reference, cross-linked by id/refid/backref the same
// way real docutils' pair is (see resolveTargets) — appends the message to
// *messages for resolveTargets to collect into a trailing section, and
// returns the <problematic> to replace the reference with in place.
func problematicReference(ref *doctree.Element, messages *[]*doctree.Element, msgCount *int) *doctree.Element {
	*msgCount++
	n := strconv.Itoa(*msgCount)
	prb := doctree.NewElement(doctree.TagProblematic, &doctree.Text{Data: doctree.AsText(ref)})
	prb.SetAttr("id", "problematic-"+n)
	prb.SetAttr("refid", "system-message-"+n)
	msg := doctree.NewElement(doctree.TagSystemMessage,
		doctree.NewElement(doctree.TagParagraph, &doctree.Text{Data: `Unknown target name: "` + ref.Attr("refname") + `".`}))
	msg.SetAttr("id", "system-message-"+n)
	msg.SetAttr("backref", "problematic-"+n)
	*messages = append(*messages, msg)
	return prb
}

// systemMessagesSection collects every dangling-reference message into one
// trailing section, docutils' own Messages transform: "loose" system
// messages not otherwise attached to the tree get a dedicated section of
// their own, appended at the very end of the document.
func systemMessagesSection(messages []*doctree.Element) *doctree.Element {
	sec := doctree.NewElement(doctree.TagSection)
	sec.SetAttr("class", "system-messages")
	sec.Append(doctree.NewElement(doctree.TagTitle, &doctree.Text{Data: "Docutils System Messages"}))
	for _, m := range messages {
		sec.Append(m)
	}
	return sec
}
