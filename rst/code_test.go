package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCodeDirective covers docutils.parsers.rst.directives.body.CodeBlock
// (read directly): ".. code:: [language]" wraps its content in a
// <literal_block class="code ...">. Real docutils would additionally
// run the content through Pygments for syntax coloring when the
// document's syntax_highlight setting isn't "none" — not ported here
// (see code.go's own doc comment) — every case below matches the
// corpus's own default-settings output, which shows no highlighting
// either. The ":number-lines:" option IS fully ported, independent of
// lexical analysis.
func TestCodeDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare code block with no language",
			".. code::\n\n   This is a code block.\n",
			"<document>\n    <literal_block class=\"code\">\n        This is a code block.\n",
		},
		{
			"generic class/name options, indented by only 2 columns",
			".. code::\n  :class: testclass\n  :name: without argument\n\n  This is a code block with generic options.\n",
			"<document>\n    <literal_block class=\"code testclass\" id=\"without-argument\" name=\"without argument\">\n        This is a code block with generic options.\n",
		},
		{
			"a language argument becomes a class even though it's never highlighted",
			".. code:: text\n  :class: testclass\n\n  This is a code block with text.\n",
			"<document>\n    <literal_block class=\"code text testclass\">\n        This is a code block with text.\n",
		},
		{
			"a bare :number-lines: flag defaults to starting at 1",
			".. code::\n  :number-lines:\n\n  This is a code block with text.\n",
			"<document>\n    <literal_block class=\"code\">\n        <inline class=\"ln\">\n            1 \n        This is a code block with text.\n",
		},
		{
			"a :number-lines: value sets the starting line number",
			".. code::\n  :number-lines: 30\n\n  This is a code block with text.\n",
			"<document>\n    <literal_block class=\"code\">\n        <inline class=\"ln\">\n            30 \n        This is a code block with text.\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. code::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"code\" directive; none found.\n        <literal_block>\n            .. code::\n",
		},
		{
			"multi-line numbered code pads line numbers to the width of the LAST line number",
			".. code:: python\n  :number-lines: 7\n\n  def my_function():\n      '''Test the lexer.\n      '''\n\n      # and now for something completely different\n      print(8/2)\n",
			"<document>\n    <literal_block class=\"code python\">\n        <inline class=\"ln\">\n             7 \n        def my_function():\n        <inline class=\"ln\">\n             8 \n            '''Test the lexer.\n        <inline class=\"ln\">\n             9 \n            '''\n        <inline class=\"ln\">\n            10 \n        \n        <inline class=\"ln\">\n            11 \n            # and now for something completely different\n        <inline class=\"ln\">\n            12 \n            print(8/2)\n",
		},
		{
			"an unhighlighted language keeps special characters completely literal",
			".. code:: latex\n\n  hello \\emph{world} % emphasize\n",
			"<document>\n    <literal_block class=\"code latex\">\n        hello \\emph{world} % emphasize\n",
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
