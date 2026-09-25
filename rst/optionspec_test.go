package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDirectiveOptionSpec covers the SHARED spec table. Option
// validation began as a bespoke loop inside the code directive
// (v0.96.0); math needed the identical rule, so the spec moved into one
// transcribed table and both directives consult it. The table lists
// every directive this package implements, but only the ones wired to
// the validator are enforced -- an entry alone changes nothing, which
// keeps the table safe to complete ahead of its callers.
//
// Every expectation is a bare Parser().parse() at the judge's
// report_level.
func TestDirectiveOptionSpec(t *testing.T) {
	cases := []struct {
		name, source string
		wantErr      []string
		wantNode     string
	}{
		{
			// sphinx's :label: is not in MathBlock's spec.
			"math rejects an option outside its spec",
			".. math::\n   :label: dup\n\n   a^2\n",
			[]string{`Error in "math" directive:`, `unknown option: "label".`}, "",
		},
		{"math accepts :class:", ".. math::\n   :class: c\n\n   a^2\n", nil, `<math_block class="c">`},
		{"math accepts no options", ".. math::\n\n   a^2\n", nil, "<math_block>"},
		{
			// The control the code directive paid two files to learn: a
			// directive's CONTENT may start field-marker-shaped, and the
			// scan must stop where the options do.
			"a field-shaped first content line is not an option",
			".. math::\n\n   :x: not an option\n", nil, "<math_block>",
		},
		{
			"code still rejects sphinx's :caption:",
			".. code-block:: ruby\n   :caption: cap\n\n   x = 1\n",
			[]string{`Error in "code-block" directive:`, `unknown option: "caption".`}, "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			for _, w := range tc.wantErr {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q:\n%s", w, got)
				}
			}
			if tc.wantNode != "" && !strings.Contains(got, tc.wantNode) {
				t.Errorf("missing %s:\n%s", tc.wantNode, got)
			}
			if tc.wantErr == nil && strings.Contains(got, "unknown option") {
				t.Errorf("a valid invocation was rejected:\n%s", got)
			}
		})
	}
}

// TestUnwiredDirectiveIsNotValidated pins what the table still does NOT
// promise. Every directive listed in it is now wired to the validator
// (v0.133.0: the eleven admonition-shaped ones, container, image, figure,
// table, list-table, topic, sidebar, rubric, parsed-literal, line-block,
// joining code and math); a directive ABSENT from it is not validated at
// all, and docutils does reject an unknown option there too. csv-table is
// the case, and this states the divergence rather than leaving the table
// to be read as complete.
func TestUnwiredDirectiveIsNotValidated(t *testing.T) {
	got := doctree.Dump(Parse(".. csv-table:: T\n   :bogus: x\n\n   a, b\n"))
	if strings.Contains(got, "unknown option") {
		t.Errorf("csv-table is absent from the spec table, so it cannot be validated; it rejected an option anyway:\n%s", got)
	}
}

// TestWiredDirectivesRejectAnUnknownOption is the other half: one case per
// wired directive, each the reference's own error. A table entry alone
// changes nothing, so the only way to know a directive is enforced is to
// ask it.
func TestWiredDirectivesRejectAnUnknownOption(t *testing.T) {
	cases := []struct{ name, source, written string }{
		{"note", ".. note::\n   :bogus: x\n\n   body\n", "note"},
		{"hint with sphinx :collapsible:", ".. hint::\n   :collapsible: closed\n\n   body\n", "hint"},
		{"admonition", ".. admonition:: T\n   :bogus: x\n\n   body\n", "admonition"},
		{"compound", ".. compound::\n   :bogus: x\n\n   body\n", "compound"},
		{"container", ".. container:: cls\n   :bogus: x\n\n   body\n", "container"},
		{"image", ".. image:: a.png\n   :bogus: x\n", "image"},
		{"figure", ".. figure:: a.png\n   :bogus: x\n", "figure"},
		{"table", ".. table::\n   :bogus: x\n\n   ==  ==\n   a   b\n   ==  ==\n", "table"},
		{"list-table", ".. list-table:: T\n   :bogus: x\n\n   * - a\n", "list-table"},
		{"topic", ".. topic:: T\n   :bogus: x\n\n   body\n", "topic"},
		{"sidebar", ".. sidebar:: S\n   :bogus: x\n\n   body\n", "sidebar"},
		{"rubric", ".. rubric:: R\n   :bogus: x\n", "rubric"},
		{"parsed-literal", ".. parsed-literal::\n   :bogus: x\n\n   a\n", "parsed-literal"},
		{"line-block", ".. line-block::\n   :bogus: x\n\n   | a\n", "line-block"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			want := `Error in "` + tc.written + `" directive:`
			if !strings.Contains(got, want) || !strings.Contains(got, `unknown option: "`) {
				t.Errorf("want %q and an unknown-option line in:\n%s", want, got)
			}
		})
	}
}

