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
