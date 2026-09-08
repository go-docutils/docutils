package rst

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestSectionIDsShareOneNamespace pins that a section's id is claimed
// from the SAME set as every other id in the document. document.set_id
// has one namespace per document; this parser had two, because
// assignSectionTargets kept a private map, so a section whose make_id
// matched an explicit target's id silently reused it -- two nodes with
// id="get-started", which is not a valid document.
//
// ".. _get-started:" above a "Get Started" heading is how pytest's and
// sphinx's documentation label their pages, so 46 real-world files
// carried the duplicate; no docutils testsuite fixture pairs a target
// with a section whose title normalizes to the same id.
//
// The expectations come from a BARE Parser().parse(), the way the
// corpus judge runs -- not publish_string, whose transforms merge a
// target into the section that follows it and would have shown a single
// node carrying both ids.
func TestSectionIDsShareOneNamespace(t *testing.T) {
	ids := func(dump string) []string {
		return regexp.MustCompile(`id="([^"]*)"`).FindAllString(dump, -1)
	}
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			"an explicit target, then a section that would take its id",
			".. _get-started:\n\nGet Started\n===========\n\ntext\n",
			[]string{`id="get-started"`, `id="get-started-1"`},
		},
		{
			"the same pair in the other order",
			"Get Started\n===========\n\n.. _get-started:\n\ntext\n",
			[]string{`id="get-started"`, `id="get-started-1"`},
		},
		{
			"two sections with the same title still disambiguate",
			"A\n=\n\nx\n\nA\n=\n\ny\n",
			[]string{`id="a"`, `id="a-1"`},
		},
		{
			// The control: no collision, no suffix. Without it a parser
			// that suffixed everything would pass the cases above.
			"no collision leaves both ids bare",
			".. _other:\n\nGet Started\n===========\n\ntext\n",
			[]string{`id="other"`, `id="get-started"`},
		},
		{
			"a target colliding with a LATER section",
			"A\n=\n\n.. _b:\n\nB\n=\n\ny\n",
			[]string{`id="a"`, `id="b"`, `id="b-1"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if g := ids(got); strings.Join(g, ",") != strings.Join(tc.want, ",") {
				t.Errorf("ids = %v, want %v:\n%s", g, tc.want, got)
			}
		})
	}
}
