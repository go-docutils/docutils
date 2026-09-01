package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestTableDirectives covers ".. table::" and ".. list-table::"
// (docutils.parsers.rst.directives.tables' Table/RSTTable/ListTable
// classes, read directly): title (the directive's own argument, inline-
// parsed), :class:/:name:/:align:/:width:/:widths: options,
// :header-rows:/:stub-columns: (list-table only), the "exactly one
// table expected" / "none found" / bullet-list-shape / widths-mismatch
// diagnostics, and — a real bug found along the way, not RST-directive-
// specific — a table cell centered or otherwise inset within its own
// fixed-width column no longer gets wrapped in a spurious block_quote
// (dedentCellLines, already used by the grid-table path, now also
// covers the simple-table one). Every case verified against the
// foreign judge (Parser().parse(), the same bare, pre-transform tree
// doctree.Dump produces).
func TestTableDirectives(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"table directive: title, class, name, and a centered header cell (regression: no spurious block_quote)",
			".. table:: Truth table for \"not\"\n   :class: custom\n   :name:  tab:truth.not\n\n   =====  =====\n     A    not A\n   =====  =====\n   False  True\n   True   False\n   =====  =====\n",
			"<document>\n    <table class=\"custom\" id=\"tab-truth-not\" name=\"tab:truth.not\">\n        <title>\n            Truth table for \"not\"\n        <tgroup cols=\"2\">\n            <colspec colwidth=\"5\">\n            <colspec colwidth=\"5\">\n            <thead>\n                <row>\n                    <entry>\n                        <paragraph>\n                            A\n                    <entry>\n                        <paragraph>\n                            not A\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            False\n                    <entry>\n                        <paragraph>\n                            True\n                <row>\n                    <entry>\n                        <paragraph>\n                            True\n                    <entry>\n                        <paragraph>\n                            False\n",
		},
		{
			// A 4-space body indent, no argument on the directive's own
			// line — regression: gatherExplicitBody previously assumed a
			// fixed 3-column dedent (the ".. " prefix width), leaving a
			// stray leading space on every option/content line once the
			// real indent was wider, which broke option recognition
			// entirely.
			"table directive with 4-space body indent: :widths: auto keeps the underlying grid markup's own widths, adds a class",
			".. table::\n    :widths: auto\n\n    +--------------+-------------------+\n    | Columns with | automatic widths. |\n    +--------------+-------------------+\n",
			"<document>\n    <table class=\"colwidths-auto\">\n        <tgroup cols=\"2\">\n            <colspec colwidth=\"14\">\n            <colspec colwidth=\"19\">\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            Columns with\n                    <entry>\n                        <paragraph>\n                            automatic widths.\n",
		},
		{
			"table directive whose content isn't a table at all: ERROR, exactly one table expected",
			".. table:: Not a table.\n\n   This is a paragraph.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error parsing content block for the \"table\" directive: exactly one table expected.\n        <literal_block>\n            .. table:: Not a table.\n            \n               This is a paragraph.\n",
		},
		{
			"table directive with no content at all: WARNING, none found",
			".. table:: empty\n",
			"<document>\n    <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Content block expected for the \"table\" directive; none found.\n        <literal_block>\n            .. table:: empty\n",
		},
		{
			"list-table: title, :widths:, :header-rows:, :stub-columns:, each innermost list item's own children become one entry",
			".. list-table:: list table with integral header\n   :widths: 10 20 30\n   :header-rows: 1\n   :stub-columns: 1\n\n   * - Treat\n     - Quantity\n     - Description\n   * - Albatross\n     - 2.99\n     - On a stick!\n",
			"<document>\n    <table class=\"colwidths-given\">\n        <title>\n            list table with integral header\n        <tgroup cols=\"3\">\n            <colspec colwidth=\"10\" stub=\"1\">\n            <colspec colwidth=\"20\">\n            <colspec colwidth=\"30\">\n            <thead>\n                <row>\n                    <entry>\n                        <paragraph>\n                            Treat\n                    <entry>\n                        <paragraph>\n                            Quantity\n                    <entry>\n                        <paragraph>\n                            Description\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            Albatross\n                    <entry>\n                        <paragraph>\n                            2.99\n                    <entry>\n                        <paragraph>\n                            On a stick!\n",
		},
		{
			"list-table with :widths: not matching the actual column count: ERROR",
			".. list-table::\n   :widths: 10 20\n\n   * - \":widths:\" option doesn't match columns\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            \"list-table\" widths do not match the number of columns in table (1).\n        <literal_block>\n            .. list-table::\n               :widths: 10 20\n            \n               * - \":widths:\" option doesn't match columns\n",
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
