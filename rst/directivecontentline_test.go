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
