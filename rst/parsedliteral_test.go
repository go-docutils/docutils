package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestParsedLiteralDirective covers docutils.parsers.rst.directives.body.
// ParsedLiteral (read directly): ".. parsed-literal::" is a literal_block
// whose content IS inline-parsed, unlike an ordinary literal block —
// inline markup spanning multiple physical lines resolves as a single
// node, since the whole joined content is parsed in ONE inline_text call.
func TestParsedLiteralDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"inline markup spanning two physical lines resolves as one node",
			".. parsed-literal::\n\n   This is a parsed literal block.\n   It may contain *inline markup\n   spanning lines.*\n",
			"<document>\n    <literal_block>\n        This is a parsed literal block.\n        It may contain \n        <emphasis>\n            inline markup\n            spanning lines.\n",
		},
		{
			":class:/:name: options, 2-space indentation",
			".. parsed-literal::\n  :class: myliteral\n  :name: example: parsed\n\n   This is a parsed literal block with options.\n",
			"<document>\n    <literal_block class=\"myliteral\" id=\"example-parsed\" name=\"example: parsed\">\n         This is a parsed literal block with options.\n",
		},
		{
			"same-line content, no argument declared at all",
			".. parsed-literal:: content may start on same line\n",
			"<document>\n    <literal_block>\n        content may start on same line\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. parsed-literal::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"parsed-literal\" directive; none found.\n        <literal_block>\n            .. parsed-literal::\n",
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
