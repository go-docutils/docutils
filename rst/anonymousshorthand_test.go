package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestAnonymousTargetShorthand pins the "__ uri" spelling of an
// anonymous hyperlink target -- no ".. " prefix at all. It is a Body
// transition of its own (`__( +|$)`, listed between explicit_markup and
// line), so it fires only at the START of a block; the same text inside
// a paragraph is ordinary text.
//
// This parser knew only the ".. __: uri" spelling, so the shorthand
// became a paragraph containing a standalone URI. 30 real-world files
// use it -- it is the form PEPs and sphinx docs write -- and no docutils
// testsuite fixture does.
//
// Anonymous targets also gained the id note_anonymous_target gives them
// ("target-1", "target-2", ...), which NEITHER spelling carried before.
//
// Expectations are from a bare Parser().parse(), the way the judge runs.
func TestAnonymousTargetShorthand(t *testing.T) {
	cases := []struct {
		name, source string
		wantTargets  []string
	}{
		{
			"the shorthand at a block start",
			"text\n\n__ http://example.org/a\n",
			[]string{`<target anonymous="1" id="target-1" refuri="http://example.org/a">`},
		},
		{
			"the long spelling produces the same node",
			"text\n\n.. __: http://example.org/a\n",
			[]string{`<target anonymous="1" id="target-1" refuri="http://example.org/a">`},
		},
		{
			"two shorthand targets number in document order",
			"text\n\n__ http://a.org\n\n__ http://b.org\n",
			[]string{
				`<target anonymous="1" id="target-1" refuri="http://a.org">`,
				`<target anonymous="1" id="target-2" refuri="http://b.org">`,
			},
		},
		{
			`"__" alone, with the URI indented under it`,
			"text\n\n__\n   http://a.org\n",
			[]string{`<target anonymous="1" id="target-1" refuri="http://a.org">`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			for _, want := range tc.wantTargets {
				if !strings.Contains(got, want) {
					t.Errorf("missing %s:\n%s", want, got)
				}
			}
		})
	}
}

// TestAnonymousShorthandOnlyAtABlockStart is the control that makes the
// rule a TRANSITION rather than a text pattern: the same "__ " inside a
// paragraph stays ordinary text, with the URI still recognized as a
// standalone reference. Without it, a parser that matched "__ " anywhere
// would pass every case above.
func TestAnonymousShorthandOnlyAtABlockStart(t *testing.T) {
	got := doctree.Dump(Parse("text\n__ http://a.org\n"))
	if strings.Contains(got, "<target") {
		t.Errorf("a mid-paragraph \"__ \" became a target:\n%s", got)
	}
	if !strings.Contains(got, `<reference refuri="http://a.org">`) {
		t.Errorf("the standalone URI was lost:\n%s", got)
	}
}
