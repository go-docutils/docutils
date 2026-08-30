package rst

import (
	"strings"
	"unicode"

	"github.com/go-docutils/docutils/doctree"
)

// inliner parses reStructuredText inline markup within a single block of
// text (a paragraph, title, etc.), modeled on docutils.parsers.rst.states.
// Inliner.parse.
//
// SCOPE (v1): strong (**x**), emphasis (*x*), literal (two backticks
// around x), interpreted text with a role — prefix (:role:`x`) or
// suffix (`x`:role:) — for the small fixed set of docutils' built-in
// GENERIC roles (emphasis, strong, literal, subscript/sub,
// superscript/sup, title-reference/title/t, abbreviation/ab,
// acronym/ac; see roleTags) plus a bare `x` with no role at all
// (docutils' DEFAULT role, title_reference); any OTHER role name falls
// back to a generic <inline role="name"> instead of docutils' `.. role
// ::`-registry dispatch (there is no role registry here, same
// no-registry philosophy as directives) — real docutils also ERRORS on
// a role it cannot find at all (rewriting it to `problematic`), which
// this parser does not replicate, so an unknown role never fails, it
// just becomes generic. Named/anonymous reference both bare (x_, x__)
// and backtick-quoted (`x`_, `x`__) including an embedded URI or
// indirect name-alias target (`text <https://example.com>`_, `text
// <alias_>`_), substitution reference (|x|), footnote/citation
// reference ([1]_ / [#]_ / [#name]_ / [*]_ / [name]_), and backslash
// escapes. NOT yet ported: docutils' non-generic built-in roles (code,
// math, pep-reference, rfc-reference, raw — each has real, non-generic
// behavior this doesn't replicate), standalone URI/email/PEP/RFC
// recognition, inline internal targets, a substitution reference used
// as a hyperlink (|x|_ / |x|__), the extra <target> sibling docutils
// emits next to a resolved embedded-link reference (this parser sets
// refuri/refname directly on the <reference> instead — reference
// resolution still works the same way since resolveTargets matches by
// name, it just doesn't produce that second node). Content between
// markers is treated as plain text and is not re-parsed for further
// inline markup, matching docutils' actual (not merely documented)
// behavior: nested inline markup does not match.
func parseInline(text string) []doctree.Node {
	runes := escapeBackslashes(text)
	var out []doctree.Node
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, &doctree.Text{Data: buf.String()})
			buf.Reset()
		}
	}

	i := 0
	n := len(runes)
	for i < n {
		if node, consumed, ok := tryFootnoteRef(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := tryBareReference(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := tryInterpretedOrPhraseRef(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := tryMarker(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		buf.WriteRune(unescapeRune(runes[i]))
		i++
	}
	flush()
	return out
}

// escapedBase marks a backslash-escaped character: shifted into the
// Supplementary Private Use Area-A so it can never collide with a real
// marker rune ('*', '`', '|', ...) during matching, mirroring what
// docutils.utils.escape2null does with a NUL-byte prefix. Escaped runes
// are restored to their literal form wherever they end up as output text
// (see unescapeRune / unescapeRunes) — the point is only that they must
// never be RECOGNIZED as markup delimiters.
const escapedBase = 0xF0000

func escapeRune(r rune) rune { return escapedBase + r }

func isEscapedRune(r rune) bool { return r >= escapedBase && r < escapedBase+0x10000 }

func unescapeRune(r rune) rune {
	if isEscapedRune(r) {
		return r - escapedBase
	}
	return r
}

func unescapeRunes(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		b.WriteRune(unescapeRune(r))
	}
	return b.String()
}

// escapeBackslashes returns text as runes with every backslash-escaped
// character (`\X`) replaced by its escaped form, so later marker matching
// can simply refuse to treat an escaped rune as a delimiter.
func escapeBackslashes(s string) []rune {
	src := []rune(s)
	out := make([]rune, 0, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) {
			out = append(out, escapeRune(src[i+1]))
			i++
			continue
		}
		out = append(out, src[i])
	}
	return out
}

// marker describes one inline-markup delimiter pair. Backtick-quoted
// text (interpreted text / phrase references) is NOT in this table —
// its content needs role/underscore lookahead beyond "does the closing
// delimiter match", so it's handled separately by
// tryInterpretedOrPhraseRef.
type marker struct {
	open string
	tag  string
}

var markers = []marker{
	{"**", doctree.TagStrong},
	{"``", doctree.TagLiteral},
	{"*", doctree.TagEmphasis},
	{"|", doctree.TagSubstitutionRef},
}

// tryMarker attempts to match an inline-markup construct starting at
// runes[i]. Returns the produced node, the number of runes consumed, and
// whether a match was found.
func tryMarker(runes []rune, i int) (doctree.Node, int, bool) {
	for _, m := range markers {
		ol := len([]rune(m.open))
		if !hasPrefixAt(runes, i, m.open) {
			continue
		}
		if !validStartBoundary(runes, i, ol) {
			continue
		}
		// find matching close, honoring end-string-suffix boundary rule
		closeAt, closeLen, ok := findClose(runes, i+ol, m.open)
		if !ok {
			continue
		}
		content := unescapeRunes(runes[i+ol : closeAt])
		if content == "" {
			continue // start-string immediately followed by end-string: no match
		}
		total := (closeAt + closeLen) - i
		el := doctree.NewElement(m.tag, &doctree.Text{Data: content})
		if m.tag == doctree.TagSubstitutionRef {
			el.SetAttr("refname", normalizeWhitespace(content))
		}
		return el, total, true
	}
	return nil, 0, false
}

// referenceOrPhrase builds either a title_reference (bare `text`, no
// trailing underscore — docutils' DEFAULT role) or a `text`_ / `text`__
// reference node (checking the text immediately after the closing
// backquote for the trailing underscore(s)), and returns how many extra
// runes that consumed.
func referenceOrPhrase(content string, afterClose int, runes []rune) (*doctree.Element, int) {
	if !(afterClose < len(runes) && runes[afterClose] == '_') {
		return doctree.NewElement(doctree.TagTitleReference, &doctree.Text{Data: content}), 0
	}
	anonymous := false
	extra := 1
	if afterClose+1 < len(runes) && runes[afterClose+1] == '_' {
		anonymous = true
		extra = 2
	}
	display, target, kind, hasEmbedded := splitEmbeddedLink(content)
	text := content
	if hasEmbedded {
		text = display
	}
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: text})
	el.SetAttr("name", normalizeWhitespace(text))
	switch {
	case hasEmbedded && kind == "uri":
		el.SetAttr("refuri", target)
	case hasEmbedded && kind == "name":
		el.SetAttr("refname", normalizeName(target))
	case anonymous:
		el.SetAttr("anonymous", "true")
	default:
		el.SetAttr("refname", normalizeName(text))
	}
	return el, extra
}

