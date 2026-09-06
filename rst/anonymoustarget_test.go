package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestAnonymousTargetResolution(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"targets defined before their references resolve by document-order position",
			".. __: https://a.example\n.. __: https://b.example\n\nSee first__ and second__.\n",
			"<document>\n    <target anonymous=\"1\" refuri=\"https://a.example\">\n    <target anonymous=\"1\" refuri=\"https://b.example\">\n    <paragraph>\n        See \n        <reference anonymous=\"1\" name=\"first\" refuri=\"https://a.example\">\n            first\n         and \n        <reference anonymous=\"1\" name=\"second\" refuri=\"https://b.example\">\n            second\n        .\n",
		},
		{
			"targets defined after their references still resolve the same way",
			"See first__ and second__.\n\n.. __: https://a.example\n.. __: https://b.example\n",
			"<document>\n    <paragraph>\n        See \n        <reference anonymous=\"1\" name=\"first\" refuri=\"https://a.example\">\n            first\n         and \n        <reference anonymous=\"1\" name=\"second\" refuri=\"https://b.example\">\n            second\n        .\n    <target anonymous=\"1\" refuri=\"https://a.example\">\n    <target anonymous=\"1\" refuri=\"https://b.example\">\n",
		},
		{
			"an indirect anonymous target chases through a named target to its refuri",
			".. _real: https://example.org/real\n\n.. __: real_\n\nit__\n",
			"<document>\n    <target id=\"real\" name=\"real\" refuri=\"https://example.org/real\">\n    <target anonymous=\"1\" refname=\"real\">\n    <paragraph>\n        <reference anonymous=\"1\" name=\"it\" refuri=\"https://example.org/real\">\n            it\n",
		},
		{
			"an indirect anonymous target that fails to chase still consumes its document-order slot",
			".. __: nosuch_\n\n.. __: https://example.org/second\n\nfirst__ second__\n",
			"<document>\n    <target anonymous=\"1\" refname=\"nosuch\">\n    <target anonymous=\"1\" refuri=\"https://example.org/second\">\n    <paragraph>\n        <reference anonymous=\"1\" name=\"first\">\n            first\n         \n        <reference anonymous=\"1\" name=\"second\" refuri=\"https://example.org/second\">\n            second\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(parseResolvingReferences(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestAnonymousAttributeIsOne pins the anonymous attribute's VALUE. It is
// a boolean in docutils (reference['anonymous'] = True), and
// nodes.Element.starttag renders a bool as str(int(value)) -- so every
// pseudoxml dump spells it anonymous="1". This parser wrote "true", and
// thirteen corpus fixtures differed from it on nothing else at all.
func TestAnonymousAttributeIsOne(t *testing.T) {
	got := doctree.Dump(parseResolvingReferences("ref__\n\n.. __: http://x\n"))
	if strings.Contains(got, `anonymous="true"`) {
		t.Errorf(`anonymous is spelled "true"; docutils writes "1":`+"\n%s", got)
	}
	for _, want := range []string{`<reference anonymous="1" name="ref"`, `<target anonymous="1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}
