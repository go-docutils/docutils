package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestArgumentBlockOfADirectiveWithNoOptions covers the two directives
// whose option_spec is None — class and title. docutils'
// parse_directive_block only looks for options when there IS an
// option_spec, and both declare final_argument_whitespace, so the
// ARGUMENT is the whole block up to the first blank line and a
// field-marker-shaped line inside it is part of the argument.
//
// sphinx's own extdev/appapi.rst writes
//
//	.. class:: Sphinx
//	   :no-index:
//
// which is the two classes "sphinx" and "no-index". This package read
// ":no-index:" as CONTENT and built a field list out of it; with a plain
// word there it produced a second paragraph beside the real one.
//
// The blank line is what separates the two readings, and the CONTROL is
// the case with one: there the body is content, which is what the
// existing fixture (".. class:: c1" + blank + indented text) already
// pinned and what a first attempt at this broke.
func TestArgumentBlockOfADirectiveWithNoOptions(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a field-marker-shaped second line is part of the argument",
			".. class:: Sphinx\n   :no-index:\n\n   para\n",
			`<paragraph class="sphinx no-index">`,
		},
		{
			"a plain second line is too",
			".. class:: a\n   b\n\n   para\n",
			`<paragraph class="a b">`,
		},
		{
			"the argument may start on the line below",
			".. class::\n   a b\n\n   para\n",
			`<paragraph class="a b">`,
		},
		{
			"with no content at all it is still one argument block",
			".. class:: Sphinx\n   :no-index:\n\npara\n",
			"class: ['sphinx', 'no-index']",
		},
		{
			// CONTROL: a blank line under the directive means the body is
			// CONTENT, not more argument.
			"a blank line separates the argument from the content",
			"x\n\n.. class:: c1\n\n   See the text.\n",
			`<paragraph class="c1">`,
		},
		{
			// CONTROL, same shape, and the one a first attempt broke: the
			// content's own words must not become class names.
			"content words are not class names",
			"x\n\n.. class:: c1\n\n   See the text.\n",
			"See the text.",
		},
		{
			"the title directive keeps its whole argument block",
			".. title:: My Title\n   :x:\n",
			"<document title=\"My Title\n:x:\">",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%s", tc.source, got, tc.want)
			}
			if strings.Contains(got, "field_list") {
				t.Errorf("Parse(%q) read the argument block as a field list:\n%s", tc.source, got)
			}
		})
	}
}
