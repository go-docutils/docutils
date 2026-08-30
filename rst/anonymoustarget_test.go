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
			"<document>\n    <target anonymous=\"true\" refuri=\"https://a.example\">\n    <target anonymous=\"true\" refuri=\"https://b.example\">\n    <paragraph>\n        See \n        <reference anonymous=\"true\" name=\"first\" refuri=\"https://a.example\">\n            first\n         and \n        <reference anonymous=\"true\" name=\"second\" refuri=\"https://b.example\">\n            second\n        .\n",
		},
		{
			"targets defined after their references still resolve the same way",
			"See first__ and second__.\n\n.. __: https://a.example\n.. __: https://b.example\n",
			"<document>\n    <paragraph>\n        See \n        <reference anonymous=\"true\" name=\"first\" refuri=\"https://a.example\">\n            first\n         and \n        <reference anonymous=\"true\" name=\"second\" refuri=\"https://b.example\">\n            second\n        .\n    <target anonymous=\"true\" refuri=\"https://a.example\">\n    <target anonymous=\"true\" refuri=\"https://b.example\">\n",
		},
		{
			"an anonymous reference with no matching target stays unresolved, not a crash",
			"See first__ and second__ but no target defined at all.\n",
			"<document>\n    <paragraph>\n        See \n        <reference anonymous=\"true\" name=\"first\">\n            first\n         and \n        <reference anonymous=\"true\" name=\"second\">\n            second\n         but no target defined at all.\n",
		},
		{
			"an indirect anonymous target chases through a named target to its refuri",
			".. _real: https://example.org/real\n\n.. __: real_\n\nit__\n",
			"<document>\n    <target name=\"real\" refuri=\"https://example.org/real\">\n    <target anonymous=\"true\" refname=\"real\">\n    <paragraph>\n        <reference anonymous=\"true\" name=\"it\" refuri=\"https://example.org/real\">\n            it\n",
		},
		{
			"an indirect anonymous target that fails to chase still consumes its document-order slot",
			".. __: nosuch_\n\n.. __: https://example.org/second\n\nfirst__ second__\n",
			"<document>\n    <target anonymous=\"true\" refname=\"nosuch\">\n    <target anonymous=\"true\" refuri=\"https://example.org/second\">\n    <paragraph>\n        <reference anonymous=\"true\" name=\"first\">\n            first\n         \n        <reference anonymous=\"true\" name=\"second\" refuri=\"https://example.org/second\">\n            second\n",
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
