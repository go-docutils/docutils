package rst

import (
	"regexp"
	"strings"
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

// TestURISchemesAreWhitelisted pins standalone-hyperlink recognition to
// docutils' own urischemes list. Its fixture states the rule outright:
// "None of these are standalone hyperlinks (their 'schemes' are not
// recognized): signal:noise, a:b."
//
// The list matters in BOTH directions. Without it, allowing the
// single-slash form turned every "word:word" into a link -- a field name
// like ":field:name:with:embedded:colons:" became one. And this used to
// accept ANY scheme as long as it had "://", which docutils does not.
func TestURISchemesAreWhitelisted(t *testing.T) {
	refuri := regexp.MustCompile(`refuri="([^"]*)"`)
	links := func(src string) []string {
		var out []string
		for _, m := range refuri.FindAllStringSubmatch(doctree.Dump(Parse(src)), -1) {
			out = append(out, m[1])
		}
		return out
	}
	// docutils' hierarchical part is "(//?)?" -- two slashes, one, or none.
	for _, tc := range []struct{ source, want string }{
		{"http://example.org/x\n", "http://example.org/x"},
		{"http:/one-slash-only.absolute.path\n", "http:/one-slash-only.absolute.path"},
		{"mailto:someone@somewhere.com\n", "mailto:someone@somewhere.com"},
		{"news:comp.lang.python\n", "news:comp.lang.python"},
	} {
		if got := links(tc.source); len(got) != 1 || got[0] != tc.want {
			t.Errorf("Parse(%q) links = %v, want [%q]", tc.source, got, tc.want)
		}
	}
	for _, src := range []string{"signal:noise\n", "a:b\n", "unknownscheme://x.y\n"} {
		if got := links(src); len(got) != 0 {
			t.Errorf("Parse(%q) recognized an UNKNOWN scheme: %v", src, got)
		}
	}
}

// TestEmailHostRule covers the host half of docutils' email pattern:
// "[chars]+" followed by a separate FINAL URI char group, so a host needs
// at least TWO characters and must end on one of [_~*/=+a-zA-Z0-9]. The
// rule this replaced -- "the domain must contain a dot" -- appears
// nowhere in that pattern and rejected both cases below.
func TestEmailHostRule(t *testing.T) {
	refuri := regexp.MustCompile(`refuri="([^"]*)"`)
	for _, tc := range []struct{ source, want string }{
		{"see user@host now\n", "mailto:user@host"},
		{"a@b-c\n", "mailto:a@b-c"},
		{"x@y.com.\n", "mailto:x@y.com"},
		// Stops before the "?", which is not a valid FINAL character.
		{"(a.question.mark@end?)\n", "mailto:a.question.mark@end"},
	} {
		m := refuri.FindStringSubmatch(doctree.Dump(Parse(tc.source)))
		if m == nil || m[1] != tc.want {
			t.Errorf("Parse(%q) = %v, want %q", tc.source, m, tc.want)
		}
	}
	// A single-character host has nothing left for the final group.
	if got := doctree.Dump(Parse("a@b\n")); strings.Contains(got, "refuri=") {
		t.Errorf(`"a@b" was treated as an address:`+"\n%s", got)
	}
}
