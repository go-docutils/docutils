package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCompoundDirective covers docutils.parsers.rst.directives.body.Compound
// (read directly): ".. compound::" wraps its content in a <compound>,
// reusing runAdmonitionOrGeneric directly (no arguments, :class:/:name:
// options, content REQUIRED) — structurally identical to the nine
// generic admonitions, just a different tag. The third case is the one
// that caught a real bug in parseDirectiveBlock shared by every directive
// that folds a same-line argument back into content (see that function's
// own doc comment): the blank line separating the same-line text from a
// later paragraph was being dropped during the fold, silently merging
// two paragraphs into one.
func TestCompoundDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"content with an embedded bullet list, split into multiple paragraphs",
			".. compound::\n\n   Compound paragraphs are single logical paragraphs\n   which contain embedded\n\n   * lists\n   * tables\n   * literal blocks\n   * and other body elements\n\n   and are split into multiple physical paragraphs.\n",
			"<document>\n    <compound>\n        <paragraph>\n            Compound paragraphs are single logical paragraphs\n            which contain embedded\n        <bullet_list bullet=\"*\">\n            <list_item>\n                <paragraph>\n                    lists\n            <list_item>\n                <paragraph>\n                    tables\n            <list_item>\n                <paragraph>\n                    literal blocks\n            <list_item>\n                <paragraph>\n                    and other body elements\n        <paragraph>\n            and are split into multiple physical paragraphs.\n",
		},
		{
			"class/name options plus an embedded literal block",
			".. compound::\n   :name: interesting\n   :class: log\n\n   This is an extremely interesting compound paragraph containing a\n   simple paragraph, a literal block with some useless log messages::\n\n       Connecting... OK\n       Transmitting data... OK\n       Disconnecting... OK\n\n   and another simple paragraph which is actually just a continuation\n   of the first simple paragraph, with the literal block in between.\n",
			"<document>\n    <compound class=\"log\" id=\"interesting\" name=\"interesting\">\n        <paragraph>\n            This is an extremely interesting compound paragraph containing a\n            simple paragraph, a literal block with some useless log messages:\n        <literal_block>\n            Connecting... OK\n            Transmitting data... OK\n            Disconnecting... OK\n        <paragraph>\n            and another simple paragraph which is actually just a continuation\n            of the first simple paragraph, with the literal block in between.\n",
		},
		{
			"same-line content stays a separate paragraph from what follows the blank line",
			".. compound:: content may start on same line\n\n   second paragraph\n",
			"<document>\n    <compound>\n        <paragraph>\n            content may start on same line\n        <paragraph>\n            second paragraph\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. compound::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"compound\" directive; none found.\n        <literal_block>\n            .. compound::\n",
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
