package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCodeDirectiveOptionValidation pins CodeBlock's three-entry
// option_spec (class, name, number-lines). Anything else is an
// "unknown option" ERROR and NO <literal_block> is produced -- sphinx's
// own :caption: and :emphasize-lines: are what real documents carry, and
// this parser was silently ignoring them. 17 real-world files.
//
// The messages name the directive AS WRITTEN: ".. code-block::" says
// "code-block", not "code". The same applies to the missing-content
// error, which said "code" whatever the spelling.
//
// Expectations come from a bare Parser().parse() run with the corpus
// judge's own settings (report_level=5). That matters here: a valid code
// block ALSO raises "Cannot analyze code. Pygments package not found.",
// but at level 2 it is suppressed and the <literal_block> still lands,
// while the unknown-option ERROR is level 3 and survives.
func TestCodeDirectiveOptionValidation(t *testing.T) {
	cases := []struct {
		name, source string
		wantErr      []string
		wantBlock    bool
	}{
		{
			"an unknown option is an error, and no block is produced",
			".. code-block:: ruby\n   :caption: cap\n\n   x = 1\n",
			[]string{`Error in "code-block" directive:`, `unknown option: "caption".`}, false,
		},
		{
			"the message names the spelling used",
			".. sourcecode:: ruby\n   :caption: cap\n\n   x = 1\n",
			[]string{`Error in "sourcecode" directive:`, `unknown option: "caption".`}, false,
		},
		{
			"missing content also names the spelling used",
			".. code-block:: ruby\n\ntext\n",
			[]string{`Content block expected for the "code-block" directive; none found.`}, false,
		},
		{"a known option is accepted", ".. code:: ruby\n   :class: c\n\n   x = 1\n", nil, true},
		{"number-lines is accepted", ".. code:: ruby\n   :number-lines:\n\n   x = 1\n", nil, true},
		{"no options at all", ".. code:: ruby\n\n   x = 1\n", nil, true},
		{
			// The control that cost two real-world files when it was
			// missing: a code block's CONTENT may itself look like a
			// field marker, and scanning past the option block turned
			// sphinx's own ":no-search:" example into a rejected option.
			"a field-marker-shaped line in the CONTENT is not an option",
			".. code-block:: rst\n\n   :no-search:\n", nil, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			// Checked line by line: the dump indents each line of a
			// message's paragraph, so the two sentences are not
			// contiguous in it.
			for _, want := range tc.wantErr {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q:\n%s", want, got)
				}
			}
			if hasBlock := strings.Contains(got, `<literal_block class="code`); hasBlock != tc.wantBlock {
				t.Errorf("literal_block = %v, want %v:\n%s", hasBlock, tc.wantBlock, got)
			}
		})
	}
}

// TestCodeClassOptionAppends pins that :class: adds to the "code
// <language>" classes rather than replacing them.
func TestCodeClassOptionAppends(t *testing.T) {
	got := doctree.Dump(Parse(".. code:: ruby\n   :class: c\n\n   x = 1\n"))
	if want := `<literal_block class="code ruby c">`; !strings.Contains(got, want) {
		t.Errorf("missing %s:\n%s", want, got)
	}
}
