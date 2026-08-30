package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestRawDirective covers the one directive this parser gives real
// semantics: ".. raw:: FORMAT" (Options.RawEnabled, true by default,
// matching real docutils' own --raw-enabled default despite its
// --no-raw flag's "default: True" quirk in optparse terms — the help
// text says plainly "Enable the raw directive. (default)").
func TestRawDirective(t *testing.T) {
	cases := []struct {
		name   string
		opts   Options
		source string
		want   string
	}{
		{
			"enabled by default",
			DefaultOptions(),
			".. raw:: html\n\n   <p>hello</p>\n",
			"<document>\n    <raw format=\"html\">\n        <p>hello</p>\n",
		},
		{
			"a format list may name several writers, space-separated",
			DefaultOptions(),
			".. raw:: html latex\n\n   content\n",
			"<document>\n    <raw format=\"html latex\">\n        content\n",
		},
		{
			"format is lowercased and whitespace-normalized",
			DefaultOptions(),
			".. raw::   HTML   LaTeX  \n\n   content\n",
			"<document>\n    <raw format=\"html latex\">\n        content\n",
		},
		{
			"disabled falls back to the same structural capture any other unimplemented directive gets",
			Options{RawEnabled: false},
			".. raw:: html\n\n   <p>hello</p>\n",
			"<document>\n    <directive arguments=\"html\" name=\"raw\">\n        <p>hello</p>\n",
		},
		{
			"no format argument at all is never treated as raw, regardless of RawEnabled",
			DefaultOptions(),
			".. raw::\n\n   <p>hello</p>\n",
			"<document>\n    <directive name=\"raw\">\n        <p>hello</p>\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(ParseWithOptions(tc.source, tc.opts))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("ParseWithOptions(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

func TestParseUsesDefaultOptions(t *testing.T) {
	got := doctree.Dump(Parse(".. raw:: html\n\n   <p>hi</p>\n"))
	if !strings.Contains(got, "<raw") {
		t.Errorf("Parse did not use DefaultOptions (RawEnabled=true):\n%s", got)
	}
}
