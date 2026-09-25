package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestInvalidClassArgument covers directives/misc.py's Class.run (read
// directly): its argument goes through directives.class_option, which is
// one nodes.make_id per whitespace-separated token and raises ValueError
// on any token whose id comes out EMPTY -- caught by run() into
//
//	Invalid class attribute value for "class" directive: "<the whole argument>".
//
// Note WHICH text the message quotes: the whole argument as written, not
// the offending token, and not the normalized form. Before v0.136.5 this
// package silently dropped the empty ids and applied the rest, so
// ".. class:: 1" produced a class-less block instead of an error --
// costing two real-world corpus files (sphinx's own cpp-domain pages,
// which write ".. class:: template<typename T>"-shaped arguments).
//
// The last two cases are the CONTROLS, and they are the reason this test
// can fail in both directions: a valid argument must still be accepted,
// including a MULTI-LINE one whose tokens contain punctuation that
// make_id merely folds away rather than erasing ("template<typename T,"
// -> "template-typename"). A rule that rejected those would pass every
// error case above and break the two files this round fixed.
func TestInvalidClassArgument(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a digit-only token has no id at all",
			".. class:: 1\n\n   para\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid class attribute value for \"class\" directive: \"1\".\n        <literal_block>\n            .. class:: 1\n            \n               para\n",
		},
		{
			"punctuation only",
			".. class:: ---\n\n   para\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid class attribute value for \"class\" directive: \"---\".\n        <literal_block>\n            .. class:: ---\n            \n               para\n",
		},
		{
			"ONE bad token rejects the whole argument, quoted whole",
			".. class:: good 1\n\n   para\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid class attribute value for \"class\" directive: \"good 1\".\n        <literal_block>\n            .. class:: good 1\n            \n               para\n",
		},
		{
			"CONTROL: a valid multi-line argument still applies",
			".. class:: a\n   b\n\n   para\n",
			"<document>\n    <paragraph class=\"a b\">\n        para\n",
		},
		{
			"CONTROL: punctuation make_id FOLDS is not punctuation it erases",
			".. class:: template<typename T,\n   int N>\n\n   para\n",
			"<document>\n    <paragraph class=\"template-typename t int n\">\n        para\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
