package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestContainerDirective covers docutils.parsers.rst.directives.body.Container
// (read directly): ".. container:: [class ...]" wraps its content in a
// <container class="...">, the argument (optionally spanning multiple
// lines, joined with a space) becoming the node's own classes, not a
// :class: option (Container's own option_spec only recognizes :name:).
// The last case (a :name: option referenced by a same-document
// "`my name`_") deliberately does NOT assert refuri: real docutils'
// bare Parser().parse() never resolves a hyperlink reference to a
// refuri at parse time (that's a later transform) — this project
// resolves eagerly, the same documented confound already affecting
// resolveTargets elsewhere, so the reference DOES carry refuri="#my-name"
// here even though the corpus's own bare-parse ground truth doesn't.
// What this test actually guards is that the reference resolves at all
// (a <reference>, not a dangling <problematic>) — see collectTargets'
// own doc comment for the real bug this fixed: a directive's own
// :name: option didn't register as an implicit hyperlink target at all.
func TestContainerDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare container with two paragraphs",
			".. container::\n\n   \"container\" is a generic element, an extension mechanism for\n   users & applications.\n\n   Containers may contain arbitrary body elements.\n",
			"<document>\n    <container>\n        <paragraph>\n            \"container\" is a generic element, an extension mechanism for\n            users & applications.\n        <paragraph>\n            Containers may contain arbitrary body elements.\n",
		},
		{
			"a single-word argument becomes the class",
			".. container:: custom\n\n   Some text.\n",
			"<document>\n    <container class=\"custom\">\n        <paragraph>\n            Some text.\n",
		},
		{
			"a multi-line, multi-word argument joins with spaces",
			".. container:: one two three\n   four\n\n   Multiple classes.\n\n   Multi-line argument.\n\n   Multiple paragraphs in the container.\n",
			"<document>\n    <container class=\"one two three four\">\n        <paragraph>\n            Multiple classes.\n        <paragraph>\n            Multi-line argument.\n        <paragraph>\n            Multiple paragraphs in the container.\n",
		},
		{
			"no content at all is an error, with the raw source attached",
			".. container::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"container\" directive; none found.\n        <literal_block>\n            .. container::\n",
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

// TestContainerNameRegistersAsTarget is the isolated regression test for
// the collectTargets fix: a directive's own :name: option (verified
// directly against the foreign judge to apply generically — note, table,
// code, container all behave the same way, not just container, which is
// what surfaced this) must register as a resolvable same-document
// hyperlink target, the same as an explicit ".. _name:" target or a
// section title already did. Before the fix, collectTargets only
// examined <target> and <section> elements, so a reference to any OTHER
// directive's :name: was wrongly treated as dangling and rewritten into
// a <problematic> node with a spurious system_message.
func TestContainerNameRegistersAsTarget(t *testing.T) {
	src := ".. container::\n   :name: my name\n\n   The name argument allows hyperlinks to `my name`_.\n"
	got := doctree.Dump(Parse(src))
	if strings.Contains(got, "problematic") {
		t.Errorf("Parse(%q) dump still contains a dangling <problematic> reference:\n%s", src, got)
	}
	if !strings.Contains(got, `<reference name="my name" refname="my name" refuri="#my-name">`) {
		t.Errorf("Parse(%q) dump doesn't show the reference resolved against the container's own :name:, got:\n%s", src, got)
	}
}
