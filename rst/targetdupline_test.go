package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDuplicateTargetNameLine pins the line a duplicate-name message
// about an explicit hyperlink TARGET carries: the target's OWN first
// line -- the ".. _name:" line, even when the URI continues below it.
//
// Footnotes, citations, substitution definitions and sections all
// recorded a line; a hyperlink target did not, so every such message
// about one came out with no line at all. 10 real-world files, and the
// docutils testsuite corpus never notices because its own duplicate
// fixtures put both targets on lines 1 and 2, where a missing line
// reads much like a right one.
//
// Note this is the target's own line, NOT the state-machine position
// v0.85.0 needed for the inline families -- two different conventions,
// each checked against the reference rather than assumed to be the
// other.
func TestDuplicateTargetNameLine(t *testing.T) {
	cases := []struct {
		name, source, wantLine, wantText string
	}{
		{
			"two targets with one name: the SECOND one's line",
			".. _dup: http://a\n.. _dup: http://b\n\ntext\n",
			`line="2"`, `Duplicate explicit target name: "dup".`,
		},
		{
			"the same pair further down the file",
			".. _dup: http://a\n\nx\n\n.. _dup: http://b\n\ntext\n",
			`line="5"`, `Duplicate explicit target name: "dup".`,
		},
		{
			"a target overriding an implicit name",
			"`<dup>`_ there.\n\n.. _dup: http://b\n\ntext\n",
			`line="3"`, `Target name overrides implicit target name "dup".`,
		},
		{
			// The URI on the following line does not move the report:
			// it is the marker's line, not the construct's last.
			"a target whose URI continues on the next line",
			".. _dup: http://a\n\n.. _dup:\n   http://b\n\ntext\n",
			`line="3"`, `Duplicate explicit target name: "dup".`,
		},
		{
			"two targets naming the SAME uri",
			".. _dup: http://a\n.. _dup: http://a\n\ntext\n",
			`line="2"`, `Duplicate name "dup" for external target "http://a".`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.wantText) {
				t.Fatalf("missing %q:\n%s", tc.wantText, got)
			}
			if !strings.Contains(got, tc.wantLine) {
				t.Errorf("missing %s:\n%s", tc.wantLine, got)
			}
		})
	}
}
