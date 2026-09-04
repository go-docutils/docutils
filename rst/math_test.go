package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestMathDirective covers docutils.parsers.rst.directives.body.MathBlock
// (read directly): ".. math::" wraps TeX math source in <math_block>
// nodes, content kept VERBATIM (never inline-parsed — it's TeX, so a
// backslash or asterisk in it is math syntax, not markup). The
// distinctive part is that content SPLITS ON BLANK LINES into several
// sibling blocks from one directive, and that no arguments are declared
// at all, so same-line text folds into content's own first line exactly
// like a generic admonition's does — which is why an argument PLUS an
// indented body produces two blocks rather than one.
func TestMathDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"same-line text becomes the block's own content, not an argument",
			".. math:: y = f(x)\n",
			"<document>\n    <math_block>\n        y = f(x)\n",
		},
		{
			"an indented body works the same way",
			".. math::\n\n  1+1=2\n",
			"<document>\n    <math_block>\n        1+1=2\n",
		},
		{
			":class:/:name: options, the name lowercased and slugified for its id",
			".. math::\n  :class: new\n  :name: eq:Eulers law\n\n  e^i*2*\\pi = 1\n",
			"<document>\n    <math_block class=\"new\" id=\"eq-eulers-law\" name=\"eq:eulers law\">\n        e^i*2*\\pi = 1\n",
		},
		{
			"same-line text AND an indented body are two blocks, not an argument plus content",
			".. math:: y = f(x)\n\n  1+1=2\n\n",
			"<document>\n    <math_block>\n        y = f(x)\n    <math_block>\n        1+1=2\n",
		},
		{
			"a blank line inside the content splits it into sibling blocks",
			".. math::\n\n  1+1=2\n\n  E = mc^2\n",
			"<document>\n    <math_block>\n        1+1=2\n    <math_block>\n        E = mc^2\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. math::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"math\" directive; none found.\n        <literal_block>\n            .. math::\n",
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
