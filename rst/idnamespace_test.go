package rst

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestEveryIDComesFromOneNamespace pins the two properties of
// document.set_id that every id-bearing node shares. v0.94.0 gave them
// to sections; a directive's :name: option and an embedded-URI target
// still computed make_id() directly, so they neither disambiguated
// against other ids nor had a fallback when make_id came back EMPTY.
//
//   - a name with no ASCII-alphanumeric start ("1") has an empty
//     make_id, and the id falls back to make_id(tagname) + a counter --
//     "target-1", "note-1", "table-1". Claiming the empty string
//     instead produced id="" and then id="-1".
//   - a name colliding with an id already taken gets the "-1" suffix
//     from the SHARED set.
//
// Every expectation is the reference's own.
func TestEveryIDComesFromOneNamespace(t *testing.T) {
	ids := func(dump string) []string {
		return regexp.MustCompile(`id="([^"]*)"`).FindAllString(dump, -1)
	}
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			// Two footnote-style links: docutils numbers the targets by
			// tag because "1" and "2" have no usable make_id.
			"embedded-URI targets named by digits",
			"See `1 <http://a>`_ and `2 <http://b>`_.\n",
			[]string{`id="target-1"`, `id="target-2"`},
		},
		{
			"a directive :name: of digits falls back to its TAG",
			".. note::\n   :name: 1\n\n   body\n",
			[]string{`id="note-1"`},
		},
		{
			"and the tag really is the directive's own",
			".. table:: T\n   :name: 1\n\n   ===  ===\n   a    b\n   ===  ===\n",
			[]string{`id="table-1"`},
		},
		{
			// The control: an ordinary name is unchanged, and a second
			// one colliding with it takes the suffix.
			"two directives with the same :name: share one namespace",
			".. note::\n   :name: n\n\n   a\n\n.. note::\n   :name: n\n\n   b\n",
			[]string{`id="n"`, `id="n-1"`},
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
