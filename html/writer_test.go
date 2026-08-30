package html

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/rst"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"paragraph with inline markup",
			"A paragraph with *emphasis*, **strong**, and ``literal``.\n",
			"<p>A paragraph with <em>emphasis</em>, <strong>strong</strong>, and <code>literal</code>.</p>",
		},
		{
			"nested sections use increasing heading levels",
			"Top\n===\n\nIntro.\n\nSub\n---\n\nNested.\n",
			"<section><h1>Top</h1><p>Intro.</p><section><h2>Sub</h2><p>Nested.</p></section></section>",
		},
		{
			"bullet and enumerated lists",
			"- a\n- b\n\n1. c\n2. d\n",
			"<ul><li><p>a</p></li><li><p>b</p></li></ul><ol><li><p>c</p></li><li><p>d</p></li></ol>",
		},
		{
			"block quote and transition",
			"Para.\n\n    Quoted.\n\n----\n\nAfter.\n",
			"<p>Para.</p><blockquote><p>Quoted.</p></blockquote><hr><p>After.</p>",
		},
		{
			"literal block",
			"Sample::\n\n    code here\n",
			"<p>Sample:</p><pre><code>code here</code></pre>",
		},
		{
			"reference with resolved refuri",
			"See `Python <https://python.org>`_ now.\n",
			`<p>See <a href="https://python.org">Python</a> now.</p>`,
		},
		{
			"bare reference with no target becomes problematic text plus a trailing system-messages section",
			"See `nowhere`_ now.\n",
			"<p>See nowhere now.</p><section><h1>Docutils System Messages</h1><p>Unknown target name: &quot;nowhere&quot;.</p></section>",
		},
		{
			"inline internal target keeps its text and gets an id; a later reference resolves to a same-document anchor",
			"See the _`term` and later `term`_.\n",
			`<p>See the <a id="term">term</a> and later <a href="#term">term</a>.</p>`,
		},
		{
			"footnote reference links to its footnote by id",
			"See [1]_.\n\n.. [1] Text.\n",
			`<p>See <a href="#1">1</a>.</p><div class="footnote" id="1"><span class="label">1</span><p>Text.</p></div>`,
		},
		{
			"field list and definition list as dl/dt/dd",
			":name: value\n\nTerm\n    Def.\n",
			"<dl><dt>name</dt><dd><p>value</p></dd></dl><dl><dt>Term</dt><dd><p>Def.</p></dd></dl>",
		},
		{
			"a leading field list promotes typed bibliographic fields",
			":Author: Jane Doe\n:Authors: Jane Doe, John Smith\n:Version: 1.0\n\nBody.\n",
			"<dl><dt>author</dt><dd>Jane Doe</dd><dt>authors</dt><dd>Jane Doe, John Smith</dd><dt>version</dt><dd>1.0</dd></dl><p>Body.</p>",
		},
		{
			"simple table with header and colspan",
			"=====  =====\na      b\n=====  =====\n1      2\n=====  =====\n",
			"<table><thead><tr><th><p>a</p></th><th><p>b</p></th></tr></thead>" +
				"<tbody><tr><td><p>1</p></td><td><p>2</p></td></tr></tbody></table>",
		},
		{
			"HTML-special characters are escaped",
			"Escaping <tags> & \"quotes\".\n",
			"<p>Escaping &lt;tags&gt; &amp; &quot;quotes&quot;.</p>",
		},
		{
			"unknown role renders as span with class",
			"See :custom:`text` here.\n",
			`<p>See <span class="custom">text</span> here.</p>`,
		},
		{
			"subscript, superscript, abbreviation",
			":sub:`x` :sup:`y` :ab:`WHO`\n",
			"<p><sub>x</sub> <sup>y</sup> <abbr>WHO</abbr></p>",
		},
		{
			"code role renders as a plain code span, same as a backtick literal",
			":code:`x = 1`\n",
			"<p><code>x = 1</code></p>",
		},
		{
			"math role renders with the MathJax inline delimiters, HTML-escaped",
			"A :math:`x < y` term.\n",
			`<p>A \(x &lt; y\) term.</p>`,
		},
		{
			"comment renders as an HTML comment",
			".. a comment\n",
			"<!-- a comment -->",
		},
		{
			"directive renders as a labeled pre block, not silently dropped",
			".. note::\n\n   content\n",
			`<pre class="directive" data-directive="note">content</pre>`,
		},
		{
			"line block renders as nested divs, one per line",
			"| one\n| two\n",
			`<div class="line-block"><div>one</div><div>two</div></div>`,
		},
		{
			"citation and citation reference",
			"See [CIT2002]_.\n\n.. [CIT2002] Text.\n",
			`<p>See <a href="#cit2002">CIT2002</a>.</p><div class="footnote" id="cit2002"><span class="label">CIT2002</span><p>Text.</p></div>`,
		},
		{
			"substitution definition is invisible, unresolved reference falls back to its name as text",
			"A |sub| here.\n\n.. |sub| replace:: value\n",
			"<p>A sub here.</p>",
		},
		{
			"auto-numbered footnote reference gets a real assigned number and a working link (see footnotenum.go)",
			"An auto footnote [#]_.\n\n.. [#] Text.\n",
			`<p>An auto footnote <a href="#footnote-1">1</a>.</p><div class="footnote" id="footnote-1"><span class="label">1</span><p>Text.</p></div>`,
		},
		{
			"grid table column span renders as colspan",
			"+-----+-----+-----+\n| a   | b   | c   |\n+-----+-----+-----+\n| wide      | d   |\n+-----+-----+-----+\n",
			`<table><tbody><tr><td><p>a</p></td><td><p>b</p></td><td><p>c</p></td></tr>` +
				`<tr><td colspan="2"><p>wide</p></td><td><p>d</p></td></tr></tbody></table>`,
		},
		{
			"grid table row span renders as rowspan",
			"+-----+------+\n| a   | tall |\n+-----+      |\n| b   |      |\n+-----+------+\n",
			`<table><tbody><tr><td><p>a</p></td><td rowspan="2"><p>tall</p></td></tr>` +
				`<tr><td><p>b</p></td></tr></tbody></table>`,
		},
		{
			"option list renders as a dl, grouped options joined by comma",
			"-f, --file=FILE  Grouped short+long.\n-ovalue       Embedded.\n",
			`<dl><dt>-f, --file=FILE</dt><dd><p>Grouped short+long.</p></dd>` +
				`<dt>-ovalue</dt><dd><p>Embedded.</p></dd></dl>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := rst.Parse(tc.source)
			got := Render(doc)
			if got != tc.want {
				t.Errorf("Render(%q) =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

func TestRenderTagsAreBalanced(t *testing.T) {
	source := "Top\n===\n\n- a\n\n  - nested\n\n1. b\n\n:field: value\n\nTerm\n    Def.\n\n-f  Opt.\n\n=====  =====\na      b\n=====  =====\n1      2\n=====  =====\n\nSee [1]_.\n\n.. [1] Text.\n"
	got := Render(rst.Parse(source))
	var stack []string
	i := 0
	for i < len(got) {
		if got[i] != '<' {
			i++
			continue
		}
		end := strings.IndexByte(got[i:], '>')
		if end < 0 {
			t.Fatalf("unterminated tag at %d in %s", i, got)
		}
		tagText := got[i+1 : i+end]
		i += end + 1
		if tagText == "" || tagText[0] == '!' {
			continue
		}
		if strings.HasPrefix(tagText, "/") {
			name := tagText[1:]
			if len(stack) == 0 || stack[len(stack)-1] != name {
				t.Fatalf("mismatched close tag %q, stack=%v\nfull output: %s", name, stack, got)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		name, _, _ := strings.Cut(tagText, " ")
		if name == "hr" {
			continue
		}
		stack = append(stack, name)
	}
	if len(stack) != 0 {
		t.Fatalf("unclosed tags at EOF: %v\nfull output: %s", stack, got)
	}
}
