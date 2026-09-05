package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDuplicateNames walks docutils' set_duplicate_name transition table
// (see dupnames.go). Every case below was run against the reference
// implementation and matches it exactly in structure: which element keeps
// "name" and which is invalidated to "dupname", the ids, and the message's
// own text, level, backref and POSITION.
//
// The one thing deliberately not matched is the `line` attribute on these
// particular messages. docutils reports them against its own node.line
// bookkeeping rather than the line the duplicate sits on, and that is not
// a constant offset: probing it gave line 4 for a duplicate on line 3, 5
// for one on line 5, 6 for one on line 5 in a different shape, and 3 for
// one on line 3 inside a block quote. Three separate hypotheses ("the line
// after the paragraph", "the paragraph's first line + 1", "the duplicate's
// own line") each fitted some probes and failed others. Rather than invent
// a formula that happens to fit the corpus fixture, these carry this
// parser's ordinary convention -- the real source line -- and the one
// corpus case riding on the difference stays an honest mismatch.
func TestDuplicateNames(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// explicit + explicit: BOTH invalidated, WARNING.
			"two explicit targets with one name invalidate each other",
			".. _dup: http://a\n.. _dup: http://b\n\ntext\n",
			"<document>\n    <target dupname=\"dup\" id=\"dup\" refuri=\"http://a\">\n    <system_message level=\"2\" type=\"WARNING\">\n        <paragraph>\n            Duplicate explicit target name: \"dup\".\n    <target dupname=\"dup\" id=\"dup-1\" refuri=\"http://b\">\n    <paragraph>\n        text\n",
		},
		{
			// implicit over explicit: the EXPLICIT one wins and keeps its
			// name; only the newcomer is invalidated.
			"an implicit target loses to an explicit one already holding the name",
			"_`dup` here.\n\n`<dup>`_ there.\n",
			"<document>\n    <paragraph>\n        <target id=\"dup\" name=\"dup\">\n            dup\n         here.\n    <system_message level=\"1\" line=\"3\" type=\"INFO\">\n        <paragraph>\n            Duplicate implicit target name: \"dup\".\n    <paragraph>\n        <reference name=\"dup\" refuri=\"dup\">\n            dup\n        <target dupname=\"dup\" id=\"dup-1\" refuri=\"dup\">\n         there.\n",
		},
		{
			// explicit over implicit: the explicit one OVERRIDES, taking
			// the name from the implicit one rather than colliding.
			"an explicit target overrides an implicit one already holding the name",
			"`<dup>`_ there.\n\n_`dup` here.\n",
			"<document>\n    <paragraph>\n        <reference name=\"dup\" refuri=\"dup\">\n            dup\n        <target dupname=\"dup\" id=\"dup\" refuri=\"dup\">\n         there.\n    <system_message backref=\"dup-1\" level=\"1\" line=\"3\" type=\"INFO\">\n        <paragraph>\n            Target name overrides implicit target name \"dup\".\n    <paragraph>\n        <target id=\"dup-1\" name=\"dup\">\n            dup\n         here.\n",
		},
		{
			// The case checked AHEAD of the table: same destination, so
			// only the newcomer is invalidated and the message is about an
			// external target rather than a duplicate name.
			"two targets naming the same URI keep the first and report an external duplicate",
			".. _dup: http://a\n.. _dup: http://a\n\ntext\n",
			"<document>\n    <target id=\"dup\" name=\"dup\" refuri=\"http://a\">\n    <system_message level=\"1\" type=\"INFO\">\n        <paragraph>\n            Duplicate name \"dup\" for external target \"http://a\".\n    <target dupname=\"dup\" id=\"dup-1\" refuri=\"http://a\">\n    <paragraph>\n        text\n",
		},
		{
			// The message is BUILT and then DROPPED when msgnode cannot
			// hold body elements. Inside a line block msgnode is the
			// <line>, so the duplicate is invalidated silently -- the
			// corpus fixture's own title is "System messages are no longer
			// inserted between <line>s".
			"a duplicate inside a line block is invalidated with NO message at all",
			"| `uff <test1>`_\n| `uff <test2>`_\n",
			"<document>\n    <line_block>\n        <line>\n            <reference name=\"uff\" refuri=\"test1\">\n                uff\n            <target dupname=\"uff\" id=\"uff\" refuri=\"test1\">\n        <line>\n            <reference name=\"uff\" refuri=\"test2\">\n                uff\n            <target dupname=\"uff\" id=\"uff-1\" refuri=\"test2\">\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestReferenceNameIsNotATargetClaim guards the distinction that made the
// first version of this pass collide every target with its own reference:
// a <reference>'s "name" is display text (docutils keeps the name it
// POINTS AT in document.refnames, a different map), so it must never be
// treated as claiming the name as a target.
func TestReferenceNameIsNotATargetClaim(t *testing.T) {
	got := doctree.Dump(Parse("See the _`important term` and later refer to `important term`_.\n"))
	if want := `<target id="important-term" name="important term">`; !strings.Contains(got, want) {
		t.Errorf("the inline target lost its name to its own reference:\n%s", got)
	}
	if strings.Contains(got, "Duplicate") {
		t.Errorf("a reference was wrongly treated as a duplicate target claim:\n%s", got)
	}
}
