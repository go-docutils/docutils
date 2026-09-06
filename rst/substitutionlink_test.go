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
