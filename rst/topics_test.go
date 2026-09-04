package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestTopicsAndSidebars covers docutils.parsers.rst.directives.body's
// BasePseudoSection/Topic/Sidebar (body.py, read directly): topic's
// REQUIRED title argument vs. sidebar's OPTIONAL one (plus sidebar's
// own :subtitle:, valid only alongside a title), :class:/:name: options,
// the "content required"/"argument required" diagnostics, and the
// nesting restriction — a topic/sidebar is only valid directly inside
// <document>/<section> (topic ALSO directly inside <sidebar>); anywhere
// else (a list item, another topic, ...) is an ERROR, checked against
// the parser's own `parent` argument, which IS the same "current
// container" real docutils checks (state_machine.node). Every case
// verified against the foreign judge (Parser().parse(), the same bare,
// pre-transform tree doctree.Dump produces).
func TestTopicsAndSidebars(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a topic with a title and body",
			".. topic:: Title\n\n   Body.\n",
			"<document>\n    <topic>\n        <title>\n            Title\n        <paragraph>\n            Body.\n",
		},
		{
			":class:/:name: options",
			".. topic:: With Options\n   :class: custom\n   :name: my point\n\n   Body.\n",
			"<document>\n    <topic class=\"custom\" id=\"my-point\" name=\"my point\">\n        <title>\n            With Options\n        <paragraph>\n            Body.\n",
		},
		{
			"a topic with no title argument at all: ERROR (title is REQUIRED, unlike sidebar)",
			".. topic::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"topic\" directive:\n            1 argument(s) required, 0 supplied.\n        <literal_block>\n            .. topic::\n",
		},
		{
			"a topic with a title but no content: ERROR",
			".. topic:: Title\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"topic\" directive; none found.\n        <literal_block>\n            .. topic:: Title\n",
		},
		{
			"two sibling topics at the top level, both valid",
			".. topic:: First\n\n   Body\n\n.. topic:: Second\n\n   Body.\n",
			"<document>\n    <topic>\n        <title>\n            First\n        <paragraph>\n            Body\n    <topic>\n        <title>\n            Second\n        <paragraph>\n            Body.\n",
		},
		{
			"a sidebar with a title, :subtitle:, and a nested topic (valid: topic may nest inside sidebar)",
			".. sidebar:: Title\n   :subtitle: Outer\n\n   .. topic:: Nested\n\n      Body.\n\n   More.\n\nMore.\n",
			"<document>\n    <sidebar>\n        <title>\n            Title\n        <subtitle>\n            Outer\n        <topic>\n            <title>\n                Nested\n            <paragraph>\n                Body.\n        <paragraph>\n            More.\n    <paragraph>\n        More.\n",
		},
		{
			// No "line" attribute: a list item's own content is a rebased
			// sub-slice with no known absolute-document correspondence
			// threaded through it (parser.currentLine's own doc comment;
			// runTopicOrSidebar's lineBase param is only ever real for
			// TOPIC/SIDEBAR content specifically, not list items) — the
			// SAME "unknown → omitted, never a coincidentally-plausible
			// wrong number" convention msgLine already gives every other
			// diagnostic in this package. An earlier version of this test
			// expected line="1", which happened to be numerically correct
			// for THIS specific one-line input purely by chance (the
			// pre-fix code always reported "i+1" using an index local to
			// the list item's own rebased content, not the real document)
			// — verified against the foreign judge that real docutils
			// itself DOES report the true absolute line here, which this
			// project still doesn't track for list-item content at all;
			// this test only guards against a WRONG plausible-looking
			// number reappearing, not full parity with real docutils.
			"a topic inside a list item is NOT one of topic's valid parents: ERROR",
			"- .. topic:: In a list\n\n     Not allowed.\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <system_message level=\"3\" type=\"ERROR\">\n                <paragraph>\n                    The \"topic\" directive may not be used within topics or body elements.\n                <literal_block>\n                    .. topic:: In a list\n                    \n                       Not allowed.\n",
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

// TestTopicsAndSidebarsNestedLineNumbers covers the line-number/warning
// gap this file's own README flagged as "worth a dedicated round"
// (v0.23.0's code_parsing][0], confirmed to also affect nested topic/
// sidebar ERROR messages in v0.28.0): a NESTED topic/sidebar's own
// rejection diagnostic used to always report line=1 (an index local to
// the OUTER topic/sidebar's own rebased content, not the real document),
// now a real absolute line via runTopicOrSidebar's own lineBase/
// contentLineBase threading — see that function's own doc comment for
// the derivation. The two-levels-deep case (sidebar > topic > sidebar)
// checks the threading survives more than one level of nesting, not
// just one.
func TestTopicsAndSidebarsNestedLineNumbers(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a nested sidebar reports its OWN real absolute line, not the outer sidebar's",
			".. sidebar:: Outer\n\n   .. sidebar:: Nested\n\n      Body.\n",
			"<document>\n    <sidebar>\n        <title>\n            Outer\n        <system_message level=\"3\" line=\"3\" type=\"ERROR\">\n            <paragraph>\n                The \"sidebar\" directive may not be used within a sidebar element.\n            <literal_block>\n                .. sidebar:: Nested\n                \n                   Body.\n",
		},
		{
			"two levels of nesting (sidebar > topic > sidebar) still threads a real line number",
			".. sidebar:: Outer\n\n   .. topic:: Topic\n\n      .. sidebar:: Inner\n\n         text\n",
			"<document>\n    <sidebar>\n        <title>\n            Outer\n        <topic>\n            <title>\n                Topic\n            <system_message level=\"3\" line=\"5\" type=\"ERROR\">\n                <paragraph>\n                    The \"sidebar\" directive may not be used within topics or body elements.\n                <literal_block>\n                    .. sidebar:: Inner\n                    \n                       text\n",
		},
		{
			"a nested topic that ends abruptly (no blank line) ALSO gets the missing unindent warning",
			".. topic:: Title\n\n   .. topic:: Nested\n\n      Body.\n   More.\n",
			"<document>\n    <topic>\n        <title>\n            Title\n        <system_message level=\"3\" line=\"3\" type=\"ERROR\">\n            <paragraph>\n                The \"topic\" directive may not be used within topics or body elements.\n            <literal_block>\n                .. topic:: Nested\n                \n                   Body.\n        <system_message level=\"2\" line=\"6\" type=\"WARNING\">\n            <paragraph>\n                Explicit markup ends without a blank line; unexpected unindent.\n        <paragraph>\n            More.\n",
		},
		{
			"the SAME shape, but blank-line-separated: no unindent warning",
			".. topic:: Title\n\n   .. topic:: Nested\n\n      Body.\n\n   More.\n\nMore.\n",
			"<document>\n    <topic>\n        <title>\n            Title\n        <system_message level=\"3\" line=\"3\" type=\"ERROR\">\n            <paragraph>\n                The \"topic\" directive may not be used within topics or body elements.\n            <literal_block>\n                .. topic:: Nested\n                \n                   Body.\n        <paragraph>\n            More.\n    <paragraph>\n        More.\n",
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
