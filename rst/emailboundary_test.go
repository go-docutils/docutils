package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestEmailEndBoundary pins the end_string_suffix class a standalone
// email address may stop on. It is the SAME class the standalone-URI
// scan uses, which is the whole point: the URI path was corrected in
// v0.63.0 because ">" is a MATH SYMBOL in Unicode rather than
// punctuation, and the email path kept an ad-hoc IsSpace||IsPunct test
// sixteen lines below the comment explaining why that is wrong.
//
// The consequence was invisible in the docutils testsuite corpus and
// enormous in the real-world one: "<user@host>" is how essentially every
// PEP writes an author, so 386 of 1564 real files differed on this one
// character class.
//
// Every case below was run against the reference.
func TestEmailEndBoundary(t *testing.T) {
	cases := []struct {
		name, source string
		wantRefURI   string
	}{
		{"angle brackets, the PEP author shape", "Mail <jane@example.com> now.\n", "mailto:jane@example.com"},
		{"parentheses", "Mail (jane@example.com) now.\n", "mailto:jane@example.com"},
		{"bare, with a trailing period", "Mail jane@example.com now.\n", "mailto:jane@example.com"},
		{"angle brackets then a comma", "See <a@b.c>, ok.\n", "mailto:a@b.c"},
		// The control from the sibling construct: a URI in angle brackets
		// already worked, and that asymmetry is what identified the bug.
		{"a URI in angle brackets, unchanged", "Mail <https://example.com> now.\n", "https://example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if want := `refuri="` + tc.wantRefURI + `"`; !strings.Contains(got, want) {
				t.Errorf("missing %s:\n%s", want, got)
			}
		})
	}
	// And the guard that keeps this from being "anything with an @":
	// docutils' host part needs at least two characters.
	if got := doctree.Dump(Parse("Mail a@b now.\n")); strings.Contains(got, "mailto:") {
		t.Errorf("a one-character host was accepted as an address:\n%s", got)
	}
}
