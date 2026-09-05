package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestSectionImplicitTarget covers assignSectionTargets/collectTargets'
// TagSection case: every section title is an implicit hyperlink target,
// docutils' own new_subsection + note_implicit_target, ported. Each case
// verified against the foreign judge (Parser().parse(), before the
// non-transform-ported details — dupnames/ambiguous-reference diagnostics —
// this project already doesn't implement for any other name collision).
func TestSectionImplicitTarget(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare section gets a name and a slug id",
			"Section Title\n=============\n\nContent.\n",
			"<document>\n    <section id=\"section-title\" name=\"section title\">\n        <title>\n            Section Title\n        <paragraph>\n            Content.\n",
		},
		{
			"a reference to a section title resolves to a same-document anchor by id",
			"See `My Section`_.\n\nMy Section\n==========\n\nContent.\n",
			"<document>\n    <paragraph>\n        See \n        <reference name=\"My Section\" refname=\"my section\" refuri=\"#my-section\">\n            My Section\n        .\n    <section id=\"my-section\" name=\"my section\">\n        <title>\n            My Section\n        <paragraph>\n            Content.\n",
		},
		{
			"accented letters fold to ASCII in the id but not in the name",
			"Café Notes\n==========\n\nContent.\n",
			"<document>\n    <section id=\"cafe-notes\" name=\"café notes\">\n        <title>\n            Café Notes\n        <paragraph>\n            Content.\n",
		},
		{
			"a title with no ASCII-alnum content gets the tag-name fallback, always suffixed",
			"1\n=\n\n2\n-\n",
			"<document>\n    <section id=\"section-1\" name=\"1\">\n        <title>\n            1\n        <section id=\"section-2\" name=\"2\">\n            <title>\n                2\n",
		},
		{
			// Content deliberately NOT "A."/"B." — those are valid
			// loweralpha-enumerator shapes (states.py's own
			// sequencepats, read directly) once alpha/roman enumerator
			// support exists, which would entangle this test (about
			// duplicate section id/name suffixing, nothing to do with
			// enumerated lists) with unrelated enum-parsing behavior.
			// Both sections are INVALIDATED, not merely disambiguated: two
			// implicit targets sharing a name is docutils' implicit/implicit
			// row -- the name moves to "dupname" on BOTH and neither stays
			// resolvable, with an INFO inside the second section right after
			// its title. Checked against the reference; this test used to
			// expect both sections to keep "name", which predates dupname
			// support entirely.
			"duplicate section titles are BOTH invalidated, with distinct ids",
			"Same\n====\n\nFirst paragraph.\n\nSame\n====\n\nSecond paragraph.\n",
			"<document>\n    <section dupname=\"same\" id=\"same\">\n        <title>\n            Same\n        <paragraph>\n            First paragraph.\n    <section dupname=\"same\" id=\"same-1\">\n        <title>\n            Same\n        <system_message backref=\"same-1\" level=\"1\" type=\"INFO\">\n            <paragraph>\n                Duplicate implicit target name: \"same\".\n        <paragraph>\n            Second paragraph.\n",
		},
		{
			"the trailing system-messages section is never given a name/id of its own",
			"See `broken`_ reference.\n",
			"<document>\n    <paragraph>\n        See \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            broken\n         reference.\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-1\" id=\"system-message-1\">\n            <paragraph>\n                Unknown target name: \"broken\".\n",
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
