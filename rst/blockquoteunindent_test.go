package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestBlockQuoteUnindentWarning covers the last unported call site of
// docutils' RSTState.unindent_warning. That one method raises
// "<X> ends without a blank line; unexpected unindent." from NINE places
// — Block quote, Bullet list, Enumerated list, Field list, Option list,
// Explicit markup (twice), Definition list and Literal block — and this
// package already had every one of them except Block quote (and Option
// list, which no corpus fixture reaches).
//
// The line number is the OFFENDING line, not the block quote's own:
// unindent_warning reports "one line below the current line", its own
// comment's words. Checked against the reference.
func TestBlockQuoteUnindentWarning(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an unindented non-blank line right after a block quote warns",
			"Line 1.\nLine 2.\n\n   Indented.\nno blank line\n",
			"<document>\n    <paragraph>\n        Line 1.\n        Line 2.\n    <block_quote>\n        <paragraph>\n            Indented.\n    <system_message level=\"2\" line=\"5\" type=\"WARNING\">\n        <paragraph>\n            Block quote ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n",
		},
		{
			// The control: a blank line between them is the normal, correct
			// shape and must stay silent.
			"a blank line after a block quote is silent",
			"Line 1.\n\n   Indented.\n\nblank line first\n",
			"<document>\n    <paragraph>\n        Line 1.\n    <block_quote>\n        <paragraph>\n            Indented.\n    <paragraph>\n        blank line first\n",
		},
		{
			// The other control: a block quote ending at end-of-document has
			// no following line to be unindented, so nothing is reported.
			"a block quote at the end of the document is silent",
			"Line 1.\n\n   Indented.\n",
			"<document>\n    <paragraph>\n        Line 1.\n    <block_quote>\n        <paragraph>\n            Indented.\n",
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
