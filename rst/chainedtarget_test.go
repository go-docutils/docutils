package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// resolved parses with reference resolution on, which is the only mode in which
// a target's destination is visible at all: a bare parse leaves a reference
// carrying its refname and nothing else, which is why the two corpus sweeps --
// both comparing against docutils' own bare parse -- cannot see this defect and
// did not move when it was fixed.
func resolved(src string) string {
	return doctree.Dump(ParseWithOptions(src, Options{
		ResolveReferences:        true,
		ReportDanglingReferences: true,
	}))
}

// TestABareTargetChainedOntoAURLInheritsIt pins PropagateTargets' effect on a
// target's DESTINATION. A ".. _name:" with no reference of its own hands its
// names to the next node, so when that node is another target the name lands on
// it and inherits what it points at. Each expectation was taken from the
// reference run on that exact source.
//
// Each expectation names the <reference> ELEMENT, not just the refuri. The
// first version matched the refuri anywhere in the dump, where the chained
// TARGET carries it too -- so three of the four cases passed with the defect
// still in place, and only the negative case below witnessed anything.
func TestABareTargetChainedOntoAURLInheritsIt(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{
			name: "chained onto a URL",
			src:  "See pythondoc_.\n\n.. _pythondoc:\n.. _gendoc: http://example.com/gendoc\n",
			want: `<reference name="pythondoc" refname="pythondoc" refuri="http://example.com/gendoc">`,
		},
		{
			name: "chained onto an indirect target",
			src:  "See a_.\n\n.. _a:\n.. _b: c_\n\n.. _c: http://example.com/c\n",
			want: `<reference name="a" refname="a" refuri="http://example.com/c">`,
		},
		{
			// The other side of the rule, and the reference's answer too: a
			// bare target chained onto another BARE one keeps its own id. The
			// node they both precede carries every one of the ids, so each
			// fragment lands in the same place. Like the CONTROL below it
			// passes either way -- it is here to pin the boundary of the rule,
			// not to witness it.
			name: "chained onto another bare target keeps its own id",
			src:  "See a_.\n\n.. _a:\n.. _b:\n\nSection One\n===========\n",
			want: `<reference name="a" refname="a" refuri="#a">`,
		},
		{
			// CONTROL: an unchained bare target is a same-document anchor and
			// was always right. It passes either way.
			name: "an unchained bare target is still an anchor",
			src:  "See lone_.\n\n.. _lone:\n\nSection Here\n============\n",
			want: `<reference name="lone" refname="lone" refuri="#lone">`,
		},
	}
	for _, c := range cases {
		got := resolved(c.src)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want %s in:\n%s", c.name, c.want, got)
		}
	}
}

// TestAChainedTargetIsNotASameDocumentAnchor is the negative half: the wrong
// answer must be GONE, not merely accompanied by the right one.
func TestAChainedTargetIsNotASameDocumentAnchor(t *testing.T) {
	got := resolved("See pythondoc_.\n\n.. _pythondoc:\n.. _gendoc: http://example.com/gendoc\n")
	if strings.Contains(got, `refuri="#pythondoc"`) {
		t.Errorf("the name still resolves to a fragment nothing carries:\n%s", got)
	}
}

// TestAnInlineInternalTargetIsStillItsOwnDestination guards the distinction the
// fix turns on: an inline "_`text`" target carries its own visible content and
// IS the destination, so it must not be chained to whatever follows it. Without
// the no-children test in isBareBlockTarget, an inline target next to a
// hyperlink target would have inherited that target's URL.
func TestAnInlineInternalTargetIsStillItsOwnDestination(t *testing.T) {
	got := resolved("A paragraph with _`an anchor` in it.\n\nSee `an anchor`_.\n\n.. _other: http://example.com/\n")
	if !strings.Contains(got, `refuri="#an-anchor"`) {
		t.Errorf("an inline internal target is its own destination:\n%s", got)
	}
}
