package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestAdmonitions covers docutils.parsers.rst.directives.admonitions
// (read directly): the nine generic admonitions (case-insensitive
// directive name, no argument at all — same-line text is the
// directive's own first content line, not an argument), :class:/:name:
// options, content appearing BEFORE an option line (real docutils'
// Body.parse_directive_block/parse_directive_options algorithm, which
// scans an arg block top-to-bottom for the FIRST field-marker-shaped
// line rather than assuming options are always at the very start), the
// "content required" diagnostic, and the generic ".. admonition::"
// directive (a REQUIRED title argument, auto-class "admonition-<slug>"
// unless overridden, and its own "argument required" diagnostic). Every
// case verified against the foreign judge (Parser().parse(), the same
// bare, pre-transform tree doctree.Dump produces).
func TestAdmonitions(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"generic admonitions are recognized case-insensitively; :name:/:class: options",
			".. Attention:: Directives at large.\n\n.. Note:: :name: mynote\n   :class: testnote\n\n   Admonitions support the generic \"name\" and \"class\" options.\n",
			"<document>\n    <attention>\n        <paragraph>\n            Directives at large.\n    <note class=\"testnote\" id=\"mynote\" name=\"mynote\">\n        <paragraph>\n            Admonitions support the generic \"name\" and \"class\" options.\n",
		},
		{
			"a one-line note: same-line text is content, not an argument",
			".. note:: One-line notes.\n",
			"<document>\n    <note>\n        <paragraph>\n            One-line notes.\n",
		},
		{
			// The KEY case: real docutils' own option-scanning finds the
			// FIRST field-marker-shaped line WHEREVER it starts, not just
			// at the very top of the block — content before it stays
			// content. A role invocation that merely LOOKS like it could
			// be near an option line, or starts a line on its own, is
			// still NOT mistaken for one (matchFieldMarker's own
			// backtick-refusal, v0.22.0).
			"content before an option line is still content, not swallowed as an argument",
			".. note:: Content before options\n   is possible too.\n   :class: mynote\n\n.. note:: :strong:`a role is not an option`.\n   :name: role not option\n",
			"<document>\n    <note class=\"mynote\">\n        <paragraph>\n            Content before options\n            is possible too.\n    <note id=\"role-not-option\" name=\"role not option\">\n        <paragraph>\n            <strong>\n                a role is not an option\n            .\n",
		},
		{
			"a generic admonition with no content at all: ERROR",
			".. note::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"note\" directive; none found.\n        <literal_block>\n            .. note::\n",
		},
		{
			"the generic admonition directive: required title argument becomes <title>, default class from the title's own slug",
			".. admonition:: Admonition\n\n   This is a generic admonition.\n",
			"<document>\n    <admonition class=\"admonition-admonition\">\n        <title>\n            Admonition\n        <paragraph>\n            This is a generic admonition.\n",
		},
		{
			"the generic admonition directive with no title argument at all: ERROR",
			".. admonition::\n\n   Generic admonitions require a title.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"admonition\" directive:\n            1 argument(s) required, 0 supplied.\n        <literal_block>\n            .. admonition::\n            \n               Generic admonitions require a title.\n",
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
