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
			"<document>\n    <substitution_definition name=\"sub\">\n        replacement text\n    <paragraph>\n        See \n        <reference anonymous=\"true\" refuri=\"https://example.org/anon\">\n            <substitution_reference refname=\"sub\">\n                sub\n         for more.\n    <target anonymous=\"true\" refuri=\"https://example.org/anon\">\n",
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