// TestWiredDirectivesStillAcceptTheirOwnOptions is the CONTROL for the
// list above: the same directives, each with an option its own spec
// declares, must be unaffected. Without it, a validator that rejected
// EVERYTHING would pass every case above.
func TestWiredDirectivesStillAcceptTheirOwnOptions(t *testing.T) {
	cases := []struct{ name, source, wantNode string }{
		{"note :class:", ".. note::\n   :class: c\n\n   body\n", `<note class="c">`},
		{"admonition :name:", ".. admonition:: T\n   :name: n\n\n   body\n", `name="n"`},
		{"container :name:", ".. container:: cls\n   :name: n\n\n   body\n", `name="n"`},
		{"image :alt:", ".. image:: a.png\n   :alt: text\n", `alt="text"`},
		{"figure :figclass:", ".. figure:: a.png\n   :figclass: f\n", `class="f"`},
		{"table :widths:", ".. table::\n   :widths: 30,70\n\n   ==  ==\n   a   b\n   ==  ==\n", `colwidths-given`},
		{"list-table :header-rows:", ".. list-table:: T\n   :header-rows: 1\n\n   * - head\n   * - body\n", `<thead>`},
		{"topic :class:", ".. topic:: T\n   :class: c\n\n   body\n", `<topic class="c">`},
		{"sidebar :subtitle:", ".. sidebar:: S\n   :subtitle: sub\n\n   body\n", `<subtitle>`},
		{"rubric :class:", ".. rubric:: R\n   :class: c\n", `<rubric class="c">`},
		{"parsed-literal :class:", ".. parsed-literal::\n   :class: c\n\n   a\n", `<literal_block class="c">`},
		{"line-block :class:", ".. line-block::\n   :class: c\n\n   | a\n", `<line_block class="c">`},
		{
			// The control the code directive paid two real-world files
			// for, re-stated for a directive whose content is free text:
			// a field-marker-shaped first CONTENT line is not an option.
			"a field-shaped first content line is not an option",
			".. note::\n\n   :x: not an option\n", `<field_name>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.Contains(got, "unknown option") {
				t.Errorf("a declared option was rejected:\n%s", got)
			}
			if !strings.Contains(got, tc.wantNode) {
				t.Errorf("want %s in:\n%s", tc.wantNode, got)
			}
		})
	}
}

// TestReportUnknownDirectiveOptions pins BOTH directions of the opt-out.
// A test that only asserted the lenient side would pass on a parser that
// had simply lost the check, so each case is run twice -- once with the
// flag on, once off -- and the two results must DIFFER exactly where the
// option is unknown and agree everywhere else.
//
// The flag exists because faithfulness has a price a renderer pays: 32
// of the 1564 real-world corpus files carry a sphinx-only option on a
// code block or an equation, and every one of them loses the block to an
// error paragraph. go-richdoc/rst sets it false for that reason.
func TestReportUnknownDirectiveOptions(t *testing.T) {
	cases := []struct {
		name, source string
		unknown      bool   // is the option outside the directive's spec?
		lenientNode  string // what the lenient parse must produce instead
	}{
		{"sphinx :caption: on a code block", ".. code:: go\n   :caption: hi\n\n   x := 1\n", true, "<literal_block"},
		{"sphinx :label: on an equation", ".. math::\n   :label: eq\n\n   a^2\n", true, "<math_block"},
		{"an option the directive DOES declare", ".. math::\n   :class: c\n\n   a^2\n", false, `<math_block class="c">`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strict := DefaultOptions()
			if !strict.ReportUnknownDirectiveOptions {
				t.Fatal("ReportUnknownDirectiveOptions must default TRUE: docutils raises this in Body.parse_directive_options, in the PARSER")
			}
			lenient := strict
			lenient.ReportUnknownDirectiveOptions = false

			gotStrict := doctree.Dump(ParseWithOptions(tc.source, strict))
			gotLenient := doctree.Dump(ParseWithOptions(tc.source, lenient))

			if strings.Contains(gotStrict, `unknown option`) != tc.unknown {
				t.Errorf("strict parse: unknown-option error present = %v, want %v\n%s",
					!tc.unknown, tc.unknown, gotStrict)
			}
			if strings.Contains(gotLenient, `unknown option`) {
				t.Errorf("lenient parse must never report an unknown option, got:\n%s", gotLenient)
			}
			if !strings.Contains(gotLenient, tc.lenientNode) {
				t.Errorf("lenient parse: want %s in\n%s", tc.lenientNode, gotLenient)
			}
			// The control: turning the flag off must change NOTHING for a
			// directive whose options are all declared.
			if !tc.unknown && gotStrict != gotLenient {
				t.Errorf("flag changed a parse it has no business touching:\nstrict:\n%s\nlenient:\n%s", gotStrict, gotLenient)
			}
		})
	}
}
