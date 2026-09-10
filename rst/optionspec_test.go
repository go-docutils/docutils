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

// TestUnwiredDirectiveIsNotValidated pins the opt-in: a directive listed
// in the table but not wired to the validator keeps its previous
// behaviour, silently ignoring an option it does not know. Stating it
// prevents the table from being read as a promise the callers do not
// yet keep.
func TestUnwiredDirectiveIsNotValidated(t *testing.T) {
	got := doctree.Dump(Parse(".. note::\n   :bogus: x\n\n   body\n"))
	if strings.Contains(got, "unknown option") {
		t.Errorf("note is not wired to the validator yet, but rejected an option:\n%s", got)
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
