package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestFieldBodySeparatorIsNotIndentation covers the space after a field
// marker. ":Description:  value" -- two spaces, which is how a PEP aligns
// a column of field values -- made the value an INDENTED block here, so
// it came out as a <block_quote>, and a second line after it drew a
// spurious "Block quote ends without a blank line" warning that split one
// field body into three nodes.
//
// docutils reaches the plain paragraph by another road:
// get_first_known_indented cuts the first line at the marker and then
// trims the block by the minimum indent of the lines AFTER it, so
// whatever the first line has left over never sets an indent at all.
// Every expectation here is the reference's own output.
func TestFieldBodySeparatorIsNotIndentation(t *testing.T) {
	const body = "<field_body>\n                <paragraph>\n                    text here"
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"one space", ":Description: text here\n", body},
		{"two spaces", ":Description:  text here\n", body},
		{"five spaces", ":Description:     text here\n", body},
		{
			"two spaces then a shallower continuation",
			":Description:  text here\n   and more\n",
			body + "\n                    and more",
		},
		{
			// The continuation is DEEPER than the marker and still joins
			// the same paragraph -- the first line's leftover spaces set
			// no baseline for it to be measured against.
			"two spaces then a deeper continuation",
			":Description:  text here\n      deeper\n",
			body + "\n                    deeper",
		},
		{
			// CONTROL: a body that starts on the NEXT line is a real
			// indented block, and its own indent is the one that gets
			// trimmed. This case never went through a block quote and
			// must not start.
			"an empty marker line with the body below it",
			":Description:\n   text here\n", body,
		},
		{"an empty marker line with a deeper body", ":Description:\n      text here\n", body},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%s", tc.source, got, tc.want)
			}
			if strings.Contains(got, "block_quote") {
				t.Errorf("Parse(%q) made the field value a block quote:\n%s", tc.source, got)
			}
		})
	}
}

// TestEmbeddedLinkWithNoText covers "`<target>`_" -- a phrase reference
// whose text is omitted entirely, so the TARGET supplies it. docutils is
// one line, "if not text: text = alias", and alias is the value it has
// already computed: normalize_name for a name alias, adjust_uri for a URI.
//
// This package re-derived it instead of reusing it, and got both branches
// wrong in different ways -- which is what re-deriving a value costs.
func TestEmbeddedLinkWithNoText(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// Two PEPs write this. The displayed text is the NORMALIZED
			// name, not the spelling inside the angle brackets.
			"a name alias displays its normalized name",
			"See `<Feature Negotiation_>`__.\n",
			`<reference name="feature negotiation" refname="feature negotiation">` + "\n            feature negotiation",
		},
		{
			// And a URI alias displays the URI it LINKS to, mailto: and
			// all -- adjust_uri runs before the text is taken from it.
			"an email alias displays the adjusted URI",
			"See `<user@e.org>`__.\n",
			`<reference name="mailto:user@e.org" refuri="mailto:user@e.org">` + "\n            mailto:user@e.org",
		},
		{
			"a plain URI alias is unchanged",
			"See `<https://e.org/X>`__.\n",
			`<reference name="https://e.org/X" refuri="https://e.org/X">`,
		},
		{
			// CONTROL: with text of its own, the text wins and the alias
			// is only the target. This is the common form and must not
			// move.
			"text in front of the alias is untouched",
			"See `text <Feature Negotiation_>`__.\n",
			`<reference name="text" refname="feature negotiation">` + "\n            text",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
