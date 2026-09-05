package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestNestedTitlesAreErrors covers docutils' match_titles=False handling:
// inside a block quote, a list item, a table cell — anywhere that is not
// the document or a section — neither a section title nor a transition is
// allowed, and Body.line / Text.underline turn what looks like one into an
// ERROR carrying the offending source as a <literal_block>.
//
// Every case here was run against the reference implementation, including
// the three that are NOT about producing an error at all: the mid-paragraph
// adornment (folded into the paragraph, no diagnostic), the too-short one
// (an INFO, then treated as ordinary text) and the single character (no
// message whatsoever). The first of those three is why the error branches
// are guarded on being at the START of a block: without that guard this
// invented an ERROR for an ordinary wrapped paragraph.
func TestNestedTitlesAreErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an underlined title inside a block quote is an ERROR, both lines consumed",
			"x\n\n    Title\n    =====\n    Paragraph.\n",
			"<document>\n    <paragraph>\n        x\n    <block_quote>\n        <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n            <paragraph>\n                Unexpected section title.\n            <literal_block>\n                Title\n                =====\n        <paragraph>\n            Paragraph.\n",
		},
		{
			"a lone adornment inside a block quote is an ERROR, not a transition",
			"x\n\n    Block quote.\n\n    --------\n\n    Paragraph.\n",
			"<document>\n    <paragraph>\n        x\n    <block_quote>\n        <paragraph>\n            Block quote.\n        <system_message level=\"3\" line=\"5\" type=\"ERROR\">\n            <paragraph>\n                Unexpected section title or transition.\n            <literal_block>\n                --------\n        <paragraph>\n            Paragraph.\n",
		},
		{
			// The guard's own case: an adornment-looking line in the MIDDLE
			// of a paragraph is not a title attempt at all.
			"an adornment mid-paragraph draws no diagnostic",
			"x\n\n    Line one\n    Line two\n    ========\n",
			"<document>\n    <paragraph>\n        x\n    <block_quote>\n        <paragraph>\n            Line one\n            Line two\n        <paragraph>\n            ========\n",
		},
		{
			// Too short to be an overline: docutils says so, then treats it
			// as ordinary text. Any length below 4 draws it.
			"a too-short adornment gets an INFO and stays text",
			"x\n\n    text\n\n    ---\n\n    more\n",
			"<document>\n    <paragraph>\n        x\n    <block_quote>\n        <paragraph>\n            text\n        <system_message level=\"1\" line=\"5\" type=\"INFO\">\n            <paragraph>\n                Unexpected possible title overline or transition.\n                Treating it as ordinary text because it's so short.\n        <paragraph>\n            ---\n        <paragraph>\n            more\n",
		},
		{
			// A SINGLE character draws it too. An earlier version of this
			// test used "-" and concluded there was a floor of 2 -- but
			// "-" is a BULLET, so that probe was measuring list
			// recognition, not adornment length. "~" is not.
			"even a single-character adornment gets the INFO",
			"x\n\n    text\n\n    ~\n\n    more\n",
			"<document>\n    <paragraph>\n        x\n    <block_quote>\n        <paragraph>\n            text\n        <system_message level=\"1\" line=\"5\" type=\"INFO\">\n            <paragraph>\n                Unexpected possible title overline or transition.\n                Treating it as ordinary text because it's so short.\n        <paragraph>\n            ~\n        <paragraph>\n            more\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