// splitEmbeddedLink recognizes docutils' "embedded URI or alias" form
// of a phrase reference: `display text <target>`_, where <target> is
// either a URI (kind "uri") or, when it ends in "_" and doesn't look
// like a URI, the name of another target defined elsewhere (kind
// "name"). The "<" must be preceded by whitespace or start the string,
// matching docutils' embedded_link pattern.
func splitEmbeddedLink(content string) (display, target, kind string, ok bool) {
	if !strings.HasSuffix(content, ">") {
		return "", "", "", false
	}
	idx := strings.LastIndexByte(content, '<')
	if idx < 0 || !(idx == 0 || content[idx-1] == ' ') {
		return "", "", "", false
	}
	target = content[idx+1 : len(content)-1]
	if target == "" {
		return "", "", "", false
	}
	display = strings.TrimRight(content[:idx], " ")
	if strings.HasSuffix(target, "_") && !strings.Contains(target, "://") {
		return display, target[:len(target)-1], "name", true
	}
	if strings.Contains(target, "@") && !strings.Contains(target, "://") {
		target = "mailto:" + target
	}
	return display, target, "uri", true
}

// tryInterpretedOrPhraseRef handles every backtick-quoted construct:
// role-prefixed (:role:`x`), role-suffixed (`x`:role:), a plain phrase
// reference or bare title_reference (referenceOrPhrase, no role at
// all). docutils.parsers.rst.states.Inliner.interpreted_or_phrase_ref.
func tryInterpretedOrPhraseRef(runes []rune, i int) (doctree.Node, int, bool) {
	backtickAt := i
	prefixRole := ""
	if runes[i] == ':' {
		role, afterColon, ok := tryRoleName(runes, i+1)
		if !ok || afterColon >= len(runes) || runes[afterColon] != '`' {
			return nil, 0, false
		}
		prefixRole = role
		backtickAt = afterColon
	}
	if runes[backtickAt] != '`' {
		return nil, 0, false
	}
	if backtickAt+1 < len(runes) && runes[backtickAt+1] == '`' {
		// A single backtick immediately followed by another is the
		// START of a double-backtick literal (``x``), not interpreted
		// text — docutils' backquote pattern is '`(?!`)'. Let tryMarker
		// handle it instead.
		return nil, 0, false
	}
	if !validStartBoundary(runes, i, backtickAt-i+1) {
		return nil, 0, false
	}
	closeAt, closeLen, ok := findClose(runes, backtickAt+1, "`")
	if !ok {
		return nil, 0, false
	}
	content := unescapeRunes(runes[backtickAt+1 : closeAt])
	if content == "" {
		return nil, 0, false
	}
	afterClose := closeAt + closeLen

	if prefixRole != "" {
		return roleElement(prefixRole, content), afterClose - i, true
	}
	if afterClose < len(runes) && runes[afterClose] == ':' {
		if role, afterRole, ok := tryRoleName(runes, afterClose+1); ok {
			return roleElement(role, content), afterRole - i, true
		}
	}
	el, extra := referenceOrPhrase(content, afterClose, runes)
	return el, (afterClose - i) + extra, true
}

