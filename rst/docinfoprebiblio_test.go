package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDocInfoStepsOverPreBibliographicNodes covers the FIRST half of
// transforms/frontmatter.py's DocInfo.apply (read directly):
//
//	index = document.first_child_not_matching_class(nodes.PreBibliographic)
//
// The field list it promotes need not be the document's own first child
// -- it is the first child that is not PreBibliographic, a class whose
// members are meta, comment, decoration, system_message, target and
// substitution_definition. This package required position 0 EXACTLY,
// which was harmless only for as long as nothing ever preceded the field
// list. Fixing the ".. meta::" hoist to docutils' own insertion point
// (meta.go, the same round) put the meta nodes AHEAD of a leading field
// list, at which point the two features became mutually exclusive: the
// docinfo silently stopped being promoted at all, in a document that had
// both.
//
// The two CONTROLS are what make this test able to fail in the other
// direction: skipping the leading nodes must not turn into skipping
// ANYTHING, so a paragraph before the field list still refuses promotion
// (it is not PreBibliographic), and a bare parse still promotes nothing
// at all -- DocInfo is a transform, which is why this whole file goes
// through parsePromotingDocInfo and not Parse.
func TestDocInfoStepsOverPreBibliographicNodes(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// The insertion point is the SECOND half of the same
			// function: first_child_not_matching_class((Titular,
			// Decorative, meta)), so the docinfo lands AFTER the meta
			// and BEFORE the comment -- not where the field list was.
			"a comment and a hoisted meta are both stepped over",
			".. a comment\n\n:date: 2026-01-01\n\n.. meta::\n   :name: content\n",
			"<document>\n    <meta content=\"content\" name=\"name\">\n    <docinfo>\n        <date>\n            2026-01-01\n    <comment>\n        a comment\n",
		},
		{
			"a leading internal target is stepped over too",
			".. _t: http://x/\n\n:date: 2026-01-01\n",
			"<document>\n    <docinfo>\n        <date>\n            2026-01-01\n    <target id=\"t\" name=\"t\" refuri=\"http://x/\">\n",
		},
		{
			"CONTROL: a paragraph is not PreBibliographic, so nothing is promoted",
			"para\n\n:date: 2026-01-01\n",
			"<document>\n    <paragraph>\n        para\n    <field_list>\n        <field>\n            <field_name>\n                date\n            <field_body>\n                <paragraph>\n                    2026-01-01\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(parsePromotingDocInfo(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("parsePromotingDocInfo(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
	t.Run("CONTROL: a bare parse promotes nothing, even with no leading node at all", func(t *testing.T) {
		got := doctree.Dump(Parse(":date: 2026-01-01\n"))
		if !strings.Contains(got, "<field_list>") || strings.Contains(got, "<docinfo>") {
			t.Errorf("Parse should leave a plain field_list, got:\n%s", got)
		}
	})
}
