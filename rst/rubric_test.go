package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestRubricDirective covers docutils.parsers.rst.directives.body.Rubric
// (read directly): ".. rubric:: TEXT" produces a <rubric> whose children
// are TEXT's own inline-parsed content — no wrapping <title>, unlike
// topic/admonition. Rubric declares no has_content at all, so a real
// indented body is a genuine "no content permitted" ERROR, not silently
// ignored.
func TestRubricDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare rubric with plain text",
			".. rubric:: This is a rubric\n",
			"<document>\n    <rubric>\n        This is a rubric\n",
		},
		{
			"a missing argument, and a present argument with disallowed content, are two SEPARATE errors",
			".. rubric::\n.. rubric:: A rubric has no content\n\n   Invalid content\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"rubric\" directive:\n            1 argument(s) required, 0 supplied.\n        <literal_block>\n            .. rubric::\n    <system_message level=\"3\" line=\"2\" type=\"ERROR\">\n        <paragraph>\n            Error in \"rubric\" directive:\n            no content permitted.\n        <literal_block>\n            .. rubric:: A rubric has no content\n            \n               Invalid content\n",
		},
		{
			"a rubric followed by a sibling block quote (not swallowed as its own content)",
			".. rubric:: A rubric followed by a block quote\n..\n\n   Block quote\n",
			"<document>\n    <rubric>\n        A rubric followed by a block quote\n    <comment>\n    <block_quote>\n        <paragraph>\n            Block quote\n",
		},
		{
			":class:/:name: options",
			".. rubric:: A Rubric\n   :class: foo bar\n   :name: Foo Rubric\n",
			"<document>\n    <rubric class=\"foo bar\" id=\"foo-rubric\" name=\"foo rubric\">\n        A Rubric\n",
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
