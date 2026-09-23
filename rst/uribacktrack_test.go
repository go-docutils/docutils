package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestImplicitURIBacktracking covers what a regular expression does for
// free and a per-position scanner has to be told.
//
// docutils matches a standalone URI (and an email address) with ONE
// pattern that ends in "uri_end" followed by "end_string_suffix". When
// the longest span fails that trailing lookahead the regex engine
// BACKTRACKS to a shorter span that satisfies it. This package checked
// the boundary once and gave up, so a URI merely FOLLOWED by an unusual
// character became plain text — pytest's changelog template writes
// "https://github.com/pytest-dev/pytest/issues/{{ value[1:] }}", and "{"
// is neither a URI character nor a closer.
//
// Every expectation is real docutils 0.23's own output, and the whole
// set comes from a 672-case differential probe (8 URI/email shapes x 42
// following characters and strings, each bare and inside parentheses)
// which now agrees on all 672.
func TestImplicitURIBacktracking(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a URI followed by a brace links its longest valid prefix",
			"See https://e.org/issues/{{ x }} end\n",
			"<document>\n    <paragraph>\n        See \n        <reference refuri=\"https://e.org/issues\">\n            https://e.org/issues\n        /{{ x }} end\n",
		},
		{
			// CONTROL, and the line the corpus file actually contains:
			// its URI is an EXPLICIT embedded target, which never goes
			// through the implicit scan at all. What the corpus file
			// needed fixing for is the OTHER URI on the same line, the
			// bare one above. Kept because "the corpus case" and "the
			// defect" are not the same string, and a reader will look
			// for this one.
			"the pytest changelog template's own embedded URI is untouched",
			"- `{{ value }} <https://e.org/issues/{{ value[1:] }}>`_ end\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                <reference name=\"{{ value }}\" refuri=\"https://e.org/issues/{{value[1:]}}\">\n                    {{ value }}\n                <target id=\"value\" name=\"{{ value }}\" refuri=\"https://e.org/issues/{{value[1:]}}\">\n                 end\n",
		},
		{
			// The EMAIL half of the same rule. emailc contains both "/"
			// and "{", so the host ran to the brace and the single
			// boundary check then refused the whole address.
			"an email address followed by a brace does the same",
			"See user@e.org/{ end\n",
			"<document>\n    <paragraph>\n        See \n        <reference refuri=\"mailto:user@e.org\">\n            user@e.org\n        /{ end\n",
		},
		{
			// A bare "name_" reference is another alternative of the
			// same patterns.initial, found in the FIRST pass — so an
			// implicit URI cannot swallow one, any more than it can
			// swallow an emphasis start-string.
			"a trailing underscore is a reference, not part of the URI",
			"See file://x/y_ end\n",
			"<document>\n    <paragraph>\n        See \n        <reference refuri=\"file://x/\">\n            file://x/\n        <reference name=\"y\" refname=\"y\">\n            y\n         end\n",
		},
		{
			// CONTROL: nothing to backtrack past — the boundary is
			// already valid, and the whole URI is linked.
			"a URI followed by a space is unchanged",
			"See https://e.org/issues/ok end\n",
			"<document>\n    <paragraph>\n        See \n        <reference refuri=\"https://e.org/issues/ok\">\n            https://e.org/issues/ok\n         end\n",
		},
		{
			// CONTROL: "{" IS an emailc character, so this address runs
			// past it and ends on "x", with the closer "}" after — no
			// backtracking, and the result must not change.
			"an email address may contain a brace",
			"See user@e.org{x} end\n",
			"<document>\n    <paragraph>\n        See \n        <reference refuri=\"mailto:user@e.org{x\">\n            user@e.org{x\n        } end\n",
		},
		{
			// CONTROL: an EXPLICIT embedded URI is not scanned by this
			// path at all, so a brace inside one stays in the URI.
			"a brace inside an embedded URI is untouched",
			"`text <https://e.org/a{b}c>`_\n",
			"<document>\n    <paragraph>\n        <reference name=\"text\" refuri=\"https://e.org/a{b}c\">\n            text\n        <target id=\"text\" name=\"text\" refuri=\"https://e.org/a{b}c\">\n",
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
