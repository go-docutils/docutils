package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func doctreeDump(src string) string { return doctree.Dump(Parse(src)) }

func TestSplitEmbeddedLink(t *testing.T) {
	cases := []struct {
		content     string
		wantDisplay string
		wantTarget  string
		wantKind    string
		wantOK      bool
	}{
		{"Python <https://python.org>", "Python", "https://python.org", "uri", true},
		{"alias <target_>", "alias", "target", "name", true},
		{"Jane <jane@example.com>", "Jane", "jane@example.com", "uri", true},
		{"no angle brackets here", "", "", "", false},
		{"missing space<https://example.com>", "", "", "", false},
		{"empty target <>", "", "", "", false},
	}
	for _, tc := range cases {
		display, kind, targetRunes, ok := splitEmbeddedLink(escapeBackslashes(tc.content))
		target := string(targetRunes)
		if ok != tc.wantOK || display != tc.wantDisplay || target != tc.wantTarget || kind != tc.wantKind {
			t.Errorf("splitEmbeddedLink(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				tc.content, display, target, kind, ok,
				tc.wantDisplay, tc.wantTarget, tc.wantKind, tc.wantOK)
		}
	}
}

func TestJoinEmbeddedURI(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"http://example.com/long/path", "http://example.com/long/path"},
		{"http://example.com/\nlong/path", "http://example.com/long/path"},
		{"http://example.com/\nlong/path /and  /whitespace", "http://example.com/long/path/and/whitespace"},
		{"http://example.com/a\\ long/path\\ and/some\\ escaped\\ whitespace", "http://example.com/a long/path and/some escaped whitespace"},
	}
	for _, tc := range cases {
		got := joinEmbeddedURI(escapeBackslashes(tc.content))
		if got != tc.want {
			t.Errorf("joinEmbeddedURI(%q) = %q, want %q", tc.content, got, tc.want)
		}
	}
}

func TestAdjustEmbeddedURI(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"jane@example.com", "mailto:jane@example.com"},
	}
	for _, tc := range cases {
		if got := adjustEmbeddedURI(tc.uri); got != tc.want {
			t.Errorf("adjustEmbeddedURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

// TestInlineLiteralBackslashes pins the one place docutils' end-string
// lookbehind deliberately DIFFERS between markers. As built in states.py:
//
//	emphasis: (?<![\s\x00])(\*)($|(?=...))
//	literal:  (?<!\s)(``)($|(?=...))
//
// Only emphasis (and every other marker) refuses a delimiter carrying the
// \x00 escape marker; the literal form has no \x00 in its lookbehind at
// all. That is the spec's "backslashes are not escapes inside inline
// literals" made mechanical -- a backslash neither protects the closing
// backquotes nor disappears from the content. Every case here was run
// against the reference implementation.
func TestInlineLiteralBackslashes(t *testing.T) {
	cases := []struct{ source, want string }{
		// The backslash does NOT protect the close; it stays as content.
		{"``literal\\``\n", "<document>\n    <paragraph>\n        <literal>\n            literal\\\n"},
		// An escaped backquote mid-content is kept verbatim, backslash and
		// all, and the real close is still found afterwards.
		{"``a\\`b``\n", "<document>\n    <paragraph>\n        <literal>\n            a\\`b\n"},
		// Two backslashes stay two backslashes -- no collapsing either.
		{"``a\\\\``\n", "<document>\n    <paragraph>\n        <literal>\n            a\\\\\n"},
		// The end-boundary rule still applies after the close.
		{"``a\\`` b\n", "<document>\n    <paragraph>\n        <literal>\n            a\\\n         b\n"},
		// The CONTRAST case: emphasis does honor the escape, so this is
		// one <emphasis> spanning the escaped asterisk, not two.
		{"*a\\*b*\n", "<document>\n    <paragraph>\n        <emphasis>\n            a*b\n"},
	}
	for _, tc := range cases {
		if got := doctreeDump(tc.source); got != tc.want {
			t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
		}
	}
}

// TestPipesAreNotSubstitutionStarts covers docutils' substitution
// start-string `\|(?!\|)`: a "|" followed by another "|" is not a
// start-string at all. Without the negative lookahead this opened a
// substitution reference at the first "|" of the "||" pair and swallowed
// the rest of the line.
func TestPipesAreNotSubstitutionStarts(t *testing.T) {
	src := "first | then || and finally |||\n"
	want := "<document>\n    <paragraph>\n        first | then || and finally |||\n"
	if got := doctreeDump(src); got != want {
		t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", src, got, want)
	}
}

// TestDumpDoesNotEscapeQuotesInAttributes pins Dump to docutils'
// nodes.pseudo_quoteattr, which is literally `'"%s"' % value` -- it
// escapes nothing, producing technically-invalid XML for a value holding
// a quote and not caring, because pseudoxml is a debug format.
func TestDumpDoesNotEscapeQuotesInAttributes(t *testing.T) {
	got := doctreeDump("_`\"target2\"` with quotes\n")
	if !strings.Contains(got, `name=""target2""`) {
		t.Errorf("attribute quote was escaped rather than passed through:\n%s", got)
	}
}
