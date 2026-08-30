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
// around x), simple
// named reference (`x`_ and x_) and anonymous reference (x__), and
// backslash escapes. NOT yet ported: interpreted text roles (`x`:role:),
// substitution references, footnote/citation references, embedded URIs
// in phrase references, standalone URI/email/PEP/RFC recognition, inline
// internal targets. Content between markers is treated as plain text and
// is not re-parsed for further inline markup, matching docutils' actual
// (not merely documented) behavior: nested inline markup does not match.
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

// marker describes one inline-markup delimiter pair.
type marker struct {
	open string
	tag  string // doctree tag, or "" for reference handling
}

var markers = []marker{
	{"**", doctree.TagStrong},
	{"``", doctree.TagLiteral},
	{"*", doctree.TagEmphasis},
	{"`", ""}, // interpreted/phrase reference, handled specially below
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
		if m.tag == "" {
			el, extra := referenceOrPhrase(content, closeAt+closeLen, runes)
			return el, total + extra, true
		}
		el := doctree.NewElement(m.tag, &doctree.Text{Data: content})
		return el, total, true
	}
	return nil, 0, false
}

// referenceOrPhrase builds a `text`_ / `text`__ reference node, checking
// the text immediately after the closing backquote for the trailing
// underscore(s), and returns how many extra runes that consumed.
func referenceOrPhrase(content string, afterClose int, runes []rune) (*doctree.Element, int) {
	anonymous := false
	extra := 0
	if afterClose < len(runes) && runes[afterClose] == '_' {
		extra++
		if afterClose+1 < len(runes) && runes[afterClose+1] == '_' {
			anonymous = true
			extra++
		}
	}
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: content})
	if anonymous {
		el.SetAttr("anonymous", "true")
	}
	el.SetAttr("refname", content)
	return el, extra
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
