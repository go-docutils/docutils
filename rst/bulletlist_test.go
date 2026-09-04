package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestBulletListDiagnostics covers two real gaps in parseBulletList,
// docutils.parsers.rst.states.Body.bullet's own unindent_warning (read
// directly): the "ends without a blank line" WARNING every other list/
// body construct in this package already has (field lists, definition
// lists, line blocks, footnotes/citations, comments, topics/sidebars),
// but bullet lists never did — and a real byte-vs-rune indexing bug in
// bulletContentColumn: a Unicode bullet character ("•"/"‣"/"⁃", all
// multi-byte in UTF-8) produced a garbled item content, since the old
// code sliced starting at BYTE offset 1 — the middle of the marker's own
// UTF-8 encoding, not right after it.
func TestBulletListDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bullet list interrupted by a DIFFERENT bullet character with no blank line warns; true EOF right after the last item does not",
			"Different bullets:\n\n- item 1\n\n+ item 1\n\n* item 1\n- item 1\n",
			"<document>\n    <paragraph>\n        Different bullets:\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item 1\n    <bullet_list bullet=\"+\">\n        <list_item>\n            <paragraph>\n                item 1\n    <bullet_list bullet=\"*\">\n        <list_item>\n            <paragraph>\n                item 1\n    <system_message level=\"2\" line=\"8\" type=\"WARNING\">\n        <paragraph>\n            Bullet list ends without a blank line; unexpected unindent.\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item 1\n",
		},
		{
			"an item interrupted by ordinary text with no blank line warns",
			"- item\nno blank line\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Bullet list ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n",
		},
		{
			"a bare (contentless) item interrupted the same way still warns",
			"-\nempty item above, no blank line\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Bullet list ends without a blank line; unexpected unindent.\n    <paragraph>\n        empty item above, no blank line\n",
		},
		{
			"Unicode bullet markers keep their own item content intact, not garbled by a byte-offset slice into the marker's own multi-byte encoding",
			"Unicode bullets:\n\n• BULLET\n\n‣ TRIANGULAR BULLET\n\n⁃ HYPHEN BULLET\n",
			"<document>\n    <paragraph>\n        Unicode bullets:\n    <bullet_list bullet=\"•\">\n        <list_item>\n            <paragraph>\n                BULLET\n    <bullet_list bullet=\"‣\">\n        <list_item>\n            <paragraph>\n                TRIANGULAR BULLET\n    <bullet_list bullet=\"⁃\">\n        <list_item>\n            <paragraph>\n                HYPHEN BULLET\n",
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
