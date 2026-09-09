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
// That INCLUDES the `line` attribute, which three earlier attempts got
// wrong (v0.85.0). These messages do not carry the line the duplicate
// sits on: set_duplicate_name hands Reporter.system_message a base_node
// that is not yet attached and has no line of its own, so it falls back
// to the STATE MACHINE's position. For a top-level paragraph that is
// max(firstLine+1, lastLine) -- Text.text() steps at least one line past
// the first looking for a continuation, then stops on the last line it
// actually read. Hence line 4 for a one-line paragraph on line 3, which
// is what the reference gives and what these expectations now say.
//
// A duplicate inside a NESTED block still differs: the enclosing block's
// own extent clamps the value there, and that extent is not threaded
// through the recursion. See parser.currentSMLine.
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
			"<document>\n    <target dupname=\"dup\" id=\"dup\" refuri=\"http://a\">\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Duplicate explicit target name: \"dup\".\n    <target dupname=\"dup\" id=\"dup-1\" refuri=\"http://b\">\n    <paragraph>\n        text\n",
		},
		{
			// implicit over explicit: the EXPLICIT one wins and keeps its
			// name; only the newcomer is invalidated.
			"an implicit target loses to an explicit one already holding the name",
			"_`dup` here.\n\n`<dup>`_ there.\n",
			"<document>\n    <paragraph>\n        <target id=\"dup\" name=\"dup\">\n            dup\n         here.\n    <system_message level=\"1\" line=\"4\" type=\"INFO\">\n        <paragraph>\n            Duplicate implicit target name: \"dup\".\n    <paragraph>\n        <reference name=\"dup\" refuri=\"dup\">\n            dup\n        <target dupname=\"dup\" id=\"dup-1\" refuri=\"dup\">\n         there.\n",
		},
		{
			// explicit over implicit: the explicit one OVERRIDES, taking
			// the name from the implicit one rather than colliding.
			"an explicit target overrides an implicit one already holding the name",
			"`<dup>`_ there.\n\n_`dup` here.\n",
			"<document>\n    <paragraph>\n        <reference name=\"dup\" refuri=\"dup\">\n            dup\n        <target dupname=\"dup\" id=\"dup\" refuri=\"dup\">\n         there.\n    <system_message backref=\"dup-1\" level=\"1\" line=\"4\" type=\"INFO\">\n        <paragraph>\n            Target name overrides implicit target name \"dup\".\n    <paragraph>\n        <target id=\"dup-1\" name=\"dup\">\n            dup\n         here.\n",
		},
		{
			// The case checked AHEAD of the table: same destination, so
			// only the newcomer is invalidated and the message is about an
			// external target rather than a duplicate name.
			"two targets naming the same URI keep the first and report an external duplicate",
			".. _dup: http://a\n.. _dup: http://a\n\ntext\n",
			"<document>\n    <target id=\"dup\" name=\"dup\" refuri=\"http://a\">\n    <system_message level=\"1\" line=\"2\" type=\"INFO\">\n        <paragraph>\n            Duplicate name \"dup\" for external target \"http://a\".\n    <target dupname=\"dup\" id=\"dup-1\" refuri=\"http://a\">\n    <paragraph>\n        text\n",
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

// TestDuplicateSubstitutionDefinition covers the one duplicate rule that
// runs BACKWARDS compared with every other: note_substitution_def keeps
// only the LAST definition, so the OLD node is the one invalidated to
// dupname and the new one keeps its name. The ERROR lands between them.
// Byte-for-byte the reference's own output.
func TestDuplicateSubstitutionDefinition(t *testing.T) {
	got := doctree.Dump(Parse("x\n\n.. |s| image:: a.png\n.. |s| image:: b.png\n"))
	want := "<document>\n    <paragraph>\n        x\n    <substitution_definition dupname=\"s\">\n        <image alt=\"s\" uri=\"a.png\">\n    <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n        <paragraph>\n            Duplicate substitution definition name: \"s\".\n    <substitution_definition name=\"s\">\n        <image alt=\"s\" uri=\"b.png\">\n"
	if got != want {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
	}
}

// TestShortAdornmentNeedsATitleAttempt pins the guard on the
// "Possible incomplete section title" INFO: a short uniform line is only
// a title ATTEMPT when something follows it. Standing alone before a
// blank line it is simply a paragraph, and docutils says nothing --
// reported regardless, this fired on every stray "---" in a document.
func TestShortAdornmentNeedsATitleAttempt(t *testing.T) {
	if got := doctree.Dump(Parse("Short marker.\n\n---\n\nParagraph\n")); strings.Contains(got, "system_message") {
		t.Errorf("a lone short marker drew a diagnostic:\n%s", got)
	}
	if got := doctree.Dump(Parse("Short marker.\n\n---\nTitle\n---\n\nx\n")); !strings.Contains(got, "Possible incomplete section title") {
		t.Errorf("a genuine short-overline title attempt drew none:\n%s", got)
	}
}

// TestDuplicateFootnoteNamePlacement pins where the message goes when the
// duplicate is a FOOTNOTE. It is a child of the footnote itself, ahead of
// the footnote's own content -- not a sibling.
//
// That follows from the same "what was attached when the name was
// registered" reasoning as every other placement here: a footnote can
// hold body elements, so it IS the msgnode, and its body has not been
// built yet. A section differs only because its <title> HAS been.
func TestDuplicateFootnoteNamePlacement(t *testing.T) {
	got := doctree.Dump(Parse(".. [#five] One.\n.. [#five] Two.\n"))
	want := "<document>\n    <footnote auto=\"1\" dupname=\"five\" id=\"five\">\n        <paragraph>\n            One.\n    <footnote auto=\"1\" dupname=\"five\" id=\"five-1\">\n        <system_message backref=\"five-1\" level=\"2\" line=\"2\" type=\"WARNING\">\n            <paragraph>\n                Duplicate explicit target name: \"five\".\n        <paragraph>\n            Two.\n"
	if got != want {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
	}
}
