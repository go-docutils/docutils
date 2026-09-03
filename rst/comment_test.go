package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestComment covers docutils.states.Body.comment (states.py, read
// directly): a bare ".." (nothing else on the line) is only an
// immediately-empty comment when the FOLLOWING line is itself blank or
// EOF — when something non-blank follows, its own indented body is
// gathered the SAME way a footnote/citation's body is
// (get_first_known_indented, no fixed indent floor), and stored as raw,
// verbatim text — NEVER re-parsed as reST, so a directive-looking,
// hyperlink-target-looking, citation-looking, or substitution-looking
// line inside a comment's body stays plain text. Also covers the
// "Explicit markup ends without a blank line; unexpected unindent."
// warning, the same shared shape footnotes/citations, line blocks,
// definition lists, and field lists already have.
func TestComment(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare '..' immediately followed by a blank line is an empty comment",
			"..\n\nParagraph.\n",
			"<document>\n    <comment>\n    <paragraph>\n        Paragraph.\n",
		},
		{
			"a bare '..' followed by an indented body on later lines gathers it as ONE comment, not a stray sibling",
			"..\n   A comment consisting of multiple lines\n   starting on the line after the\n   explicit markup start.\n",
			"<document>\n    <comment>\n        A comment consisting of multiple lines\n        starting on the line after the\n        explicit markup start.\n",
		},
		{
			"a comment body is raw text, never re-parsed as a directive even when it looks like one",
			"..\n   comment::\n\nThe extra newline before the comment text prevents\nthe parser from recognizing a directive.\n",
			"<document>\n    <comment>\n        comment::\n    <paragraph>\n        The extra newline before the comment text prevents\n        the parser from recognizing a directive.\n",
		},
		{
			"a comment body is raw text, never re-parsed as a hyperlink target",
			"..\n   _comment: http://example.org\n\nThe extra newline before the comment text prevents\nthe parser from recognizing a hyperlink target.\n",
			"<document>\n    <comment>\n        _comment: http://example.org\n    <paragraph>\n        The extra newline before the comment text prevents\n        the parser from recognizing a hyperlink target.\n",
		},
		{
			"a comment body is raw text, never re-parsed as a citation",
			"..\n   [comment] Not a citation.\n\nThe extra newline before the comment text prevents\nthe parser from recognizing a citation.\n",
			"<document>\n    <comment>\n        [comment] Not a citation.\n    <paragraph>\n        The extra newline before the comment text prevents\n        the parser from recognizing a citation.\n",
		},
		{
			"a comment body is raw text, never re-parsed as a substitution definition",
			"..\n   |comment| image:: bogus.png\n\nThe extra newline before the comment text prevents\nthe parser from recognizing a substitution definition.\n",
			"<document>\n    <comment>\n        |comment| image:: bogus.png\n    <paragraph>\n        The extra newline before the comment text prevents\n        the parser from recognizing a substitution definition.\n",
		},
		{
			"a comment interrupted by a non-blank line warns about the missing blank line",
			".. A comment\nno blank line\n\nParagraph.\n",
			"<document>\n    <comment>\n        A comment\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Explicit markup ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n    <paragraph>\n        Paragraph.\n",
		},
		{
			"two adjacent comments with no blank line between them are fine; only the FINAL abrupt end warns",
			".. A comment.\n.. Another.\nno blank line\n\nParagraph.\n",
			"<document>\n    <comment>\n        A comment.\n    <comment>\n        Another.\n    <system_message level=\"2\" line=\"3\" type=\"WARNING\">\n        <paragraph>\n            Explicit markup ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n    <paragraph>\n        Paragraph.\n",
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
