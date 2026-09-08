package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestPhraseTargetName pins the BACKQUOTED form of a hyperlink target
// name. Body.patterns.target makes the quote optional and pairs it with
// a back-reference, so the quotes are DELIMITERS -- not part of the name
// -- and a colon inside them does not terminate it.
//
// This parser kept the quotes in the name while stripping them from the
// id (make_id drops them anyway), so the two disagreed. Every phrase
// target in pytest's and sphinx's documentation is written this way: 22
// real-world files, and no docutils testsuite fixture.
//
// Each expectation is the reference's own.
func TestPhraseTargetName(t *testing.T) {
	cases := []struct {
		name, source, wantName, wantURI string
	}{
		{"a quoted phrase loses its quotes", ".. _`a phrase`:\n\ntext\n", "a phrase", ""},
		{
			// The case the quotes exist FOR: inside them, a colon is
			// ordinary text rather than the name's terminator.
			"a colon inside the quotes does not end the name",
			".. _`with: colon`:\n\ntext\n", "with: colon", "",
		},
		{"a quoted name with a URI", ".. _`x`: http://e.org\n\ntext\n", "x", "http://e.org"},
		{"an unquoted name is unchanged", ".. _plain:\n\ntext\n", "plain", ""},
		{
			// And the rule that made the unquoted form tricky in the
			// first place (v0.75.0) still holds.
			"an unquoted name may itself contain colons",
			".. _figure:caption:\n\ntext\n", "figure:caption", "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if want := `name="` + tc.wantName + `"`; !strings.Contains(got, want) {
				t.Errorf("missing %s:\n%s", want, got)
			}
			switch tc.wantURI {
			case "":
				if strings.Contains(got, "refuri=") {
					t.Errorf("a URI-less target gained a refuri:\n%s", got)
				}
			default:
				if want := `refuri="` + tc.wantURI + `"`; !strings.Contains(got, want) {
					t.Errorf("missing %s:\n%s", want, got)
				}
			}
		})
	}
}

// TestUnclosedPhraseTargetIsNotATarget records a residual rather than
// asserting a shape this parser does not produce: an UNCLOSED quote is
// no target at all in docutils, which falls through to a comment, while
// this parser's unquoted rule still claims it and names it "`unclosed".
// What is pinned here is only that the quote-aware path does not fire --
// closing that gap belongs with the target-vs-comment fallthrough
// already noted for ".. _x:: y" (v0.90.0), and no corpus file on either
// side writes either shape.
func TestUnclosedPhraseTargetIsNotATarget(t *testing.T) {
	got := doctree.Dump(Parse(".. _`unclosed:\n\ntext\n"))
	if strings.Contains(got, `name="unclosed"`) {
		t.Errorf("an unclosed quote was treated as a closed phrase:\n%s", got)
	}
}
