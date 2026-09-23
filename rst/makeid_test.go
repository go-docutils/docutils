package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestMakeID pins docutils.nodes.make_id, whose five steps this package
// used to approximate with a hand-written rune-to-rune fold. Every
// expectation here is real docutils 0.23's own answer, and the whole set
// comes from a differential probe of 3992 inputs — one for every rune
// whose NFKD decomposition contains any ASCII, a sample of 1500 that
// contains none, and every printable ASCII character in three positions
// — which now agrees on all 3992.
func TestMakeID(t *testing.T) {
	cases := []struct{ in, want string }{
		// The apostrophe two real PEPs use in a link title. U+2019 is
		// DELETED by the ascii-ignore encode; the old fold turned it
		// into a hyphen, so the anchor was "what-s-new...".
		{"What’s New In Python 3.8: API and Feature Removals", "whats-new-in-python-3-8-api-and-feature-removals"},
		// The three digraphs the old table mapped to ONE letter each.
		{"straße", "strasze"},
		{"ÆON œuvre", "aeon-oeuvre"},
		{"Ǆ digraph", "dz-digraph"},
		// _non_id_translate: letters NFKD does not decompose, because a
		// stroke or a hook is part of the letter.
		{"ØRE đub ħat ıota łódź ŧop", "ore-dub-hat-iota-lodz-top"},
		// NFKD proper: decomposition, compatibility forms, ligatures.
		{"Ünicode Ätest", "unicode-atest"},
		{"café", "cafe"},
		{"é combining", "e-combining"},
		{"ﬁle ligature", "file-ligature"},
		{"ＦＵＬＬＷＩＤＴＨ", "fullwidth"},
		{"Ⅷ roman", "viii-roman"},
		{"x²+y³", "x2-y3"},
		{"İstanbul", "istanbul"},
		// CONTROL: the fold can produce UPPERCASE ASCII, and nothing
		// lowercases it afterwards — make_id lowercases FIRST. "℉"
		// decomposes to "°F", the degree sign is dropped, and the "F"
		// is then a non-identifier character like any other.
		{"℉ degrees", "degrees"},
		// CONTROL: a fold that is all digits is stripped by the same
		// "^[-0-9]+" rule that strips a leading digit.
		{"¼ cup", "cup"},
		{"0start", "start"},
		// CONTROL: a script with no ASCII in its decomposition yields
		// NOTHING — not a string of hyphens.
		{"中文标题", ""},
		{"Ελληνικά", ""},
		{"Привет мир", ""},
		{"-–—", ""},
		{"1", ""},
		{"--", ""},
		// A RUN of non-identifier characters collapses to ONE hyphen,
		// whitespace included.
		{"a  b", "a-b"},
		{"multi   space", "multi-space"},
		{"a(b)c", "a-b-c"},
		{"naïve — dash", "naive-dash"},
		{"under_score", "under-score"},
		{"dotted.name", "dotted-name"},
		{"colon:name", "colon-name"},
		{"  leading  ", "leading"},
		{"-lead-", "lead"},
		{"trailing---", "trailing"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := MakeID(tc.in); got != tc.want {
				t.Errorf("MakeID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestClassOptionUsesMakeID covers directives.class_option, which is one
// make_id per whitespace-separated token and nothing else. Both of this
// package's copies reimplemented make_id instead of calling it, and a
// real corpus file caught the difference: ":class: longtable, borderless"
// kept the comma's hyphen, giving the class "longtable-".
func TestClassOptionUsesMakeID(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a comma-separated class list loses the commas, not to a trailing hyphen",
			".. note::\n   :class: longtable, borderless\n\n   Body.\n",
			`class="longtable borderless"`,
		},
		{
			"an accented class name folds the same way an id does",
			".. note::\n   :class: café\n\n   Body.\n",
			`class="cafe"`,
		},
		{
			// CONTROL, and a DOCUMENTED divergence: make_id cannot
			// produce "1", so docutils raises there ("cannot make "1"
			// into a class name", an ERROR that replaces the whole
			// directive). This package's lenient class path keeps the
			// directive and drops the token — which is still closer
			// than what it did before, keeping a class named "1" that
			// make_id could never yield.
			"a digit-only class name is dropped, not kept",
			".. note::\n   :class: 1 real\n\n   Body.\n",
			`class="real"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain %s", tc.source, got, tc.want)
			}
		})
	}
}

// TestTableColwidthsClassOrder pins the order the colwidths annotation
// and the ":class:" values appear in, which is OPPOSITE for the two
// table families — not because of any rule about tables, but because
// RSTTable.run appends ":class:" to a table the nested parse already
// built and adds the annotation after, while ListTable and CSVTable set
// the annotation while BUILDING their table, before run() appends
// anything. Asked of docutils 0.23 directly: reading run() alone
// suggests the same order for both.
func TestTableColwidthsClassOrder(t *testing.T) {
	simple := "\n\n   ======= =======\n   h1      h2\n   ======= =======\n   c1      c2\n   ======= =======\n"
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an rst table puts its own classes first",
			".. table::\n   :class: longtable\n   :widths: 30,70" + simple,
			`<table class="longtable colwidths-given">`,
		},
		{
			"a list-table puts the colwidths annotation first",
			".. list-table:: t\n   :class: deprecated\n   :widths: 40, 60\n\n   * - a\n     - b\n",
			`<table class="colwidths-given deprecated">`,
		},
		{
			"a csv-table does too",
			".. csv-table:: t\n   :class: dep\n   :widths: 40, 60\n\n   a, b\n",
			`<table class="colwidths-given dep">`,
		},
		{
			// CONTROL: with no :widths: there is no annotation at all,
			// so the order cannot be read off this one.
			"no :widths: means no annotation",
			".. table::\n   :class: longtable, borderless" + simple,
			`<table class="longtable borderless">`,
		},
		{
			// CONTROL: ":widths: auto" is the other annotation, and it
			// follows the same placement rule.
			"an auto width annotation sits where the given one would",
			".. table::\n   :class: x\n   :widths: auto" + simple,
			`<table class="x colwidths-auto">`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain %s", tc.source, got, tc.want)
			}
		})
	}
}
