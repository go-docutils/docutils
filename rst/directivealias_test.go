package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCodeDirectiveAliases pins the two names docutils' English language
// module maps onto "code" (languages/en.py: code-block, sourcecode).
// That module is a layer this project had no equivalent of: a written
// directive name is resolved THERE first, and only the canonical name it
// returns reaches the registry -- which is exactly what the
// lookup-failure INFO means when it names the module by path.
//
// The docutils testsuite corpus never writes ".. code-block::", so it is
// blind to this; 81 real-world files turned on it.
func TestCodeDirectiveAliases(t *testing.T) {
	for _, name := range []string{"code", "code-block", "sourcecode"} {
		t.Run(name, func(t *testing.T) {
			got := doctree.Dump(Parse(".. " + name + ":: python\n\n   x = 1\n"))
			if want := `<literal_block class="code python">`; !strings.Contains(got, want) {
				t.Errorf("missing %s:\n%s", want, got)
			}
			if strings.Contains(got, "Unknown directive type") {
				t.Errorf("a REGISTERED alias was reported as unknown:\n%s", got)
			}
		})
	}
	// The control: a name that is NOT an alias is still unknown, so this
	// is a two-entry table rather than a rule about hyphenated names.
	got := doctree.Dump(Parse(".. code-blocks:: python\n\n   x = 1\n"))
	if !strings.Contains(got, `Unknown directive type "code-blocks"`) {
		t.Errorf("an unrelated name was accepted as an alias:\n%s", got)
	}
}

// TestClassAliasKeepsItsWrittenName guards the reason "rst-class" and
// "section-numbering" are deliberately NOT routed through the same
// helper: runClassDirective distinguishes "class" from its alias by the
// name as WRITTEN, and canonicalizing would erase that.
func TestClassAliasKeepsItsWrittenName(t *testing.T) {
	plain := doctree.Dump(Parse(".. class:: c1\n"))
	alias := doctree.Dump(Parse(".. rst-class:: c1\n"))
	if plain == alias {
		t.Errorf("class and rst-class produced identical output; the written name was lost:\n%s", plain)
	}
}