// tryRoleName recognizes a ":name" immediately followed by a closing
// ":" starting at `from` (the position right after the opening ":"),
// returning the role name and the position right after the closing ":".
func tryRoleName(runes []rune, from int) (name string, after int, ok bool) {
	j := from
	for j < len(runes) && isRoleNameChar(runes[j]) {
		j++
	}
	if j == from || j >= len(runes) || runes[j] != ':' {
		return "", 0, false
	}
	return string(runes[from:j]), j + 1, true
}

func isRoleNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-'
}

// roleTags maps docutils' built-in GENERIC role canonical/alias names
// (docutils.parsers.rst.languages.en.roles, English only — this parser
// doesn't support other languages' role names) to the doctree tag they
// produce. A role not in this table falls back to a generic <inline>.
var roleTags = map[string]string{
	"emphasis":        doctree.TagEmphasis,
	"strong":          doctree.TagStrong,
	"literal":         doctree.TagLiteral,
	"subscript":       doctree.TagSubscript,
	"sub":             doctree.TagSubscript,
	"superscript":     doctree.TagSuperscript,
	"sup":             doctree.TagSuperscript,
	"title-reference": doctree.TagTitleReference,
	"title":           doctree.TagTitleReference,
	"t":               doctree.TagTitleReference,
	"abbreviation":    doctree.TagAbbreviation,
	"ab":              doctree.TagAbbreviation,
	"acronym":         doctree.TagAcronym,
	"ac":              doctree.TagAcronym,
}

func roleElement(role, content string) *doctree.Element {
	if tag, ok := roleTags[strings.ToLower(role)]; ok {
		return doctree.NewElement(tag, &doctree.Text{Data: content})
	}
	el := doctree.NewElement(doctree.TagInline, &doctree.Text{Data: content})
	el.SetAttr("role", strings.ToLower(role))
	return el
}

// tryBareReference recognizes a reference with no backtick delimiters
// at all: word_ / word__ (docutils' 'whole' construct: a `simplename`
// immediately followed by one or two trailing underscores). Only
// letters, digits, and hyphens make up the name here — an internal "_"
// would itself look like the very suffix this is trying to detect, so
// (like real reST) a name containing one isn't reachable through this
// bare form; write it backtick-quoted instead.
func tryBareReference(runes []rune, i int) (doctree.Node, int, bool) {
	if !isSimpleNameChar(runes[i]) || !validStartBoundary(runes, i, 0) {
		return nil, 0, false
	}
	j := i
	for j < len(runes) && isSimpleNameChar(runes[j]) {
		j++
	}
	if j >= len(runes) || runes[j] != '_' {
		return nil, 0, false
	}
	anonymous := false
	end := j + 1
	if end < len(runes) && runes[end] == '_' {
		anonymous = true
		end++
	}
	if end < len(runes) {
		next := runes[end]
		if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
			return nil, 0, false
		}
	}
	name := unescapeRunes(runes[i:j])
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: name})
	el.SetAttr("name", normalizeWhitespace(name))
	if anonymous {
		el.SetAttr("anonymous", "true")
	} else {
		el.SetAttr("refname", normalizeName(name))
	}
	return el, end - i, true
}

func isSimpleNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-'
}

// tryFootnoteRef recognizes "[label]_" — a footnote reference ("[1]_",
// "[#]_", "[#name]_", "[*]_") or citation reference ("[name]_"),
// docutils' Inliner.footnote_reference. Unlike the phrase-reference
// form, only a single trailing "_" is meaningful here (no "__" form).
func tryFootnoteRef(runes []rune, i int) (doctree.Node, int, bool) {
	if runes[i] != '[' || !validStartBoundary(runes, i, 1) {
		return nil, 0, false
	}
	end := -1
	for j := i + 1; j < len(runes); j++ {
		if runes[j] == ']' {
			end = j
			break
		}
	}
	if end < 0 || end+1 >= len(runes) || runes[end+1] != '_' {
		return nil, 0, false
	}
	label := unescapeRunes(runes[i+1 : end])
	if label == "" {
		return nil, 0, false
	}
	total := end + 2 - i
	if end+2 < len(runes) {
		next := runes[end+2]
		if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
			return nil, 0, false
		}
	}

	var el *doctree.Element
	switch {
	case label == "*":
		el = doctree.NewElement(doctree.TagFootnoteReference)
		el.SetAttr("auto", "*")
	case label[0] == '#':
		el = doctree.NewElement(doctree.TagFootnoteReference)
		el.SetAttr("auto", "1")
		if name := label[1:]; name != "" {
			el.SetAttr("refname", normalizeName(name))
		}
	case isAllDigits(label):
		el = doctree.NewElement(doctree.TagFootnoteReference, &doctree.Text{Data: label})
		el.SetAttr("refname", label)
	default:
		el = doctree.NewElement(doctree.TagCitationReference, &doctree.Text{Data: label})
		el.SetAttr("refname", normalizeName(label))
	}
	return el, total, true
}

func hasPrefixAt(runes []rune, i int, s string) bool {
	rs := []rune(s)
	if i+len(rs) > len(runes) {
		return false
	}
	for k, r := range rs {
		if runes[i+k] != r {
			return false
		}
	}
	return true
}

// validStartBoundary implements a simplified version of docutils'
// start-string-prefix: must be at the beginning of the text, or preceded
// by whitespace or opening punctuation, AND not immediately followed by
// whitespace. The full implementation keys off Unicode General Category
// tables (docutils.utils.punctuation_chars); this uses unicode.IsSpace /
// unicode.IsPunct as a documented approximation, close enough for ASCII
// prose but a known fidelity gap for exotic Unicode punctuation.
func validStartBoundary(runes []rune, i, openLen int) bool {
	if i > 0 {
		prev := runes[i-1]
		if !unicode.IsSpace(prev) && !unicode.IsPunct(prev) {
			return false
		}
	}
	if i+openLen < len(runes) && unicode.IsSpace(runes[i+openLen]) {
		return false
	}
	return true
}

// findClose scans forward from `from` for the next occurrence of `open`
// (the close-string is the same characters as the open-string in reST)
// satisfying the end-string-suffix boundary: not immediately preceded by
// whitespace, and followed by end-of-text, whitespace, or punctuation.
func findClose(runes []rune, from int, open string) (int, int, bool) {
	ol := len([]rune(open))
	for j := from; j <= len(runes)-ol; j++ {
		if !hasPrefixAt(runes, j, open) {
			continue
		}
		if j == from {
			continue // empty content, e.g. "****"
		}
		prev := runes[j-1]
		if unicode.IsSpace(prev) {
			continue
		}
		if j+ol < len(runes) {
			next := runes[j+ol]
			if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
				continue
			}
		}
		return j, ol, true
	}
	return 0, 0, false
}
