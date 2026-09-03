package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestFootnoteAndCitationBodies covers docutils.states.Body.footnote/
// Body.citation and their shared body-gathering (get_first_known_indented)
// and explicit-markup-chain diagnostics (Body.explicit_markup/
// explicit_list), read directly and verified against the foreign judge
// (test_footnotes.py[footnotes]) — a manually-numbered footnote or a
// citation gets a real "id" attribute (falling back to a positional
// "footnote-N"/"citation-N" when the name itself can't become a valid
// id, as any purely-numeric footnote label can't — see explicitTargetID),
// its continuation lines dedent by their own TRUE minimum indentation
// rather than a fixed column (so a line indented less than the marker's
// own width still joins the same body), an empty body warns instead of
// silently producing nothing, and a body that ends on a non-blank
// unindented line — one that ISN'T itself another explicit-markup
// construct — warns about the missing blank line.
func TestFootnoteAndCitationBodies(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a single-line footnote gets a positional id, since its numeric name can't become one",
			".. [1] This is a footnote.\n",
			"<document>\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            This is a footnote.\n",
		},
		{
			"a continuation line indented LESS than the marker's own width still joins the same paragraph",
			".. [1] This is a footnote\n     on multiple lines with more space.\n\n.. [2] This is a footnote\n  on multiple lines with less space.\n",
			"<document>\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            This is a footnote\n            on multiple lines with more space.\n    <footnote id=\"footnote-2\" name=\"2\">\n        <label>\n            2\n        <paragraph>\n            This is a footnote\n            on multiple lines with less space.\n",
		},
		{
			"a footnote whose body starts on the line after a bare marker",
			".. [1]\n   This is a footnote on multiple lines\n   whose block starts on line 2.\n",
			"<document>\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            This is a footnote on multiple lines\n            whose block starts on line 2.\n",
		},
		{
			"an empty footnote warns instead of silently producing an empty body",
			"An empty footnote raises a warning:\n\n.. [1]\n\n",
			"<document>\n    <paragraph>\n        An empty footnote raises a warning:\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <system_message level=\"2\" line=\"4\" type=\"WARNING\">\n            <paragraph>\n                Footnote content expected.\n",
		},
		{
			"a footnote body ending on a non-blank, non-explicit-markup line warns about the missing blank line",
			".. [1] spamalot\nNo blank line.\n",
			"<document>\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            spamalot\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Explicit markup ends without a blank line; unexpected unindent.\n    <paragraph>\n        No blank line.\n",
		},
		{
			"two adjacent footnotes with no blank line between them are NOT an abrupt unindent — the chain just continues",
			".. [1] One.\n.. [2] Two.\n",
			"<document>\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            One.\n    <footnote id=\"footnote-2\" name=\"2\">\n        <label>\n            2\n        <paragraph>\n            Two.\n",
		},
		{
			"a citation's own (non-numeric) name becomes its id directly, no positional fallback needed",
			".. [note] A citation.\n",
			"<document>\n    <citation id=\"note\" name=\"note\">\n        <label>\n            note\n        <paragraph>\n            A citation.\n",
		},
		{
			"an empty citation gets its own message text, not the footnote one",
			"An empty citation raises a warning:\n\n.. [note]\n\n",
			"<document>\n    <paragraph>\n        An empty citation raises a warning:\n    <citation id=\"note\" name=\"note\">\n        <label>\n            note\n        <system_message level=\"2\" line=\"4\" type=\"WARNING\">\n            <paragraph>\n                Citation content expected.\n",
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
