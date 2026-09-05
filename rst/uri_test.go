package rst

import (
	"regexp"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestStandaloneURICharacterClasses pins standalone URI recognition to
// docutils' own two character classes rather than the "everything that is
// not whitespace, then trim likely punctuation" heuristic this used to
// use. From its uri pattern:
//
//	body:  [-_.!~*'()[\];/:@&=+$,%a-zA-Z0-9]
//	final: [_~*/=+a-zA-Z0-9]   OR   any body char followed by ">"
//
// The body class contains neither "<" nor ">", which is what makes a URI
// inside angle brackets end at the bracket; the second alternative for the
// final character is what lets it KEEP punctuation there that the same URI
// would lose in running prose. Every expectation below was read off the
// reference implementation.
func TestStandaloneURICharacterClasses(t *testing.T) {
	refuri := regexp.MustCompile(`refuri="([^"]*)"`)
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"a sentence's full stop is not part of the URI", "see http://example.org/x. done\n", "http://example.org/x"},
		{"inside angle brackets the same full stop IS part of it", "<http://example.org/x.>\n", "http://example.org/x."},
		{"a wrapping parenthesis is dropped", "(http://example.org/p)\n", "http://example.org/p"},
		{"a query string survives", "http://example.org/q?a=1&b=2 end\n", "http://example.org/q?a=1&b=2"},
		{"a fragment survives, its trailing stop does not", "http://example.org/f#frag.\n", "http://example.org/f#frag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := refuri.FindStringSubmatch(doctree.Dump(Parse(tc.source)))
			if m == nil {
				t.Fatalf("Parse(%q) produced no reference at all", tc.source)
			}
			if m[1] != tc.want {
				t.Errorf("Parse(%q) refuri = %q, want %q", tc.source, m[1], tc.want)
			}
		})
	}
}

// KNOWN DIVERGENCE, deliberately not chased: a URI whose path segment ends
// in an underscore. "http://a/b_ end" is, in real docutils, a reference to
// "http://a/" followed by a separate bare reference "b_", because its
// implicit_inline tries the reference pattern alongside the URI one and
// that alternative wins for the tail. This parser keeps the whole
// "http://a/b_" as one URI. Matching it means modelling docutils' pattern
// ALTERNATION order, not just its character classes -- a much larger
// change, and no corpus fixture exercises the shape.
