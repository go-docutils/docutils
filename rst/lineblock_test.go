package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestLineBlockContinuationLines covers docutils.states.RSTState.
// line_block_line's own get_first_known_indented(match.end(),
// until_blank=True) (states.py, read directly) — an indented line
// following a "|"-marked one, with no "|" of its own, joins the SAME
// <line> rather than becoming an unrelated block_quote sibling and
// splitting the line block in two; inline markup can span the wrap
// since the joined text is parsed as one multi-line unit. Also covers
// the "Line block ends without a blank line." diagnostic (real
// docutils' own Body.line_block, same file) — verified against
// test_line_blocks.py's own corpus fixtures.
func TestLineBlockContinuationLines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a continuation line joins the same <line>, not a separate block_quote",
			"| Individual lines in line blocks\n  *may* wrap, as indicated by the lack of a vertical bar prefix.\n| These are called \"continuation lines\".\n",
			"<document>\n    <line_block>\n        <line>\n            Individual lines in line blocks\n            <emphasis>\n                may\n             wrap, as indicated by the lack of a vertical bar prefix.\n        <line>\n            These are called \"continuation lines\".\n",
		},
		{
			"inline markup spans the continuation-line wrap, but not past a NEW marked line",
			"| Inline markup in line blocks may also wrap *to\n  continuation lines*.\n| But not to following lines.\n",
			"<document>\n    <line_block>\n        <line>\n            Inline markup in line blocks may also wrap \n            <emphasis>\n                to\n                continuation lines\n            .\n        <line>\n            But not to following lines.\n",
		},
		{
			"a line block ending on a non-blank, non-marked line warns about the missing blank line",
			"| This line block is incomplete.\nThere should be a blank line before this paragraph.\n",
			"<document>\n    <line_block>\n        <line>\n            This line block is incomplete.\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Line block ends without a blank line.\n    <paragraph>\n        There should be a blank line before this paragraph.\n",
		},
		{
			"a continuation line's own indent can be LESS than the marked line's, still joining the same <line>",
			"| Continuation lines may be indented less\n  than their base lines.\n",
			"<document>\n    <line_block>\n        <line>\n            Continuation lines may be indented less\n            than their base lines.\n",
		},
		{
			"a diagnostic inside a line block line carries a real line number",
			"| Inline markup *may not\n| wrap* over several lines.\n",
			"<document>\n    <line_block>\n        <line>\n            Inline markup \n            <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                *\n            may not\n        <line>\n            wrap* over several lines.\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline emphasis start-string without end-string.\n",
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
