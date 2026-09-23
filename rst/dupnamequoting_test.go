package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDuplicateNameMessageQuoting covers all four messages nodes.py
// builds for a name collision. Each writes the name between LITERAL
// quote characters:
//
//	s = f'Duplicate name "{name}" for external target "{ref}".'
//
// The Go spelling used %q, which looks like the same thing and is not:
// %q is strconv.Quote, so it ESCAPES the string it interpolates. Any
// name carrying a quote or a backslash of its own came out wrong — PEP
// 446 links twice to `Python issue #16500 "Add an atfork module"`, and
// the message named it with \" inside.
//
// Every expectation is real docutils 0.23's own output, and the
// backslash case is the CONTROL that distinguishes the two spellings a
// second way: %q would double it.
func TestDuplicateNameMessageQuoting(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"two references to one destination",
			"A `Say \"hi\" <http://e.org/>`_ and `Say \"hi\" <http://e.org/>`_.\n",
			`Duplicate name "say "hi"" for external target "http://e.org/".`,
		},
		{
			"two explicit targets, different destinations",
			".. _`say \"hi\"`: http://a.org/\n.. _`say \"hi\"`: http://b.org/\n",
			`Duplicate explicit target name: "say "hi"".`,
		},
		{
			"an explicit target overriding a section's implicit one",
			"Say \"hi\"\n========\n\n.. _`say \"hi\"`: http://a.org/\n",
			`Target name overrides implicit target name "say "hi"".`,
		},
		{
			"two sections with the same title",
			"Say \"hi\"\n========\n\nx\n\nSay \"hi\"\n========\n\ny\n",
			`Duplicate implicit target name: "say "hi"".`,
		},
		{
			// CONTROL: a backslash in the name stays single. %q would
			// double it, so this fails for the same reason the quote
			// cases do without saying "quote" anywhere.
			"a backslash in the name is not doubled",
			"A `a\\\\b <http://e.org/>`_ and `a\\\\b <http://e.org/>`_.\n",
			`Duplicate name "a\b" for external target "http://e.org/".`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
