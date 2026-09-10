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
