package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestSubstitutionReferenceAsHyperlink covers "|name|_" / "|name|__" —
// docutils wraps the substitution_reference in a <reference> pointing at a
// target with the same name (or, doubled, an anonymous one), verified
// against real docutils: the wrapping reference itself carries no "name"
// attribute (the substitution's own content is already the display text,
// unlike "text"_ which needs to remember its display separately from its
// target).
func TestSubstitutionReferenceAsHyperlink(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"named target resolves through the substitution's own name",
			".. |sub| replace:: replacement text\n\n.. _sub: https://example.org/sub\n\nSee |sub|_ for more.\n",
			"<document>\n    <substitution_definition name=\"sub\">\n        replacement text\n    <target id=\"sub\" name=\"sub\" refuri=\"https://example.org/sub\">\n    <paragraph>\n        See \n        <reference refname=\"sub\" refuri=\"https://example.org/sub\">\n            <substitution_reference refname=\"sub\">\n                sub\n         for more.\n",
		},
		{
			"doubled trailing underscore is an anonymous target by document-order position",
			".. |sub| replace:: replacement text\n\nSee |sub|__ for more.\n\n.. __: https://example.org/anon\n",
			"<document>\n    <substitution_definition name=\"sub\">\n        replacement text\n    <paragraph>\n        See \n        <reference anonymous=\"1\" refuri=\"https://example.org/anon\">\n            <substitution_reference refname=\"sub\">\n                sub\n         for more.\n    <target anonymous=\"1\" refuri=\"https://example.org/anon\">\n",
		},
		{
			"a plain substitution reference with no trailing underscore is unaffected",
			".. |sub| replace:: x\n\nplain |sub| no link.\n",
			"<document>\n    <substitution_definition name=\"sub\">\n        x\n    <paragraph>\n        plain \n        <substitution_reference refname=\"sub\">\n            sub\n         no link.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestSubstitutionUnindentWarning covers the "Explicit markup ends
// without a blank line; unexpected unindent." warning for a SUBSTITUTION
// DEFINITION. Every other explicit construct -- footnote, citation,
// comment, directive, topic -- already appended it; the substitution path
// simply discarded gatherExplicitBody's blankFinish, so it was the one
// member of the family that stayed silent.
func TestSubstitutionUnindentWarning(t *testing.T) {
	got := doctree.Dump(Parse(".. |symbol| image:: symbol.png\nNo blank line after.\n"))
	if !strings.Contains(got, "Explicit markup ends without a blank line; unexpected unindent.") {
		t.Errorf("no unindent warning for a substitution definition:\n%s", got)
	}
	// The control: a blank line after it stays silent.
	quiet := doctree.Dump(Parse(".. |symbol| image:: symbol.png\n\nA paragraph.\n"))
	if strings.Contains(quiet, "unexpected unindent") {
		t.Errorf("warned despite a blank line:\n%s", quiet)
	}
}

// TestPipeNameRejectsEdgeWhitespace pins the fallthrough: a name with
// whitespace immediately inside either pipe is not a substitution
// definition at all, and the line becomes an ordinary comment.
func TestPipeNameRejectsEdgeWhitespace(t *testing.T) {
	got := doctree.Dump(Parse(".. | bad name | bad data\n"))
	if !strings.Contains(got, "<comment>") || strings.Contains(got, "substitution_definition") {
		t.Errorf("expected a comment, not a substitution:\n%s", got)
	}
	ok := doctree.Dump(Parse(".. |good name| replace:: x\n"))
	if !strings.Contains(ok, "<substitution_definition") {
		t.Errorf("a name with an INTERNAL space was rejected:\n%s", ok)
	}
}

// TestFailedImageInSubstitution covers the message PAIR a failed embedded
// directive produces: the directive's own ERROR, then the "empty or
// invalid" WARNING for the substitution it was standing in for. They
// quote DIFFERENT blocks -- the ERROR quotes the DIRECTIVE, the WARNING
// the whole substitution line -- which is the same split the replace
// directive's content checks use.
//
// Only the first of the two used to be emitted, and it quoted the
// substitution line.
//
// KNOWN RESIDUAL: inside the ERROR's literal block, content indented
// relative to the directive comes out dedented, because
// gatherExplicitBody strips the body's own minimum indent before the
// block is rebuilt. docutils keeps it. One corpus fixture differs on
// exactly that and nothing else; recovering the original indentation
// means threading an undedented body through, which is a larger change
// than a quoted string's leading spaces justify.
func TestFailedImageInSubstitution(t *testing.T) {
	got := doctree.Dump(Parse(".. |symbol 1| image:: symbol.png\n\n    Followed by a block quote.\n"))
	if !strings.Contains(got, `Error in "image" directive:`) {
		t.Errorf("the directive's own error is missing:\n%s", got)
	}
	if !strings.Contains(got, `Substitution definition "symbol 1" empty or invalid.`) {
		t.Errorf("the substitution warning is missing:\n%s", got)
	}
	// The ERROR quotes the DIRECTIVE, not the ".. |name|" line.
	i := strings.Index(got, `Error in "image" directive:`)
	j := strings.Index(got, `Substitution definition "symbol 1"`)
	if i < 0 || j < 0 || i > j {
		t.Fatalf("expected the ERROR before the WARNING:\n%s", got)
	}
	if strings.Contains(got[i:j], ".. |symbol 1|") {
		t.Errorf("the ERROR quoted the substitution line rather than the directive:\n%s", got[i:j])
	}
}
