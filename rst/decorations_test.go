package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestHeaderFooterDirectives covers docutils.parsers.rst.directives.
// parts.Header/Footer (read directly): both nested-parse their content
// into the document's own SINGLETON <header>/<footer> — a second
// invocation of the same directive appends more content to the SAME
// element rather than creating a new one — wrapped in one <decoration>
// with header always first and footer always last, REGARDLESS of which
// was declared first in the source (get_header/get_footer's own fixed
// insertion points, nodes.py, read directly).
func TestHeaderFooterDirectives(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare header with same-line content",
			".. header:: a paragraph for the header\n",
			"<document>\n    <decoration>\n        <header>\n            <paragraph>\n                a paragraph for the header\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. header::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"header\" directive; none found.\n        <literal_block>\n            .. header::\n",
		},
		{
			"two separate header directives merge into ONE <header>, not two",
			".. header:: first part of the header\n.. header:: second part of the header\n",
			"<document>\n    <decoration>\n        <header>\n            <paragraph>\n                first part of the header\n            <paragraph>\n                second part of the header\n",
		},
		{
			"a bare footer with same-line content",
			".. footer:: a paragraph for the footer\n",
			"<document>\n    <decoration>\n        <footer>\n            <paragraph>\n                a paragraph for the footer\n",
		},
		{
			"header always comes before footer in <decoration>, regardless of declaration order",
			".. footer:: even if a footer is declared first\n.. header:: the header appears first\n",
			"<document>\n    <decoration>\n        <header>\n            <paragraph>\n                the header appears first\n        <footer>\n            <paragraph>\n                even if a footer is declared first\n",
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
