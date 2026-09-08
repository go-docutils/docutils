package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestContentsDirective covers directives.parts.Contents' parse-time
// shape: a <topic> holding an optional <title> and the <pending> a later
// transform replaces. Every expectation is a bare Parser().parse() at
// the judge's report_level.
func TestContentsDirective(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
		absent       []string
	}{
		{
			"bare: the default title, and a name and id from it",
			"x\n\n.. contents::\n\nT\n=\n",
			[]string{`<topic class="contents" id="contents" name="contents">`, "<title>", "Contents",
				"docutils.transforms.parts.Contents"},
			nil,
		},
		{
			"an argument becomes the title, the name, and the id",
			"x\n\n.. contents:: Table of Contents\n\nT\n=\n",
			[]string{`<topic class="contents" id="table-of-contents" name="table of contents">`, "Table of Contents"},
			nil,
		},
		{
			// :local: is the one shape with NO title at all -- and it
			// still takes its name from the default label.
			"local: no title, class gains local, details carry depth",
			"x\n\n.. contents::\n   :local:\n   :depth: 2\n\nT\n=\n",
			// The <pending> follows the <topic> DIRECTLY: no title in
			// between. Checked as an adjacency rather than by the
			// absence of "<title>" anywhere, since the section below
			// legitimately has one.
			[]string{"<topic class=\"contents local\" id=\"contents\" name=\"contents\">\n        <pending>",
				"depth: 2", "local: None"},
			nil,
		},
		{
			// backlinks maps "none" onto None, not onto the string.
			"backlinks none prints as None",
			"x\n\n.. contents::\n   :backlinks: none\n\nT\n=\n",
			[]string{"backlinks: None"},
			[]string{"backlinks: 'none'"},
		},
		{
			"backlinks top keeps its string, and :class: appends",
			"x\n\n.. contents::\n   :backlinks: top\n   :class: c1\n\nT\n=\n",
			[]string{`class="contents c1"`, "backlinks: 'top'", "class: ['c1']"},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q:\n%s", w, got)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(got, a) {
					t.Errorf("unexpected %q:\n%s", a, got)
				}
			}
			if strings.Contains(got, "Unknown directive type") {
				t.Errorf("a REGISTERED directive was reported as unknown:\n%s", got)
			}
		})
	}
}
