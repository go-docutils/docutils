package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestParagraphContinuationIgnoresShape pins the rule that a continuation
// line's SHAPE is irrelevant. docutils' Text state has exactly four
// transitions — blank, indent, underline, text — and no bullet, enum,
// field, explicit-markup, doctest, line-block or table transition at all,
// so a line that merely looks like one of those, with no blank line
// before it, is still part of the same paragraph.
//
// consumeParagraph used to break on every one of those shapes, which is
// why each case here was a real defect: the paragraph was split and a
// spurious list/comment/field/table started mid-sentence. Every expected
// value below was checked against the reference implementation.
func TestParagraphContinuationIgnoresShape(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bullet-shaped continuation line does not start a list",
			"text\n- item\n",
			"<document>\n    <paragraph>\n        text\n        - item\n",
		},
		{
			"an enumerator-shaped continuation line does not start a list",
			"text\n1. one\n",
			"<document>\n    <paragraph>\n        text\n        1. one\n",
		},
		{
			"an explicit-markup-shaped continuation line does not start a comment",
			"text\n.. comment\n",
			"<document>\n    <paragraph>\n        text\n        .. comment\n",
		},
		{
			"a field-marker-shaped continuation line does not start a field list",
			"text\n:field: v\n",
			"<document>\n    <paragraph>\n        text\n        :field: v\n",
		},
		{
			"a line-block-shaped continuation line does not start a line block",
			"text\n| line\n",
			"<document>\n    <paragraph>\n        text\n        | line\n",
		},
		{
			// The CONTROL: with a blank line between them, each really is
			// its own construct. That boundary is what the blank-line check
			// above already handles, and is why the shape checks were only
			// ever reachable in the wrong situation.
			"a blank line between them really does start the list",
			"text\n\n- item\n",
			"<document>\n    <paragraph>\n        text\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item\n",
		},
		{
			// The other control: an underline IS one of the Text state's
			// four transitions, so a long uniform run still splits.
			"a title underline still splits, since underline IS a Text transition",
			"text\n====\n",
			"<document>\n    <section id=\"text\" name=\"text\">\n        <title>\n            text\n",
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
