package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestScanSimpleName(t *testing.T) {
	cases := []struct {
		in   string
		want string // the matched prefix
	}{
		{"real_target_", "real_target"},
		{"some--name_", "some"}, // double separator breaks the match
		{"-foo_", ""},           // leading separator: no match at all
		{"foo-bar_baz_", "foo-bar_baz"},
		{"plain_", "plain"},
		{"a.b.c_", "a.b.c"},
		{"trailing-_", "trailing"}, // separator with nothing alphanumeric after it
		{"", ""},
	}
	for _, tc := range cases {
		runes := []rune(tc.in)
		got := string(runes[:scanSimpleName(runes, 0)])
		if got != tc.want {
			t.Errorf("scanSimpleName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBareReferenceUnderscoredName(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an underscore-separated name resolves as a whole, not split at the first underscore",
			"See real_target_ here.\n\n.. _real_target: https://example.com\n",
			"<document>\n    <paragraph>\n        See \n        <reference name=\"real_target\" refname=\"real_target\" refuri=\"https://example.com\">\n            real_target\n         here.\n    <target name=\"real_target\" refuri=\"https://example.com\">\n",
		},
		{
			"an indirect target whose own name contains an underscore chases correctly",
			"See chained_.\n\n.. _chained: real_target_\n.. _real_target: https://example.com/real\n",
			"<document>\n    <paragraph>\n        See \n        <reference name=\"chained\" refname=\"chained\" refuri=\"https://example.com/real\">\n            chained\n        .\n    <target name=\"chained\" refname=\"real_target\">\n    <target name=\"real_target\" refuri=\"https://example.com/real\">\n",
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
