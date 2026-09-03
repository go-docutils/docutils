package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestInlineTarget(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// docutils/rst v0.32.0+ — an inline internal target now also
			// gets a slugified "id" attribute, matching every other
			// target kind (previously only "name" was set).
			"inline target keeps its visible text and a matching name attr",
			"See the _`important term` defined here.\n",
			"<document>\n    <paragraph>\n        See the \n        <target id=\"important-term\" name=\"important term\">\n            important term\n         defined here.\n",
		},
		{
			"a later reference to the same term resolves to a same-document anchor",
			"See the _`important term` and later refer to `important term`_.\n",
			"<document>\n    <paragraph>\n        See the \n        <target id=\"important-term\" name=\"important term\">\n            important term\n         and later refer to \n        <reference name=\"important term\" refname=\"important term\" refuri=\"#important-term\">\n            important term\n        .\n",
		},
		{
			"resolution is case-insensitive, same as any other reference name",
			"_`Important Term` ... `important term`_.\n",
			"<document>\n    <paragraph>\n        <target id=\"important-term\" name=\"important term\">\n            Important Term\n         ... \n        <reference name=\"important term\" refname=\"important term\" refuri=\"#important-term\">\n            important term\n        .\n",
		},
		{
			// docutils/rst v0.36.0+ — the closing backquote of an inline
			// target must itself be followed by a valid end boundary
			// (whitespace/closer/delimiter/EOF); a trailing "_" right
			// after it is a WORD character, not a valid boundary, so real
			// docutils keeps searching for a LATER close that never
			// comes here — this used to silently leave the whole
			// construct as unremarked plain text instead of the
			// problematic/warning real docutils gives.
			"a closing backquote immediately followed by an invalid boundary character never closes, becoming an unclosed-target problematic",
			"With simple-inline-markup, _`this`_ is a a target followed by an\nunderscore.\n",
			"<document>\n    <paragraph>\n        With simple-inline-markup, \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            _`\n        this`_ is a a target followed by an\n        underscore.\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline target start-string without end-string.\n",
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
