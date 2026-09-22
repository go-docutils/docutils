package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestIsUniformLine(t *testing.T) {
	cases := []struct {
		s        string
		wantChar rune
		wantOK   bool
	}{
		{"====", '=', true},
		{"----", '-', true},
		{"", 0, false},
		{"   ", 0, false},
		{"abc", 0, false},
		{"=-=", 0, false},
		{"==  ", '=', true}, // trailing spaces ignored
	}
	for _, tc := range cases {
		char, ok := isUniformLine(tc.s)
		if ok != tc.wantOK || (ok && char != tc.wantChar) {
			t.Errorf("isUniformLine(%q) = (%q, %v), want (%q, %v)", tc.s, char, ok, tc.wantChar, tc.wantOK)
		}
	}
}

func TestIsBulletLine(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"- item", true},
		{"-", true},
		{"-item", false}, // no space after marker, not end of line either
		{"", false},
		{"* item", true},
		{"a item", false},
	}
	for _, tc := range cases {
		if got := isBulletLine(tc.s); got != tc.want {
			t.Errorf("isBulletLine(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestIsEnumLine(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"1. item", true},
		{"1.", true},
		{"1.item", false},
		// "a." is a valid loweralpha enumerator shape (states.py's own
		// sequencepats union, read directly) — an earlier version of
		// this test predates alpha/roman enumerator support and wrongly
		// expected false.
		{"a. item", true},
		{"", false},
		{"12. item", true},
	}
	for _, tc := range cases {
		if got := isEnumLine(tc.s); got != tc.want {
			t.Errorf("isEnumLine(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestTrimTrailingSpace(t *testing.T) {
	cases := map[string]string{
		"abc  ": "abc",
		"abc":   "abc",
		"   ":   "",
		"":      "",
	}
	for in, want := range cases {
		if got := trimTrailingSpace(in); got != want {
			t.Errorf("trimTrailingSpace(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTrailingWhitespaceAndEscape covers two per-line rules that are
// Python's and not Go's.
//
// string2lines rstrips every line, and Python's str.rstrip() strips
// whatever str.isspace() accepts -- a no-break space, an em space, an
// ideographic space -- where this stopped at the ASCII space. And a
// backslash with nothing after it escapes nothing: escape2null appends
// a lone null, which unescape then removes, so the backslash is GONE.
//
// unicode.IsSpace was measured against str.isspace() rather than
// assumed, after two rounds of exactly that mistake: it agrees on all
// 29 except U+001C..U+001F, which Python calls whitespace and Go does
// not. U+200B is in neither set and must SURVIVE -- that is the case
// that keeps the fix from becoming "strip anything invisible".
func TestTrailingWhitespaceAndEscape(t *testing.T) {
	t.Run("rstrip", func(t *testing.T) {
		for _, tc := range []struct{ name, source, wantTerm string }{
			{"ascii space, the control", "term \n    definition\n", "term"},
			{"no-break space", "term\u00a0\n    definition\n", "term"},
			{"em space", "term\u2003\n    definition\n", "term"},
			{"ideographic space", "term\u3000\n    definition\n", "term"},
			{"unit separator", "term\u001f\n    definition\n", "term"},
			// NOT whitespace in either language: it stays.
			{"zero width space survives", "term\u200b\n    definition\n", "term\u200b"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// <definition_list>/<definition_list_item>/<term>/text
				want := "\n                " + tc.wantTerm + "\n"
				if got := doctree.Dump(Parse(tc.source)); !strings.Contains(got, want) {
					t.Errorf("term %q not found in:\n%s", tc.wantTerm, got)
				}
			})
		}
	})
	t.Run("a trailing backslash disappears", func(t *testing.T) {
		for _, tc := range []struct{ name, source, want string }{
			{"at the end of the document", "para\\\n", "<document>\n    <paragraph>\n        para\n"},
			// Already correct before: the escape had a newline to eat.
			{"before another line, the control", "para\\\nsecond\n", "<document>\n    <paragraph>\n        parasecond\n"},
			// And a DOUBLED backslash is one literal backslash.
			{"a doubled backslash is literal", "a\\\\b\n", "<document>\n    <paragraph>\n        a\\b\n"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := doctree.Dump(Parse(tc.source)); got != tc.want {
					t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
				}
			})
		}
	})
}
