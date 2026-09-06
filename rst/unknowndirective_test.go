package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestUnknownDirectiveDiagnostics covers docutils' pair for a directive
// name it cannot resolve: an INFO from directives.directive() reporting
// the lookup failure, then an ERROR from Body.unknown_directive carrying
// the directive's WHOLE source as a <literal_block> -- marker line and
// original indentation included, since it uses
// get_first_known_indented(0, strip_indent=False).
//
// Both live in the PARSER, not in a transform, which is why
// Options.ReportUnknownDirectives defaults TRUE while
// Options.ReportDanglingReferences defaults false.
func TestUnknownDirectiveDiagnostics(t *testing.T) {
	// A German admonition name is the corpus's own example, and it is NOT
	// a localization gap: docutils' default language IS English, so
	// ".. Achtung::" is simply unknown to it too.
	got := doctree.Dump(Parse(".. Achtung:: Directives at large.\n"))
	want := "<document>\n    <system_message level=\"1\" line=\"1\" type=\"INFO\">\n        <paragraph>\n            No directive entry for \"Achtung\" in module \"docutils.parsers.rst.languages.en\".\n            Trying \"Achtung\" as canonical directive name.\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Unknown directive type \"Achtung\".\n        <literal_block>\n            .. Achtung:: Directives at large.\n"
	if got != want {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
	}
}

// TestUnknownDirectiveCanBeSuppressed covers the opt-out: a consumer that
// would rather keep an unrecognized directive's content -- a Sphinx
// ".. toctree::" in a document this package was never told about -- gets
// the structural capture back.
func TestUnknownDirectiveCanBeSuppressed(t *testing.T) {
	opts := DefaultOptions()
	opts.ReportUnknownDirectives = false
	got := doctree.Dump(ParseWithOptions(".. toctree::\n\n   intro\n", opts))
	if strings.Contains(got, "Unknown directive type") {
		t.Errorf("the diagnostic was emitted despite the opt-out:\n%s", got)
	}
	if !strings.Contains(got, `<directive name="toctree">`) || !strings.Contains(got, "intro") {
		t.Errorf("the structural capture did not come back:\n%s", got)
	}
}

// TestImplementedDirectiveIsNeverUnknown guards the distinction that a
// first version of this got wrong. Reaching the generic fallback with an
// IMPLEMENTED name means the directive failed its own validation --
// ".. raw::" with no format argument -- and docutils reports that as
// `Error in "raw" directive: ...`, never as an unknown TYPE. This package
// has no per-directive argument validation, so such an invocation still
// falls back to the structural capture; what it must not do is claim the
// name is unknown.
func TestImplementedDirectiveIsNeverUnknown(t *testing.T) {
	got := doctree.Dump(Parse(".. raw::\n\n   <p>hello</p>\n"))
	if strings.Contains(got, "Unknown directive type") {
		t.Errorf(`"raw" is implemented; a failed invocation of it is not an unknown TYPE:`+"\n%s", got)
	}
}
