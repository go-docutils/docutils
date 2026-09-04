package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestBareIndirectTargetName(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantOK   bool
	}{
		{"another_", "another", true},
		{"https://example.com", "", false},
		{"https://example.com/foo_", "", false}, // ends in "_" but not a bare simplename
		{"_", "", false},                        // no name before the underscore
		{"", "", false},
	}
	for _, tc := range cases {
		name, ok := bareIndirectTargetName(tc.in)
		if ok != tc.wantOK || name != tc.wantName {
			t.Errorf("bareIndirectTargetName(%q) = (%q, %v), want (%q, %v)", tc.in, name, ok, tc.wantName, tc.wantOK)
		}
	}
}

func TestIndirectTargetResolution(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"one hop resolves to the direct target's uri",
			".. _target: another_\n\n.. _another: https://example.com\n\nSee target_.\n",
			"<document>\n    <target id=\"target\" name=\"target\" refname=\"another\">\n    <target id=\"another\" name=\"another\" refuri=\"https://example.com\">\n    <paragraph>\n        See \n        <reference name=\"target\" refname=\"target\" refuri=\"https://example.com\">\n            target\n        .\n",
		},
		{
			"a multi-hop chain resolves through every indirect target",
			".. _a: b_\n.. _b: c_\n.. _c: https://example.com\n\nSee a_.\n",
			"<document>\n    <target id=\"a\" name=\"a\" refname=\"b\">\n    <target id=\"b\" name=\"b\" refname=\"c\">\n    <target id=\"c\" name=\"c\" refuri=\"https://example.com\">\n    <paragraph>\n        See \n        <reference name=\"a\" refname=\"a\" refuri=\"https://example.com\">\n            a\n        .\n",
		},
		{
			// A cycle is genuinely a bug in the source; real docutils
			// resolves it via an odd special-cased same-document
			// self-reference this parser doesn't replicate (see
			// resolveIndirect's own doc comment) — here it stays
			// unresolved, same as any other name resolveIndirect gives
			// up on, which now means it gets the same <problematic>
			// treatment any other dangling reference does.
			"a cycle doesn't loop forever, and the reference it leaves dangling is reported like any other",
			".. _a: b_\n.. _b: a_\n\nSee a_.\n",
			"<document>\n    <target id=\"a\" name=\"a\" refname=\"b\">\n    <target id=\"b\" name=\"b\" refname=\"a\">\n    <paragraph>\n        See \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            a\n        .\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-1\" id=\"system-message-1\">\n            <paragraph>\n                Unknown target name: \"a\".\n",
		},
		{
			"a url that happens to end in an underscore is not mistaken for an indirect target",
			".. _weird: https://example.com/foo_\n\nSee weird_.\n",
			"<document>\n    <target id=\"weird\" name=\"weird\" refuri=\"https://example.com/foo_\">\n    <paragraph>\n        See \n        <reference name=\"weird\" refname=\"weird\" refuri=\"https://example.com/foo_\">\n            weird\n        .\n",
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

// TestExplicitTargetGetsAnID guards a real, previously-missing attribute:
// every NAMED explicit hyperlink target (".. _name: uri") gets an "id" —
// real docutils' own Target.run always calls set_id on it, verified
// directly against the foreign judge even for the bare, unreferenced,
// no-substitution case — parseHyperlinkTarget never set one at all
// before this fix. Only 11 of 579 corpus fixtures happen to expect
// ids= on a <target>, and every one of THOSE reaches a different code
// path (an inline target, an embedded URI) that already set id
// correctly, so this specific construct's own gap stayed invisible
// until a fixture combined it with a substitution reference
// (test_directives/test_replace.py[3]).
func TestExplicitTargetGetsAnID(t *testing.T) {
	src := ".. _Python: http://www.python.org/\n"
	got := doctree.Dump(Parse(src))
	want := "<document>\n    <target id=\"python\" name=\"python\" refuri=\"http://www.python.org/\">\n"
	if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", src, got, want)
	}
}
