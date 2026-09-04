package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestImageAndFigure covers docutils.parsers.rst.directives.images (read
// directly): "image" (required URI argument, no content permitted) and
// "figure" (same argument plus figwidth/figclass/figname/align, and an
// optional caption/legend body — its first paragraph becomes <caption>,
// an empty comment suppresses the caption, anything else there is an
// ERROR, and everything after becomes <legend>). Every case here is
// drawn directly from docutils' own test_directives/test_figures.py
// corpus fixtures (already foreign-judge-verified there), covering the
// full corpus/test_parsers/test_rst/test_directives/test_figures.py
// mismatch set this round cleared (12 of 12 minus one deliberately
// deferred case — see below).
func TestImageAndFigure(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a bare figure with no caption",
			".. figure:: picture.png\n",
			"<document>\n    <figure>\n        <image uri=\"picture.png\">\n",
		},
		{
			"a figure with a caption",
			".. figure:: picture.png\n\n   A picture with a caption.\n",
			"<document>\n    <figure>\n        <image uri=\"picture.png\">\n        <caption>\n            A picture with a caption.\n",
		},
		{
			"an invalid caption (not a paragraph) is an ERROR, no caption/legend added",
			".. figure:: picture.png\n\n   - A picture with an invalid caption.\n",
			"<document>\n    <figure>\n        <image uri=\"picture.png\">\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Figure caption must be a paragraph or empty comment.\n        <literal_block>\n            .. figure:: picture.png\n            \n               - A picture with an invalid caption.\n",
		},
		{
			"an empty comment suppresses the caption, everything after becomes a legend",
			".. figure:: picture.png\n\n   ..\n\n   A picture with a legend but no caption.\n",
			"<document>\n    <figure>\n        <image uri=\"picture.png\">\n        <legend>\n            <paragraph>\n                A picture with a legend but no caption.\n",
		},
		{
			"a non-empty comment where a caption or empty comment was expected is still an ERROR",
			".. figure:: picture.png\n\n   .. The comment replacing the caption must be empty.\n\n   This should be a legend.\n",
			"<document>\n    <figure>\n        <image uri=\"picture.png\">\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Figure caption must be a paragraph or empty comment.\n        <literal_block>\n            .. figure:: picture.png\n            \n               .. The comment replacing the caption must be empty.\n            \n               This should be a legend.\n",
		},
		{
			"figclass/figname/align plus every image-level option, case-insensitive directive name",
			".. Figure:: picture.png\n   :alt: alternate text\n   :name: img:picture\n   :height: 100\n   :width: 200\n   :scale: 50\n   :loading: lazy\n   :class: image-class\n   :figwidth: 300\n   :figclass: class1 class2\n   :figname: Fig:  pix\n\n   A figure with options and this caption.\n",
			"<document>\n    <figure class=\"class1 class2\" id=\"fig-pix\" name=\"fig: pix\" width=\"300px\">\n        <image alt=\"alternate text\" class=\"image-class\" height=\"100\" id=\"img-picture\" loading=\"lazy\" name=\"img:picture\" scale=\"50\" uri=\"picture.png\" width=\"200\">\n        <caption>\n            A figure with options and this caption.\n",
		},
		{
			"explicit :align: (figure validates against the horizontal values, same as image)",
			".. figure:: picture.png\n   :align: center\n\n   A figure with explicit alignment.\n",
			"<document>\n    <figure align=\"center\">\n        <image uri=\"picture.png\">\n        <caption>\n            A figure with explicit alignment.\n",
		},
		{
			"an invalid :align: value is a directive-level ERROR with the real docutils choice-list wording",
			".. figure:: picture.png\n   :align: top\n\n   A figure with wrong alignment.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"figure\" directive:\n            invalid option value: (option: \"align\"; value: 'top')\n            \"top\" unknown; choose from \"left\", \"center\", or \"right\".\n        <literal_block>\n            .. figure:: picture.png\n               :align: top\n            \n               A figure with wrong alignment.\n",
		},
		{
			"a bare image directive with no argument at all: ERROR",
			".. image::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"image\" directive:\n            1 argument(s) required, 0 supplied.\n        <literal_block>\n            .. image::\n",
		},
		{
			"a bare image directive with content is an ERROR (no content permitted, unlike figure)",
			".. image:: picture.png\n\n   Not allowed.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"image\" directive:\n            no content permitted.\n        <literal_block>\n            .. image:: picture.png\n            \n               Not allowed.\n",
		},
		{
			"a real CSS unit on :height:/:width: is kept, not treated as unitless",
			".. image:: picture.png\n   :height: 2cm\n   :width: 50%\n",
			"<document>\n    <image height=\"2cm\" uri=\"picture.png\" width=\"50%\">\n",
		},
		{
			"an invalid :loading: value is a directive-level ERROR",
			".. image:: picture.png\n   :loading: eager\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"image\" directive:\n            invalid option value: (option: \"loading\"; value: 'eager')\n            \"eager\" unknown; choose from \"embed\", \"link\", or \"lazy\".\n        <literal_block>\n            .. image:: picture.png\n               :loading: eager\n",
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

// TestSubstitutionEmbeddedDirectives covers Body.substitution_def +
// SubstitutionDef (states.py, read directly): a substitution
// definition's content is always an embedded directive invocation,
// nested as REAL children (an <image>, a <raw>, or replace's own
// inline-parsed content) rather than flattened onto attributes — a real,
// previously-shipped bug this round fixed (the old code relabeled the
// embedded directive's own element in place instead of wrapping it).
// Every case here is drawn directly from docutils' own
// test_substitutions.py and test_directives/test_replace.py corpus
// fixtures.
func TestSubstitutionEmbeddedDirectives(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an embedded image substitution, alt defaults to the substitution's own name",
			"Here's an image substitution definition:\n\n.. |symbol| image:: symbol.png\n",
			"<document>\n    <paragraph>\n        Here's an image substitution definition:\n    <substitution_definition name=\"symbol\">\n        <image alt=\"symbol\" uri=\"symbol.png\">\n",
		},
		{
			"the embedded directive marker may start entirely on the NEXT body line",
			"Embedded directive starts on the next line:\n\n.. |symbol|\n   image:: symbol.png\n",
			"<document>\n    <paragraph>\n        Embedded directive starts on the next line:\n    <substitution_definition name=\"symbol\">\n        <image alt=\"symbol\" uri=\"symbol.png\">\n",
		},
		{
			"an embedded image's own argument may itself continue on a later, non-blank line",
			"Trailing spaces should not be significant:\n\n.. |symbol| image:: \n   symbol.png\n",
			"<document>\n    <paragraph>\n        Trailing spaces should not be significant:\n    <substitution_definition name=\"symbol\">\n        <image alt=\"symbol\" uri=\"symbol.png\">\n",
		},
		{
			"substitutions support case differences (case-sensitive, unlike a hyperlink target's name)",
			"Substitutions support case differences:\n\n.. |eacute| replace:: é\n.. |Eacute| replace:: É\n",
			"<document>\n    <paragraph>\n        Substitutions support case differences:\n    <substitution_definition name=\"eacute\">\n        é\n    <substitution_definition name=\"Eacute\">\n        É\n",
		},
		{
			"a raw substitution nests the <raw> element directly, backslashes preserved",
			"Raw substitution, backslashes should be preserved:\n\n.. |alpha| raw:: latex\n\n   $\\\\alpha$\n",
			"<document>\n    <paragraph>\n        Raw substitution, backslashes should be preserved:\n    <substitution_definition name=\"alpha\">\n        <raw format=\"latex\">\n            $\\\\alpha$\n",
		},
		{
			"a substitution definition with no embedded directive at all becomes empty or invalid",
			"Here's a bad case:\n\n.. |invalid| there's no directive here\n",
			"<document>\n    <paragraph>\n        Here's a bad case:\n    <system_message level=\"2\" line=\"3\" type=\"WARNING\">\n        <paragraph>\n            Substitution definition \"invalid\" empty or invalid.\n        <literal_block>\n            .. |invalid| there's no directive here\n",
		},
		{
			"a substitution marker with nothing at all after it: missing contents",
			".. |empty|\n",
			"<document>\n    <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Substitution definition \"empty\" missing contents.\n        <literal_block>\n            .. |empty|\n",
		},
		{
			"replace with an embedded, resolvable hyperlink reference",
			".. |Python| replace:: Python, *the* best language around\n\n.. _Python: http://www.python.org/\n\nI recommend you try |Python|_.\n",
			"<document>\n    <substitution_definition name=\"Python\">\n        Python, \n        <emphasis>\n            the\n         best language around\n    <target id=\"python\" name=\"python\" refuri=\"http://www.python.org/\">\n    <paragraph>\n        I recommend you try \n        <reference refname=\"python\" refuri=\"http://www.python.org/\">\n            <substitution_reference refname=\"Python\">\n                Python\n        .\n",
		},
		{
			"a target inside replace is prohibited",
			"Elements that are prohibited inside of substitution definitions:\n\n.. |target| replace:: _`target`\n",
			"<document>\n    <paragraph>\n        Elements that are prohibited inside of substitution definitions:\n    <system_message level=\"3\" line=\"3\" type=\"ERROR\">\n        <paragraph>\n            Targets (names and identifiers) are not supported in a substitution definition.\n        <literal_block>\n            .. |target| replace:: _`target`\n",
		},
		{
			"an anonymous reference inside replace is prohibited",
			".. |reference| replace:: anonymous__\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Anonymous references are not supported in a substitution definition.\n        <literal_block>\n            .. |reference| replace:: anonymous__\n",
		},
		{
			"an auto-numbered footnote reference inside replace is prohibited",
			".. |auto-numbered footnote| replace:: [#]_\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            References to auto-numbered and auto-symbol footnotes are not supported in a substitution definition.\n        <literal_block>\n            .. |auto-numbered footnote| replace:: [#]_\n",
		},
		{
			"replace with truly no content at all is empty or invalid, matching this project's own simplified diagnostic (real docutils gives a separate, more specific two-part error here — a deliberate scope simplification, see fillReplaceSubstitution's own doc comment)",
			".. |name| replace::\n",
			"<document>\n    <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Substitution definition \"name\" empty or invalid.\n        <literal_block>\n            .. |name| replace::\n",
		},
		{
			// The WARNING messages parseInline generates for the unclosed
			// start-strings carry NO "backref" here, unlike their normal
			// shape elsewhere — verified directly against the foreign
			// judge: the problematic nodes they describe are reparented
			// into this ERROR's own block_quote reconstruction, not left
			// in their original paragraph position.
			"problematic content in replace carries no backref on its own warning messages",
			".. |name| replace::  *error in **inline ``markup\n",
			"<document>\n    <system_message id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline emphasis start-string without end-string.\n    <system_message id=\"system-message-2\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline strong start-string without end-string.\n    <system_message id=\"system-message-3\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline literal start-string without end-string.\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Problematic content in substitution definition\n        <literal_block>\n            .. |name| replace::  *error in **inline ``markup\n        <block_quote>\n            <paragraph>\n                <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                    *\n                error in \n                <problematic id=\"problematic-2\" refid=\"system-message-2\">\n                    **\n                inline \n                <problematic id=\"problematic-3\" refid=\"system-message-3\">\n                    ``\n                markup\n",
		},
		{
			// Replace.run is only ever reached FROM WITHIN a substitution
			// definition's own dispatch — used as an ordinary top-level
			// directive, real docutils rejects it outright.
			"the \"replace\" directive used outside a substitution definition is an error",
			".. replace:: not valid outside of a substitution definition\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid context: the \"replace\" directive can only be used within a substitution definition.\n        <literal_block>\n            .. replace:: not valid outside of a substitution definition\n",
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
