package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestLineBlockDirective covers the legacy ".. line-block::" directive
// (docutils.parsers.rst.directives.body.LineBlock, read directly) —
// distinct from the bare "| " syntax parseLineBlock already handles,
// though it reuses that function's own nestLineBlockSegment verbatim.
// UNLIKE the bare syntax, each content line is inline-parsed
// INDEPENDENTLY, never joined across a wrap: an emphasis start-string
// on one line and its end-string on the next stay two separate,
// individually-parsed lines, not one span (LineBlock.run's own per-line
// inline_text call, read directly).
func TestLineBlockDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare line block, indentation beyond the shallowest nests",
			".. line-block::\n\n   This is a line block.\n   Newlines are *preserved*.\n       As is initial whitespace.\n",
			"<document>\n    <line_block>\n        <line>\n            This is a line block.\n        <line>\n            Newlines are \n            <emphasis>\n                preserved\n            .\n        <line_block>\n            <line>\n                As is initial whitespace.\n",
		},
		{
			":class:/:name: options",
			".. line-block::\n   :class: linear\n   :name:  cit:short\n\n   This is a line block with options.\n",
			"<document>\n    <line_block class=\"linear\" id=\"cit-short\" name=\"cit:short\">\n        <line>\n            This is a line block with options.\n",
		},
		{
			"inline markup may NOT span across two content lines, unlike the bare \"| \" syntax",
			".. line-block::\n\n   Inline markup *may not span\n       multiple lines* of a line block.\n",
			"<document>\n    <line_block>\n        <line>\n            Inline markup \n            <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                *\n            may not span\n        <line_block>\n            <line>\n                multiple lines* of a line block.\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"3\" type=\"WARNING\">\n        <paragraph>\n            Inline emphasis start-string without end-string.\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. line-block::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"line-block\" directive; none found.\n        <literal_block>\n            .. line-block::\n",
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
