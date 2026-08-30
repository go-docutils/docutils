package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// Expected trees below were cross-checked against reference docutils
// 0.23 for element shape. For most cases this used
// `publish_string(..., writer_name='pseudoxml', settings_overrides=
// {'doctitle_xform': False})`; for footnotes/citations/substitutions,
// reference-resolution attributes, and tables, comparison instead used a
// bare `Parser().parse(src, document)` to see the raw pre-transform
// tree — some of these node shapes (numbering, substitution-value
// inlining, indirect-target/embedded-link resolution into a sibling
// <target>) are produced by transforms that run AFTER parsing, not by
// the parser itself (a table's <tgroup>/<colspec> column-width metadata,
// checked the same way, turned out to be a genuine exception — it IS
// part of the bare parse, see table.go/gridtable.go). This parser's OWN reference-resolution pass
// (resolveTargets, see explicit.go) runs at the end of Parse, so a
// refuri some fixtures below show already filled in is this parser
// resolving it eagerly, matching docutils' post-transform behavior even
// though the comparison target was pre-transform for the rest of the
// shape. This project's own doctree.Dump format is used throughout
// since ids/source attributes are not yet implemented (see the package
// doc's SCOPE note). A field list at the very start of a document is
// compared with `docinfo_xform: False` too — real docutils otherwise
// promotes it to a typed `<docinfo>` node, not implemented here. An unknown interpreted-text role
// is compared against docutils' error shape too (a problematic node
// plus a system-message section): this parser has no role registry, so
// it falls back to a generic <inline role="..."> instead of erroring
// (see inline.go). The simple-table and grid-table fixtures are
// docutils' OWN SimpleTableParser/GridTableParser docstring examples,
// verbatim — including header, multi-line cell, nested bullet list,
// column-span/row-span rows, and the tgroup/colspec wrapper. Two more
// known, documented divergences: a directive (including a substitution
// definition's embedded `replace::`) is captured structurally rather
// than dispatched to semantics, and an unresolved reference stays a
// bare reference node instead of being rewritten to `problematic` with
// an appended system-message section.
func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "sections, paragraph with inline markup, lists",
			source: "Section Title\n=============\n\nSubsection\n----------\n\nA paragraph with *emphasis* and ``literal`` text.\n\n- a bullet\n- list\n\n1. an enumerated\n2. list\n",
			want:   "<document>\n    <section>\n        <title>\n            Section Title\n        <section>\n            <title>\n                Subsection\n            <paragraph>\n                A paragraph with \n                <emphasis>\n                    emphasis\n                 and \n                <literal>\n                    literal\n                 text.\n            <bullet_list bullet=\"-\">\n                <list_item>\n                    <paragraph>\n                        a bullet\n                <list_item>\n                    <paragraph>\n                        list\n            <enumerated_list>\n                <list_item>\n                    <paragraph>\n                        an enumerated\n                <list_item>\n                    <paragraph>\n                        list\n",
		},
		{
			name:   "paragraph, block quote with two paragraphs, transition, paragraph",
			source: "First paragraph.\n\n    An indented block quote.\n\n    Second line of quote.\n\n----\n\nLast paragraph.\n",
			want:   "<document>\n    <paragraph>\n        First paragraph.\n    <block_quote>\n        <paragraph>\n            An indented block quote.\n        <paragraph>\n            Second line of quote.\n    <transition>\n    <paragraph>\n        Last paragraph.\n",
		},
		{
			name:   "enumerated list item containing a nested bullet list",
			source: "1. outer item one\n\n   - nested bullet a\n   - nested bullet b\n\n2. outer item two\n",
			want:   "<document>\n    <enumerated_list>\n        <list_item>\n            <paragraph>\n                outer item one\n            <bullet_list bullet=\"-\">\n                <list_item>\n                    <paragraph>\n                        nested bullet a\n                <list_item>\n                    <paragraph>\n                        nested bullet b\n        <list_item>\n            <paragraph>\n                outer item two\n",
		},
		{
			name:   "block quote containing a bullet list and a second paragraph",
			source: "Intro.\n\n    - item in quote\n    - second item\n\n    Another paragraph in the same quote.\n",
			want:   "<document>\n    <paragraph>\n        Intro.\n    <block_quote>\n        <bullet_list bullet=\"-\">\n            <list_item>\n                <paragraph>\n                    item in quote\n            <list_item>\n                <paragraph>\n                    second item\n        <paragraph>\n            Another paragraph in the same quote.\n",
		},
		{
			name:   "list item with two paragraphs and a nested enumerated list",
			source: "1. outer item\n\n   Second paragraph of outer item.\n\n   1. nested enumerated a\n   2. nested enumerated b\n",
			want:   "<document>\n    <enumerated_list>\n        <list_item>\n            <paragraph>\n                outer item\n            <paragraph>\n                Second paragraph of outer item.\n            <enumerated_list>\n                <list_item>\n                    <paragraph>\n                        nested enumerated a\n                <list_item>\n                    <paragraph>\n                        nested enumerated b\n",
		},
		{
			name:   "comment, directive, hyperlink target with reference resolution, literal block",
			source: "Intro paragraph.\n\n.. This is a comment.\n   Second comment line.\n\n.. note::\n\n   This is directive content.\n   Second line.\n\nSee `Example`_ for details.\n\n.. _Example: https://example.com\n\nHere is a code sample::\n\n    def f():\n        return 1\n\nDone.\n",
			want:   "<document>\n    <paragraph>\n        Intro paragraph.\n    <comment>\n        This is a comment.\n        Second comment line.\n    <directive name=\"note\">\n        This is directive content.\n        Second line.\n    <paragraph>\n        See \n        <reference name=\"Example\" refname=\"example\" refuri=\"https://example.com\">\n            Example\n         for details.\n    <target name=\"example\" refuri=\"https://example.com\">\n    <paragraph>\n        Here is a code sample:\n    <literal_block>\n        def f():\n            return 1\n    <paragraph>\n        Done.\n",
		},
		{
			name:   "unresolved reference, empty comment, plain comment, directive with no content",
			source: "An unresolved `Nowhere`_ reference.\n\n..\n\n.. plain comment no directive shape\n\n.. figure::\n",
			want:   "<document>\n    <paragraph>\n        An unresolved \n        <reference name=\"Nowhere\" refname=\"nowhere\">\n            Nowhere\n         reference.\n    <comment>\n    <comment>\n        plain comment no directive shape\n    <directive name=\"figure\">\n",
		},
		{
			name:   "list item containing a comment and a literal block",
			source: "- item one\n\n  .. a comment inside a list item\n\n  a literal sample::\n\n      x = 1\n\n- item two\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item one\n            <comment>\n                a comment inside a list item\n            <paragraph>\n                a literal sample:\n            <literal_block>\n                x = 1\n        <list_item>\n            <paragraph>\n                item two\n",
		},
		{
			name:   "field list with a continuation line indented less than the marker column, definition list",
			source: ":author: Jane Doe\n:version: 1.0\n:date: 2026-08-30\n  continuation line for date\n\nTerm one\n    Definition of term one.\n\nTerm two\n    First paragraph of definition two.\n\n    Second paragraph of definition two.\n\n    - a nested bullet inside the definition\n",
			want:   "<document>\n    <field_list>\n        <field>\n            <field_name>\n                author\n            <field_body>\n                <paragraph>\n                    Jane Doe\n        <field>\n            <field_name>\n                version\n            <field_body>\n                <paragraph>\n                    1.0\n        <field>\n            <field_name>\n                date\n            <field_body>\n                <paragraph>\n                    2026-08-30\n                    continuation line for date\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term one\n            <definition>\n                <paragraph>\n                    Definition of term one.\n        <definition_list_item>\n            <term>\n                Term two\n            <definition>\n                <paragraph>\n                    First paragraph of definition two.\n                <paragraph>\n                    Second paragraph of definition two.\n                <bullet_list bullet=\"-\">\n                    <list_item>\n                        <paragraph>\n                            a nested bullet inside the definition\n",
		},
		{
			name:   "a blank line after a would-be term prevents definition-list detection",
			source: "Not a term because next line is blank.\n\nTerm with no definition body next line\n\nA regular paragraph that follows.\n",
			want:   "<document>\n    <paragraph>\n        Not a term because next line is blank.\n    <paragraph>\n        Term with no definition body next line\n    <paragraph>\n        A regular paragraph that follows.\n",
		},
		{
			name:   "line block with a nested indented sub-line, and doctest block",
			source: "Intro paragraph.\n\n| Line one of the poem\n| Line two of the poem\n|     Indented line three\n\nA regular paragraph.\n\n>>> 1 + 1\n2\n>>> print(\"done\")\ndone\n\nFinal paragraph.\n",
			want:   "<document>\n    <paragraph>\n        Intro paragraph.\n    <line_block>\n        <line>\n            Line one of the poem\n        <line>\n            Line two of the poem\n        <line_block>\n            <line>\n                Indented line three\n    <paragraph>\n        A regular paragraph.\n    <doctest_block>\n        >>> 1 + 1\n        2\n        >>> print(\"done\")\n        done\n    <paragraph>\n        Final paragraph.\n",
		},
		{
			name:   "list item containing a line block and a doctest block",
			source: "- item with a line block\n\n  | verse one\n  | verse two\n\n- item with a doctest block\n\n  >>> x = 1\n  >>> x\n  1\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item with a line block\n            <line_block>\n                <line>\n                    verse one\n                <line>\n                    verse two\n        <list_item>\n            <paragraph>\n                item with a doctest block\n            <doctest_block>\n                >>> x = 1\n                >>> x\n                1\n",
		},
		{
			name:   "footnote, citation, and substitution references with their block definitions",
			source: "See footnote [1]_, an auto footnote [#]_, an auto-symbol [*]_,\na named auto footnote [#note]_, and a citation [CIT2002]_.\n\n.. [1] A manually numbered footnote.\n\n.. [#] An auto-numbered footnote.\n\n.. [*] An auto-symbol footnote.\n\n.. [#note] A named auto-numbered footnote.\n\n.. [CIT2002] A citation body.\n\nReplace this |name| with something.\n\n.. |name| replace:: substituted text\n",
			want:   "<document>\n    <paragraph>\n        See footnote \n        <footnote_reference refname=\"1\">\n            1\n        , an auto footnote \n        <footnote_reference auto=\"1\" refname=\"footnote-1\">\n            2\n        , an auto-symbol \n        <footnote_reference auto=\"*\" refname=\"footnote-2\">\n            *\n        ,\n        a named auto footnote \n        <footnote_reference auto=\"1\" refname=\"note\">\n            3\n        , and a citation \n        <citation_reference refname=\"cit2002\">\n            CIT2002\n        .\n    <footnote name=\"1\">\n        <label>\n            1\n        <paragraph>\n            A manually numbered footnote.\n    <footnote auto=\"1\" name=\"footnote-1\">\n        <label>\n            2\n        <paragraph>\n            An auto-numbered footnote.\n    <footnote auto=\"*\" name=\"footnote-2\">\n        <label>\n            *\n        <paragraph>\n            An auto-symbol footnote.\n    <footnote auto=\"1\" name=\"note\">\n        <label>\n            3\n        <paragraph>\n            A named auto-numbered footnote.\n    <citation name=\"cit2002\">\n        <label>\n            CIT2002\n        <paragraph>\n            A citation body.\n    <paragraph>\n        Replace this \n        <substitution_reference refname=\"name\">\n            name\n         with something.\n    <substitution_definition arguments=\"substituted text\" name=\"replace\" substitution=\"name\">\n",
		},
		{
			name:   "footnote and substitution definition inside list items",
			source: "- item with a footnote\n\n  .. [1] A footnote inside a list item.\n\n- item with a substitution definition\n\n  .. |x| replace:: y\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item with a footnote\n            <footnote name=\"1\">\n                <label>\n                    1\n                <paragraph>\n                    A footnote inside a list item.\n        <list_item>\n            <paragraph>\n                item with a substitution definition\n            <substitution_definition arguments=\"y\" name=\"replace\" substitution=\"x\">\n",
		},
		{
			name:   "bare and embedded-link references, indirect alias, default-role bare text",
			source: "A bare_ reference and an anon__ one.\n\n.. _bare: https://bare.example.com\n\nEmbedded `Python <https://python.org>`_ link, an anonymous `embedded <https://anon.example.com>`__ link,\nand an indirect `alias name <target_>`_ reference.\n\n.. _target: https://indirect.example.com\n\nPlain `text` uses the default role.\n",
			want:   "<document>\n    <paragraph>\n        A \n        <reference name=\"bare\" refname=\"bare\" refuri=\"https://bare.example.com\">\n            bare\n         reference and an \n        <reference anonymous=\"true\" name=\"anon\">\n            anon\n         one.\n    <target name=\"bare\" refuri=\"https://bare.example.com\">\n    <paragraph>\n        Embedded \n        <reference name=\"Python\" refuri=\"https://python.org\">\n            Python\n         link, an anonymous \n        <reference name=\"embedded\" refuri=\"https://anon.example.com\">\n            embedded\n         link,\n        and an indirect \n        <reference name=\"alias name\" refname=\"target\" refuri=\"https://indirect.example.com\">\n            alias name\n         reference.\n    <target name=\"target\" refuri=\"https://indirect.example.com\">\n    <paragraph>\n        Plain \n        <title_reference>\n            text\n         uses the default role.\n",
		},
		{
			name:   "interpreted text roles: prefix, suffix, aliases, unknown role, literal unaffected",
			source: "plain `text` here, a suffix role `role text`:emphasis: and a prefix role :strong:`prefix role`.\nAlso :sub:`subscript`, :sup:`superscript`, :title:`a title`, :ab:`WHO`, :acronym:`NASA`.\nAn unknown role :custom:`text` becomes generic, and so does `other`:unknown-role:.\nA literal ``code()`` still works fine next to it.\n",
			want:   "<document>\n    <paragraph>\n        plain \n        <title_reference>\n            text\n         here, a suffix role \n        <emphasis>\n            role text\n         and a prefix role \n        <strong>\n            prefix role\n        .\n        Also \n        <subscript>\n            subscript\n        , \n        <superscript>\n            superscript\n        , \n        <title_reference>\n            a title\n        , \n        <abbreviation>\n            WHO\n        , \n        <acronym>\n            NASA\n        .\n        An unknown role \n        <inline role=\"custom\">\n            text\n         becomes generic, and so does \n        <inline role=\"unknown-role\">\n            other\n        .\n        A literal \n        <literal>\n            code()\n         still works fine next to it.\n",
		},
		{
			name:   "simple table with header, multi-line cell, nested list, and a colspan",
			source: "=====  =====\ncol 1  col 2\n=====  =====\n1      Second column of row 1.\n2      Second column of row 2.\n       Second line of paragraph.\n3      - Second column of row 3.\n\n       - Second item in bullet\n         list (row 3, column 2).\n4 is a span\n------------\n5\n=====  =====\n",
			want:   "<document>\n    <table>\n        <tgroup cols=\"2\">\n            <colspec colwidth=\"5\">\n            <colspec colwidth=\"25\">\n            <thead>\n                <row>\n                    <entry>\n                        <paragraph>\n                            col 1\n                    <entry>\n                        <paragraph>\n                            col 2\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            1\n                    <entry>\n                        <paragraph>\n                            Second column of row 1.\n                <row>\n                    <entry>\n                        <paragraph>\n                            2\n                    <entry>\n                        <paragraph>\n                            Second column of row 2.\n                            Second line of paragraph.\n                <row>\n                    <entry>\n                        <paragraph>\n                            3\n                    <entry>\n                        <bullet_list bullet=\"-\">\n                            <list_item>\n                                <paragraph>\n                                    Second column of row 3.\n                            <list_item>\n                                <paragraph>\n                                    Second item in bullet\n                                    list (row 3, column 2).\n                <row>\n                    <entry morecols=\"1\">\n                        <paragraph>\n                            4 is a span\n                <row>\n                    <entry>\n                        <paragraph>\n                            5\n                    <entry>\n",
		},
		{
			name:   "simple table with no header",
			source: "=====  =====\n1      one\n2      two\n=====  =====\n",
			want:   "<document>\n    <table>\n        <tgroup cols=\"2\">\n            <colspec colwidth=\"5\">\n            <colspec colwidth=\"5\">\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            1\n                    <entry>\n                        <paragraph>\n                            one\n                <row>\n                    <entry>\n                        <paragraph>\n                            2\n                    <entry>\n                        <paragraph>\n                            two\n",
		},
		{
			name:   "list item containing a simple table",
			source: "- item with a table\n\n  =====  =====\n  a      b\n  =====  =====\n  1      2\n  =====  =====\n\n- item two\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item with a table\n            <table>\n                <tgroup cols=\"2\">\n                    <colspec colwidth=\"5\">\n                    <colspec colwidth=\"5\">\n                    <thead>\n                        <row>\n                            <entry>\n                                <paragraph>\n                                    a\n                            <entry>\n                                <paragraph>\n                                    b\n                    <tbody>\n                        <row>\n                            <entry>\n                                <paragraph>\n                                    1\n                            <entry>\n                                <paragraph>\n                                    2\n        <list_item>\n            <paragraph>\n                item two\n",
		},
		{
			name:   "standalone URI, email, and trailing punctuation",
			source: "Visit https://example.com/path?q=1 or email me at jane@example.com now. Trailing punctuation: https://x.org, and https://y.org.\n",
			want:   "<document>\n    <paragraph>\n        Visit \n        <reference refuri=\"https://example.com/path?q=1\">\n            https://example.com/path?q=1\n         or email me at \n        <reference refuri=\"mailto:jane@example.com\">\n            jane@example.com\n         now. Trailing punctuation: \n        <reference refuri=\"https://x.org\">\n            https://x.org\n        , and \n        <reference refuri=\"https://y.org\">\n            https://y.org\n        .\n",
		},
		{
			name:   "standalone URI inside a list item and email in another",
			source: "- see https://example.com here\n- and jane@example.org too\n\nA URL at end of line: https://example.com/end\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                see \n                <reference refuri=\"https://example.com\">\n                    https://example.com\n                 here\n        <list_item>\n            <paragraph>\n                and \n                <reference refuri=\"mailto:jane@example.org\">\n                    jane@example.org\n                 too\n    <paragraph>\n        A URL at end of line: \n        <reference refuri=\"https://example.com/end\">\n            https://example.com/end\n",
		},
		{
			name:   "grid table with header, column span, row span, and a nested list",
			source: "+------------------------+------------+----------+----------+\n| Header row, column 1   | Header 2   | Header 3 | Header 4 |\n+========================+============+==========+==========+\n| body row 1, column 1   | column 2   | column 3 | column 4 |\n+------------------------+------------+----------+----------+\n| body row 2             | Cells may span columns.          |\n+------------------------+------------+---------------------+\n| body row 3             | Cells may  | - Table cells       |\n+------------------------+ span rows. | - contain           |\n| body row 4             |            | - body elements.    |\n+------------------------+------------+---------------------+\n",
			want:   "<document>\n    <table>\n        <tgroup cols=\"4\">\n            <colspec colwidth=\"24\">\n            <colspec colwidth=\"12\">\n            <colspec colwidth=\"10\">\n            <colspec colwidth=\"10\">\n            <thead>\n                <row>\n                    <entry>\n                        <paragraph>\n                            Header row, column 1\n                    <entry>\n                        <paragraph>\n                            Header 2\n                    <entry>\n                        <paragraph>\n                            Header 3\n                    <entry>\n                        <paragraph>\n                            Header 4\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            body row 1, column 1\n                    <entry>\n                        <paragraph>\n                            column 2\n                    <entry>\n                        <paragraph>\n                            column 3\n                    <entry>\n                        <paragraph>\n                            column 4\n                <row>\n                    <entry>\n                        <paragraph>\n                            body row 2\n                    <entry morecols=\"2\">\n                        <paragraph>\n                            Cells may span columns.\n                <row>\n                    <entry>\n                        <paragraph>\n                            body row 3\n                    <entry morerows=\"1\">\n                        <paragraph>\n                            Cells may\n                            span rows.\n                    <entry morecols=\"1\" morerows=\"1\">\n                        <bullet_list bullet=\"-\">\n                            <list_item>\n                                <paragraph>\n                                    Table cells\n                            <list_item>\n                                <paragraph>\n                                    contain\n                            <list_item>\n                                <paragraph>\n                                    body elements.\n                <row>\n                    <entry>\n                        <paragraph>\n                            body row 4\n",
		},
		{
			name:   "headerless grid table",
			source: "+-----+-----+\n| a   | b   |\n+-----+-----+\n| 1   | 2   |\n+-----+-----+\n",
			want:   "<document>\n    <table>\n        <tgroup cols=\"2\">\n            <colspec colwidth=\"5\">\n            <colspec colwidth=\"5\">\n            <tbody>\n                <row>\n                    <entry>\n                        <paragraph>\n                            a\n                    <entry>\n                        <paragraph>\n                            b\n                <row>\n                    <entry>\n                        <paragraph>\n                            1\n                    <entry>\n                        <paragraph>\n                            2\n",
		},
		{
			name:   "grid table inside a list item",
			source: "- item with a grid table\n\n  +-----+-----+\n  | a   | b   |\n  +-----+-----+\n  | 1   | 2   |\n  +-----+-----+\n\n- item two\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item with a grid table\n            <table>\n                <tgroup cols=\"2\">\n                    <colspec colwidth=\"5\">\n                    <colspec colwidth=\"5\">\n                    <tbody>\n                        <row>\n                            <entry>\n                                <paragraph>\n                                    a\n                            <entry>\n                                <paragraph>\n                                    b\n                        <row>\n                            <entry>\n                                <paragraph>\n                                    1\n                            <entry>\n                                <paragraph>\n                                    2\n        <list_item>\n            <paragraph>\n                item two\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if got != tc.want {
				t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
		})
	}
}

