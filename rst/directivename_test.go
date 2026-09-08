package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDirectiveNameIsASimpleName pins the grammar a directive name has
// to satisfy: docutils' own `simplename`, alphanumeric runs joined by
// "-._+:". scanSimpleName already implemented it for ROLES (v0.54.0);
// this path had a separate character-class loop that allowed a leading
// "_" and, more consequentially, did not allow ":" at all -- so every
// namespaced directive (".. rst:directive::", ".. py:function::", the
// shape Sphinx documentation is full of) fell through to the comment
// fallback instead of being reported as an unknown directive. 49
// real-world files turned on it, and the docutils testsuite corpus has
// none.
//
// The two sibling scans had drifted apart exactly the way the email and
// URI end-boundary checks had one round earlier.
//
// Every case below was run against the reference.
func TestDirectiveNameIsASimpleName(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{"a namespaced name is a directive", ".. rst:directive:: x\n", "unknown"},
		{"another namespace", ".. py:function:: f()\n", "unknown"},
		{"every separator at once", ".. a.b-c_d+e:f:: arg\n", "unknown"},
		{"a name may START with a digit", ".. 1note:: x\n", "unknown"},
		// The boundary in the other direction: simplename cannot START
		// with an underscore, so this is not a directive. What it IS
		// instead is a separate question this change does not touch --
		// docutils falls all the way through to a <comment>, while this
		// parser's hyperlink-target pattern claims it first and builds
		// <target name="x:" refuri="y">. A real divergence, in the
		// TARGET path rather than this one, and left as such: no corpus
		// file on either side writes ".. _x:: y".
		{"a leading underscore is NOT a name", ".. _x:: y\n", "not-a-directive"},
		{"a real directive still works", ".. note:: ok\n", "directive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			switch tc.want {
			case "unknown":
				if !strings.Contains(got, "Unknown directive type") {
					t.Errorf("not reported as an unknown directive:\n%s", got)
				}
				if strings.Contains(got, "<comment>") {
					t.Errorf("fell through to the comment fallback:\n%s", got)
				}
			case "not-a-directive":
				if strings.Contains(got, "Unknown directive type") || strings.Contains(got, "<directive") {
					t.Errorf("a leading underscore was taken as a directive name:\n%s", got)
				}
			case "directive":
				if strings.Contains(got, "system_message") {
					t.Errorf("a known directive drew a diagnostic:\n%s", got)
				}
			}
		})
	}
}
