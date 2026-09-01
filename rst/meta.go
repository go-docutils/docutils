package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.misc's Meta + its own
// specialized MetaBody state class (both read directly, though the
// class itself now lives in misc.py — html.py's Meta is a deprecated
// re-export docutils itself warns about). Meta's content is a run of
// field markers reusing the SAME grammar every field list already uses
// (matchFieldMarker, fieldlist.go) — but unlike a field list, a marker's
// own NAME text is not inline-parsed at all, just backslash-unescaped
// (nodes.unescape), then split on whitespace into tokens: the first
// token is either "attr=value" (sets that attribute directly, e.g.
// ":http-equiv=Content-Type:") or a bare word (sets "name", the common
// case); every token AFTER the first MUST be "attr=value" or it's a
// real per-field ERROR (":name notattval:"). A marker's own
// continuation lines join with a SPACE (not a newline) before the SAME
// unescape pass — the reason a trailing backslash immediately before a
// line break disappears along with the break itself rather than leaving
// a literal space (an escaped-space/escaped-newline is DROPPED, not
// rendered — the exact same rule this project's own inline-markup
// boundary handling already implements, reused here via
// escapeBackslashes/unescapeRunes rather than reinvented). Parsing STOPS
// at the first content line that isn't a field marker at all (real
// docutils' own SpecializedBody state machine simply has no other
// transition to fall to there) — anything already produced before that
// point survives; the directive's WHOLE result additionally gets one
// more diagnostic appended when that leaves content unconsumed.
//
// The distinctive part: NONE of a ".. meta::" directive's own result
// nodes stay at their lexical position in the tree — real docutils
// splices them directly into the DOCUMENT ROOT's own children
// (self.state.document[index:index] = node.children, always the
// top-level document, however deeply the directive itself was nested)
// at the first position not already a leading run of Titular/meta
// siblings. This project defers the actual move to hoistMetaNodes (a
// single post-pass at the end of Parse, the same "equivalent effect,
// deferred to a whole-document pass" shape resolveTargets/
// resolveFootnoteNumbers already use for their own document-wide
// effects) rather than mutating doc mid-parse — see parser.metaNodes.

// runMetaDirective implements Meta.run + MetaBody.field_marker/
// parsemeta together. Its results are accumulated into p.metaNodes, NOT
// returned to the caller for normal in-place insertion — matching real
// docutils' own Meta.run, which always returns an empty list.
func (p *parser) runMetaDirective(lines []string, i int, body []string, lineno int, blockText string) {
	content := body
	for len(content) > 0 && isBlankStr(content[len(content)-1]) {
		content = content[:len(content)-1]
	}
	if len(content) == 0 {
		p.metaNodes = append(p.metaNodes, sectionMessage("3", "ERROR",
			`Content block expected for the "meta" directive; none found.`, lineno, blockText))
		return
	}

	bodyStart := i + 1
	for bodyStart < len(lines) && isBlankStr(lines[bodyStart]) {
		bodyStart++
	}

	j := 0
	for j < len(content) {
		name, col, ok := matchFieldMarker(content[j])
		if !ok {
			break
		}
		first := ""
		if len(content[j]) > col {
			first = content[j][col:]
		}
		fieldLine := bodyStart + j + 1
		markerLine := content[j]
		valueLines, next := gatherListItemLines(content, j, col, first)
		p.metaNodes = append(p.metaNodes, buildMetaField(name, valueLines, fieldLine, markerLine))
		j = next
		for j < len(content) && isBlankStr(content[j]) {
			j++
		}
	}
	if j != len(content) {
		p.metaNodes = append(p.metaNodes, sectionMessage("3", "ERROR", "Invalid meta directive.", lineno, blockText))
	}
}

// buildMetaField mirrors MetaBody.parsemeta for one field marker.
func buildMetaField(rawName string, valueLines []string, fieldLine int, markerLine string) doctree.Node {
	name := unescapeRunes(escapeBackslashes(rawName))
	if allBlank(valueLines) {
		return sectionMessage("1", "INFO", `No content for meta tag "`+name+`".`, fieldLine, markerLine)
	}
	content := unescapeRunes(escapeBackslashes(strings.Join(valueLines, " ")))
	el := doctree.NewElement(doctree.TagMeta)
	el.SetAttr("content", content)
	tokens := strings.Fields(name)
	if attname, val, ok := extractNameValue(tokens[0]); ok {
		el.SetAttr(strings.ToLower(attname), val)
	} else {
		el.SetAttr("name", tokens[0])
	}
	for _, tok := range tokens[1:] {
		attname, val, ok := extractNameValue(tok)
		if !ok {
			return sectionMessage("3", "ERROR", `Error parsing meta tag attribute "`+tok+`": missing "=".`, fieldLine, markerLine)
		}
		el.SetAttr(strings.ToLower(attname), val)
	}
	return el
}

// extractNameValue mirrors utils.extract_name_value for the single
// "attr=value" token shape meta ever hands it (its own caller already
// splits on whitespace first, so a quoted value with an embedded space
// — extract_name_value's own quote-handling exists for a different
// caller — can never reach here intact either, matching real docutils'
// own effective behavior for this specific directive).
func extractNameValue(token string) (name, value string, ok bool) {
	idx := strings.IndexByte(token, '=')
	if idx <= 0 {
		return "", "", false
	}
	return token[:idx], token[idx+1:], true
}

// hoistMetaNodes splices every accumulated ".. meta::" result node into
// the document root's own children, at the front — real docutils'
// first_child_not_matching_class((Titular, meta)) skips past a leading
// run of title/subtitle/already-hoisted-meta siblings; this project's
// own document model never has a top-level Titular child at all (a
// section keeps its own <title>, never promoted to one — see the
// package's own scope notes), so in practice only an existing leading
// <meta> run matters, which metaNodes' own accumulation already keeps
// contiguous. A leading <field_list>/<docinfo> is ALSO skipped here,
// even though real docutils' own exclusion set doesn't name it:
// hoisting meta nodes ahead of an unpromoted field list would shift it
// off document position 0, silently breaking promoteDocInfo's own
// strict "docinfo must be the very first child" check (docinfo.go, read
// directly) — no corpus case combines the two, so this is a deliberate,
// safety-motivated divergence, not a byte-exact port.
func hoistMetaNodes(doc *doctree.Element, metaNodes []doctree.Node) {
	if len(metaNodes) == 0 {
		return
	}
	insertAt := 0
	if len(doc.Children) > 0 {
		if el, ok := doc.Children[0].(*doctree.Element); ok &&
			(el.Tag == doctree.TagFieldList || el.Tag == doctree.TagDocinfo) {
			insertAt = 1
		}
	}
	front := append(append([]doctree.Node{}, doc.Children[:insertAt]...), metaNodes...)
	doc.Children = append(front, doc.Children[insertAt:]...)
}
