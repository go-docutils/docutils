package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestUnimplementedRoles covers the registry entries docutils binds to
// roles.unimplemented_role. Each is a KNOWN role name — the lookup finds
// it — and each then raises 'Interpreted text role "..." not
// implemented.', so the construct becomes a <problematic>.
//
// This package knew exactly ONE of the eleven and rendered the rest as a
// bare <inline role="...">, silently accepting “ :index:`word` “ —
// which three sphinx corpus files write. The alias rows are the ones a
// list of canonical names alone would have missed: the English module
// maps "i" onto index and "uri"/"url" onto uri-reference, and the message
// quotes the name AS WRITTEN, so ":i:`x`" says "i".
func TestUnimplementedRoles(t *testing.T) {
	for _, name := range []string{
		"anonymous-reference", "citation-reference", "footnote-reference",
		"i", "index", "named-reference", "substitution-reference",
		"target", "uri", "uri-reference", "url",
	} {
		t.Run(name, func(t *testing.T) {
			src := "See :" + name + ":`word` here.\n"
			got := doctree.Dump(Parse(src))
			want := `Interpreted text role "` + name + `" not implemented.`
			if !strings.Contains(got, want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain %q", src, got, want)
			}
			if !strings.Contains(got, "<problematic") {
				t.Errorf("Parse(%q) kept the role instead of replacing it:\n%s", src, got)
			}
			// The lookup SUCCEEDS for these, so there is no
			// "No role entry" INFO in front of the error.
			if strings.Contains(got, "No role entry") {
				t.Errorf("Parse(%q) reported a lookup miss for a registered role:\n%s", src, got)
			}
		})
	}
}

// TestUnimplementedRoleThatIsNotInTheLanguageModule is the twelfth name
// and the one exception: "restructuredtext-unimplemented-role" is in the
// registry but NOT in the English module's map, so its language lookup
// misses first and the INFO pair comes with the error. That asymmetry is
// why the set cannot be derived from the registry alone.
func TestUnimplementedRoleThatIsNotInTheLanguageModule(t *testing.T) {
	got := doctree.Dump(Parse("See :restructuredtext-unimplemented-role:`word` here.\n"))
	for _, want := range []string{
		`No role entry for "restructuredtext-unimplemented-role" in module`,
		`Interpreted text role "restructuredtext-unimplemented-role" not implemented.`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestImplementedRolesAreUntouched is the CONTROL: every OTHER registry
// entry still renders its node. Without it, a change that rejected every
// role would pass the tests above.
func TestImplementedRolesAreUntouched(t *testing.T) {
	cases := []struct{ source, want string }{
		{"See :emphasis:`x`.\n", "<emphasis>"},
		{"See :strong:`x`.\n", "<strong>"},
		{"See :literal:`x`.\n", "<literal>"},
		{"See :subscript:`x`.\n", "<subscript>"},
		{"See :sub:`x`.\n", "<subscript>"},
		{"See :superscript:`x`.\n", "<superscript>"},
		{"See :title-reference:`x`.\n", "<title_reference>"},
		{"See :t:`x`.\n", "<title_reference>"},
		{"See :abbreviation:`x`.\n", "<abbreviation>"},
		{"See :acronym:`x`.\n", "<acronym>"},
		{"See :math:`x`.\n", "<math>"},
		{"See :code:`x`.\n", "<literal"},
		{"See :pep:`8`.\n", "<reference"},
		{"See :rfc:`2822`.\n", "<reference"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain %q", tc.source, got, tc.want)
			}
			if strings.Contains(got, "not implemented") {
				t.Errorf("Parse(%q) rejected an implemented role:\n%s", tc.source, got)
			}
		})
	}
}

// TestCSVCellIsABlockOfLines covers a quoted csv-table cell that spans
// several source lines. docutils hands the cell to nested_parse as
// "cell.splitlines()" — a BLOCK — and this package passed the whole value
// as ONE line holding its own newlines, so a cell whose opening quote sits
// alone on the line (PEP 578 writes several) began with an empty line
// inside the paragraph.
func TestCSVCellIsABlockOfLines(t *testing.T) {
	const src = ".. csv-table::\n\n   ``a``, \"\n   Detect dynamic code compilation, where ``code``\n   could be a string.\n   \"\n"
	got := doctree.Dump(Parse(src))
	want := "<paragraph>\n                            Detect dynamic code compilation, where "
	if !strings.Contains(got, want) {
		t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%q", src, got, want)
	}
}
