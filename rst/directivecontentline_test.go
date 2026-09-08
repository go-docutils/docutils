package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDiagnosticInsideDirectiveContentHasALine pins that a message
// raised INSIDE a directive's body carries a real source line.
//
// parseBlockLines takes a lineBase so a nested construct can map its own
// local indices back to absolute lines, and block quotes, list items,
// field bodies, option lists and definitions all compute one. Every
// DIRECTIVE passed -1, so anything diagnosed inside an admonition body
// came out with no line at all -- 27 real-world files, sphinx's and
// pytest's documentation being largely made of ".. warning::" and
// ".. note::" blocks.
//
// The mapping is exact rather than approximate: the combined block a
// directive hands to parseDirectiveBlock is built as [text after "::"] +
// [the blank lines under it] + [the body], mirroring lines[i:] one for
// one, so parseDirectiveBlockAt can report where the content begins and
// the caller adds i.
//
// Every expectation below is the reference's own.
func TestDiagnosticInsideDirectiveContentHasALine(t *testing.T) {
	cases := []struct {
		name, source, wantLine string
	}{
		{"a warning body", "text\n\n.. warning::\n\n    See :issue:`1435`.\n", `line="5"`},
		{"a note further down the file", "a\n\nb\n\nc\n\n.. note::\n\n    See :issue:`7`.\n", `line="9"`},
		{"an admonition with a title argument", "x\n\n.. admonition:: Title\n\n    See :issue:`7`.\n", `line="5"`},
		{
			// The second paragraph of a body: the base has to be the
			// content's own start, not the directive's line.
			"the second paragraph of a body",
			".. note::\n\n    one\n\n    See :issue:`7`.\n", `line="5"`,
		},
		{"a container body", "text\n\n.. container:: cls\n\n    See :issue:`7`.\n", `line="5"`},
		// v0.97.0 wired the rest of the directives that nest content.
		// The class and table directives split their options off
		// themselves rather than through parseDirectiveBlock, so their
		// content is a SUFFIX of the body and needs bodyStartIndex
		// instead of the block offset -- two derivations, both pinned.
		{"a class directive body", "x\n\n.. class:: c1\n\n   See :issue:`7`.\n", `line="5"`},
		{"a figure body", "x\n\n.. figure:: a.png\n\n   See :issue:`7`.\n", `line="5"`},
		// The table directives are threaded too, but their content IS a
		// table, and a table CELL still parses at -1 -- so nothing
		// diagnosed inside one can show it yet. Deliberately not tested
		// here rather than tested against the wrong shape: a cell needs
		// a per-row offset this round does not derive.
		{"a header directive body", ".. header::\n\n   See :issue:`7`.\n", `line="3"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, "Unknown interpreted text role") {
				t.Fatalf("no diagnostic raised at all:\n%s", got)
			}
			if !strings.Contains(got, tc.wantLine) {
				t.Errorf("missing %s:\n%s", tc.wantLine, got)
			}
		})
	}
}
