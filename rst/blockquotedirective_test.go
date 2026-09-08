package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestBlockQuoteDirectives covers epigraph, highlights and pull-quote --
// three registry entries that are nothing but "a block quote carrying a
// class". BlockQuote.run calls state.block_quote() on its own content
// and appends its class list, so the content goes through the SAME
// routine a bare indented block quote uses and an attribution line
// behaves identically.
//
// Expectations are from a bare Parser().parse() at the judge's
// report_level.
func TestBlockQuoteDirectives(t *testing.T) {
	for _, name := range []string{"epigraph", "highlights", "pull-quote"} {
		t.Run(name, func(t *testing.T) {
			got := doctree.Dump(Parse(".. " + name + "::\n\n   Quote text.\n"))
			if want := `<block_quote class="` + name + `">`; !strings.Contains(got, want) {
				t.Errorf("missing %s:\n%s", want, got)
			}
			if strings.Contains(got, "Unknown directive type") {
				t.Errorf("a REGISTERED directive was reported as unknown:\n%s", got)
			}
		})
	}
}

// TestBlockQuoteDirectiveAttribution pins the reason these reuse the
// block-quote routine rather than parseBlockLines: an attribution line
// inside one becomes an <attribution>, exactly as in a bare quote.
func TestBlockQuoteDirectiveAttribution(t *testing.T) {
	got := doctree.Dump(Parse(".. epigraph::\n\n   Quote text.\n\n   -- Author\n"))
	if !strings.Contains(got, "<attribution>") {
		t.Errorf("no attribution built:\n%s", got)
	}
	if !strings.Contains(got, `<block_quote class="epigraph">`) {
		t.Errorf("missing the class:\n%s", got)
	}
}

// TestBlockQuoteDirectiveHasNoOptions is the control. These three
// declare NO option_spec at all, so a ":class:" line under one is
// CONTENT and parses as a field list INSIDE the quote -- it does not
// become an attribute. A parser that "helpfully" read it as an option
// would pass every other case here.
func TestBlockQuoteDirectiveHasNoOptions(t *testing.T) {
	got := doctree.Dump(Parse(".. epigraph::\n   :class: extra\n\n   Quote.\n"))
	if !strings.Contains(got, "<field_list>") {
		t.Errorf(":class: was not treated as content:\n%s", got)
	}
	if strings.Contains(got, `class="epigraph extra"`) || strings.Contains(got, `class="extra"`) {
		t.Errorf(":class: was consumed as an option:\n%s", got)
	}
}

// TestBlockQuoteDirectiveNeedsContent: assert_has_content, with the name
// as written.
func TestBlockQuoteDirectiveNeedsContent(t *testing.T) {
	got := doctree.Dump(Parse(".. epigraph::\n\ntext\n"))
	if !strings.Contains(got, `Content block expected for the "epigraph" directive; none found.`) {
		t.Errorf("missing the empty-content error:\n%s", got)
	}
}
