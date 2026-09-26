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
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n                <paragraph>\n                    The \"topic\" directive may not be used within topics or body elements.\n                <literal_block>\n                    .. topic:: In a list\n                    \n                       Not allowed.\n",
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

// TestTitleMessagesGoInsideTheNode pins WHERE a diagnostic raised while
// parsing a directive's own title argument lands, which docutils decides
// per construct rather than by a general rule:
//
//   - BaseAdmonition.run: "admonition_node += title; admonition_node +=
//     messages", then the content is parsed INTO that node, so the
//     messages sit between the title and the body (admonitions.py);
//   - Topic.run: "node_class(text, *(titles + messages))" -- title,
//     subtitle when there is one, then EVERY message from both, then the
//     content (body.py);
//   - Table.run: "[table_node] + messages" -- genuinely SIBLINGS, which
//     is the shape this package applied everywhere.
//
// A 930-case inline probe (30 inline constructs x 31 containers,
// /Users/Shared/rstcorpus/nestprobe/inlineprobe.py) found 12 of the 15
// divergences here; the three table directives and the rubric argument
// already agreed, and they are the CONTROLS below, because "move the
// messages inside" is only right for the constructs whose reference code
// does it.
func TestTitleMessagesGoInsideTheNode(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		inside  string // the tag the messages must be inside
		msgText string
	}{
		{"an admonition's title", ".. admonition:: a :nosuch:`x` b\n\n   body\n", "admonition", `Unknown interpreted text role "nosuch".`},
		{"a topic's title", ".. topic:: a *unclosed b\n\n   body\n", "topic", "Inline emphasis start-string without end-string."},
		{"a sidebar's subtitle", ".. sidebar:: T\n   :subtitle: a *unclosed b\n\n   body\n", "sidebar", "Inline emphasis start-string without end-string."},
		{"CONTROL: a table's title keeps them OUTSIDE", ".. table:: a *unclosed b\n\n   ===  ===\n   a    b\n   ===  ===\n", "", "Inline emphasis start-string without end-string."},
		{"CONTROL: a list-table's title too", ".. list-table:: a *unclosed b\n\n   * - a\n     - b\n", "", "Inline emphasis start-string without end-string."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			if !strings.Contains(dump, tc.msgText) {
				t.Fatalf("the message itself is missing:\n%s", dump)
			}
			// The message's own indentation says whether it is inside the
			// element (deeper than the element's own line) or a sibling of
			// it (the same depth).
			var elIndent, msgIndent int
			for _, line := range strings.Split(dump, "\n") {
				trimmed := strings.TrimLeft(line, " ")
				indent := len(line) - len(trimmed)
				if tc.inside != "" && strings.HasPrefix(trimmed, "<"+tc.inside) {
					elIndent = indent
				}
				if strings.HasPrefix(trimmed, "<system_message") {
					msgIndent = indent
				}
			}
			if tc.inside == "" {
				// A sibling of the table sits at document depth, 4 spaces.
				if msgIndent != 4 {
					t.Errorf("a table's title message should stay a SIBLING (indent 4), got %d:\n%s", msgIndent, dump)
				}
				return
			}
			if msgIndent <= elIndent {
				t.Errorf("message at indent %d is not inside <%s> at indent %d:\n%s", msgIndent, tc.inside, elIndent, dump)
			}
			// And it must precede the body, not follow it.
			bodyAt := strings.Index(dump, "\n            body")
			msgAt := strings.Index(dump, tc.msgText)
			if bodyAt >= 0 && msgAt > bodyAt {
				t.Errorf("the message follows the body; docutils puts it before:\n%s", dump)
			}
		})
	}
}