func TestInline(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"strong", "a **bold** word", "a \n<strong>\n    bold\n word\n"},
		{"emphasis", "an *italic* word", "an \n<emphasis>\n    italic\n word\n"},
		{"literal", "some ``code()`` here", "some \n<literal>\n    code()\n here\n"},
		{"named reference", "see `Section`_ for details", "see \n<reference name=\"Section\" refname=\"section\">\n    Section\n for details\n"},
		{"no nested markup inside strong", "**a *b* c**", "<strong>\n    a *b* c\n"},
		{"unmatched marker stays literal", "2 * 3 = 6", "2 * 3 = 6\n"},
		{"backslash escape suppresses markup", "\\*not emphasis\\*", "*not emphasis*\n"},
		{"anonymous reference", "see `some text`__ end", "see \n<reference anonymous=\"true\" name=\"some text\">\n    some text\n end\n"},
		{"unclosed marker falls back to plain text", "an *unclosed emphasis stays plain", "an *unclosed emphasis stays plain\n"},
		{"substitution reference", "see |name| here", "see \n<substitution_reference refname=\"name\">\n    name\n here\n"},
		{"manually numbered footnote reference", "see [1]_ here", "see \n<footnote_reference refname=\"1\">\n    1\n here\n"},
		{"auto-numbered footnote reference", "see [#]_ here", "see \n<footnote_reference auto=\"1\">\n here\n"},
		{"named auto-numbered footnote reference", "see [#note]_ here", "see \n<footnote_reference auto=\"1\" refname=\"note\">\n here\n"},
		{"auto-symbol footnote reference", "see [*]_ here", "see \n<footnote_reference auto=\"*\">\n here\n"},
		{"citation reference", "see [CIT2002]_ here", "see \n<citation_reference refname=\"cit2002\">\n    CIT2002\n here\n"},
		{"bare word reference", "see bare_ here", "see \n<reference name=\"bare\" refname=\"bare\">\n    bare\n here\n"},
		{"bare anonymous word reference", "see anon__ here", "see \n<reference anonymous=\"true\" name=\"anon\">\n    anon\n here\n"},
		{"bare default-role text", "see `plain text` here", "see \n<title_reference>\n    plain text\n here\n"},
		{"embedded URI phrase reference", "see `Python <https://python.org>`_ here", "see \n<reference name=\"Python\" refuri=\"https://python.org\">\n    Python\n here\n"},
		{"embedded indirect alias phrase reference", "see `alias <target_>`_ here", "see \n<reference name=\"alias\" refname=\"target\">\n    alias\n here\n"},
		{"prefix role", "see :strong:`bold text` here", "see \n<strong>\n    bold text\n here\n"},
		{"suffix role", "see `bold text`:strong: here", "see \n<strong>\n    bold text\n here\n"},
		{"role alias sub/sup", "see :sub:`x` and :sup:`y` here", "see \n<subscript>\n    x\n and \n<superscript>\n    y\n here\n"},
		{"unknown role falls back to generic inline", "see :custom:`x` here", "see \n<inline role=\"custom\">\n    x\n here\n"},
		{"standalone URI", "Visit https://example.com now.", "Visit \n<reference refuri=\"https://example.com\">\n    https://example.com\n now.\n"},
		{"standalone email", "Contact jane@example.com now.", "Contact \n<reference refuri=\"mailto:jane@example.com\">\n    jane@example.com\n now.\n"},
		{"standalone URI with trailing punctuation stripped", "See https://x.org, and https://y.org.", "See \n<reference refuri=\"https://x.org\">\n    https://x.org\n, and \n<reference refuri=\"https://y.org\">\n    https://y.org\n.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := parseInline(tc.text)
			var b strings.Builder
			for _, n := range nodes {
				b.WriteString(doctree.Dump(n))
			}
			if b.String() != tc.want {
				t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", b.String(), tc.want)
			}
		})
	}
}
