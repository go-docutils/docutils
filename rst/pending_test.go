package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestPendingDirectives covers the directives whose real work happens in a
// later TRANSFORM, and which therefore parse to a <pending> placeholder
// carrying the transform's name and options.
//
// This project has no transform system and is not getting one -- but that
// was never what these needed. A parse tree holds the PLACEHOLDER; nothing
// has to run. The odd indentation is nodes.pending.pformat's own, read
// directly: ".transform" and ".details" indented 5, each detail indented 7
// as "key: <repr>", details SORTED BY KEY, values rendered as Python's %r
// would (a string quoted, an integer bare, a list as ['a', 'b']).
//
// Every expectation below is byte-for-byte what the reference produces.
func TestPendingDirectives(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// Class names go through the same normalization every other
			// :class: option uses, and runs of whitespace collapse.
			"class",
			".. class:: class1  class2\n",
			"<document>\n    <pending>\n        .. internal attributes:\n             .transform: docutils.transforms.misc.ClassAttribute\n             .details:\n               class: ['class1', 'class2']\n               directive: 'class'\n",
		},
		{
			// The alias records its OWN name in the details, so "class" and
			// "rst-class" are distinguishable downstream.
			"rst-class records the alias it was written as",
			".. rst-class:: Custom Name\n",
			"<document>\n    <pending>\n        .. internal attributes:\n             .transform: docutils.transforms.misc.ClassAttribute\n             .details:\n               class: ['custom', 'name']\n               directive: 'rst-class'\n",
		},
		{
			// depth is an integer, so it prints unquoted; prefix is a
			// string and does not.
			"sectnum keeps only the options actually given",
			".. sectnum::\n   :depth: 2\n   :prefix: P\n",
			"<document>\n    <pending>\n        .. internal attributes:\n             .transform: docutils.transforms.parts.SectNum\n             .details:\n               depth: 2\n               prefix: 'P'\n",
		},
		{
			"target-notes with no options has an empty details block",
			".. target-notes::\n",
			"<document>\n    <pending>\n        .. internal attributes:\n             .transform: docutils.transforms.references.TargetNotes\n             .details:\n",
		},
		{
			"target-notes with a class",
			".. target-notes:: :class: custom\n",
			"<document>\n    <pending>\n        .. internal attributes:\n             .transform: docutils.transforms.references.TargetNotes\n             .details:\n               class: ['custom']\n",
		},
		{
			// The odd one out: title leaves NO node at all, setting an
			// attribute on the document itself.
			"title sets a document attribute and leaves no node",
			".. title:: Doc Title\n",
			"<document title=\"Doc Title\">\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
