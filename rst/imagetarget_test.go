package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestImageTargetOption covers the ":target:" option of the image and
// figure directives (docutils.parsers.rst.directives.images, Image.run's
// own branch, read directly) — the option this file's own SCOPE note used
// to say no corpus case exercised. Seven real-world files did: every
// project README puts its build badges behind one, and a badge whose link
// is dropped is a CONTENT loss, not a cosmetic difference.
//
// Every expectation below was taken from real docutils 0.23 run with the
// corpus judge's own settings (a bare parse, no transforms), not derived
// by reading run() — which is how the "no adjust_uri" detail was found:
// an email address in a ":target:" stays bare, while the same address as
// an inline embedded URI gains a "mailto:".
//
// The CONTROLS are the cases that must NOT become a reference: a URI
// ending in "_" (".../foo_"), a value with a space before the underscore
// ("some name_" without backquotes), and the double-underscore anonymous
// form, which parse_target's own pattern refuses.
func TestImageTargetOption(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a URI target wraps the image in a reference",
			".. image:: a.png\n   :target: https://example.org/x\n",
			"<document>\n    <reference refuri=\"https://example.org/x\">\n        <image uri=\"a.png\">\n",
		},
		{
			"a simplename target is an indirect reference, resolved later like any other",
			".. image:: a.png\n   :target: somename_\n\n.. _somename: https://e.org/\n",
			"<document>\n    <reference name=\"somename\" refname=\"somename\">\n        <image uri=\"a.png\">\n    <target id=\"somename\" name=\"somename\" refuri=\"https://e.org/\">\n",
		},
		{
			"a backquoted phrase target keeps its DISPLAY capitalisation in name, normalized in refname",
			".. image:: a.png\n   :target: `Some Name`_\n",
			"<document>\n    <reference name=\"Some Name\" refname=\"some name\">\n        <image uri=\"a.png\">\n",
		},
		{
			"a phrase target split across lines is one name, not one word",
			".. image:: a.png\n   :target: `some\n      name`_\n",
			"<document>\n    <reference name=\"some name\" refname=\"some name\">\n        <image uri=\"a.png\">\n",
		},
		{
			"an escaped space inside a phrase target disappears, as unescape leaves it",
			".. image:: a.png\n   :target: `a\\ b`_\n",
			"<document>\n    <reference name=\"ab\" refname=\"ab\">\n        <image uri=\"a.png\">\n",
		},
		{
			// CONTROL: "/" is not a simplename character, so a URI that
			// happens to end in "_" stays a URI.
			"a URI ending in an underscore is still a URI",
			".. image:: a.png\n   :target: https://e.org/foo_\n",
			"<document>\n    <reference refuri=\"https://e.org/foo_\">\n        <image uri=\"a.png\">\n",
		},
		{
			// CONTROL: without backquotes the space breaks the
			// simplename, so this is a URI — with its whitespace removed,
			// like any other target value.
			"an unquoted two-word value ending in an underscore is a URI, not a reference",
			".. image:: a.png\n   :target: some name_\n",
			"<document>\n    <reference refuri=\"somename_\">\n        <image uri=\"a.png\">\n",
		},
		{
			// CONTROL: Body.patterns.reference stops at ONE trailing
			// underscore, so the anonymous form never reaches a refname
			// — which is also why a ":target:" can never produce the one
			// kind of reference a substitution definition refuses.
			"the anonymous double-underscore form is a URI too",
			".. image:: a.png\n   :target: nosuch__\n",
			"<document>\n    <reference refuri=\"nosuch__\">\n        <image uri=\"a.png\">\n",
		},
		{
			// CONTROL: adjust_uri is NOT applied here, unlike an inline
			// embedded URI — run() hands parse_target's own data straight
			// to nodes.reference.
			"an email address in a target gets no mailto: prefix",
			".. image:: a.png\n   :target: user@example.org\n",
			"<document>\n    <reference refuri=\"user@example.org\">\n        <image uri=\"a.png\">\n",
		},
		{
			"a target wrapped across lines closes up with no space",
			".. image:: a.png\n   :target: https://e.org/very/\n      long/path\n",
			"<document>\n    <reference refuri=\"https://e.org/very/long/path\">\n        <image uri=\"a.png\">\n",
		},
		{
			"an escaped space in a URI target becomes exactly one space",
			".. image:: a.png\n   :target: https://e.org/a\\ b\n",
			"<document>\n    <reference refuri=\"https://e.org/a b\">\n        <image uri=\"a.png\">\n",
		},
		{
			// add_name applies to the IMAGE, not to the wrapper: the id
			// and name stay on the inner node.
			"a :name: beside a :target: stays on the image",
			".. image:: a.png\n   :name: fig one\n   :target: https://e.org/\n",
			"<document>\n    <reference refuri=\"https://e.org/\">\n        <image id=\"fig-one\" name=\"fig one\" uri=\"a.png\">\n",
		},
		{
			// unchanged_required raises at option-ASSEMBLY time, before
			// run() — so there is no image at all, only the error.
			"a :target: with no value is an error and the image is gone",
			".. image:: a.png\n   :target:\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"image\" directive:\n            invalid option value: (option: \"target\"; value: None)\n            argument required but none supplied.\n        <literal_block>\n            .. image:: a.png\n               :target:\n",
		},
		{
			"a figure holds the reference, and the reference holds the image",
			".. figure:: a.png\n   :target: https://e.org/\n\n   The caption.\n",
			"<document>\n    <figure>\n        <reference refuri=\"https://e.org/\">\n            <image uri=\"a.png\">\n        <caption>\n            The caption.\n",
		},
		{
			// The wrapper reaches a substitution definition too, and is
			// allowed there: it carries neither names nor ids.
			"an image with a target inside a substitution definition keeps both",
			"|s|\n\n.. |s| image:: a.png\n   :target: https://e.org/\n",
			"<document>\n    <paragraph>\n        <substitution_reference refname=\"s\">\n            s\n    <substitution_definition name=\"s\">\n        <reference refuri=\"https://e.org/\">\n            <image alt=\"s\" uri=\"a.png\">\n",
		},
		{
			// The SIBLING call site: ".. _name: value" runs through the
			// same parse_target, so the phrase form has to work there too
			// — it did not, and pep-0534 is the corpus file that says so.
			"a hyperlink target whose value is a backquoted phrase is indirect",
			".. _a: `some name`_\n",
			"<document>\n    <target id=\"a\" name=\"a\" refname=\"some name\">\n",
		},
		{
			// And the body lines of a target are joined with a SPACE, as
			// parse_target joins its block — concatenating them with
			// nothing made this one word.
			"a hyperlink target's phrase value split across lines is one name",
			".. _a: `some\n   name`_\n",
			"<document>\n    <target id=\"a\" name=\"a\" refname=\"some name\">\n",
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
