package latex

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/rst"
)

// TestRenderContains checks for the presence of expected LaTeX
// fragments rather than an exact match: Render's surrounding
// whitespace/blank-line choices aren't part of the contract, only that
// the right commands/environments appear in the right nesting.
func TestRenderContains(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			"inline markup",
			"A *em*, **strong**, and ``code``.\n",
			[]string{`\emph{em}`, `\textbf{strong}`, `\texttt{code}`},
		},
		{
			"code role renders the same as a backtick literal",
			":code:`x = 1`\n",
			[]string{`\texttt{x = 1}`},
		},
		{
			"math role renders as inline math mode, verbatim (not LaTeX-escaped)",
			"A :math:`x^2 + y_1` term.\n",
			[]string{`$x^2 + y_1$`},
		},
		{
			"nested sections use increasing-depth commands",
			"Top\n===\n\nSub\n---\n",
			[]string{`\section{Top}`, `\subsection{Sub}`},
		},
		{
			"lists",
			"- a\n- b\n\n1. c\n2. d\n",
			[]string{`\begin{itemize}`, `\item a`, `\item b`, `\end{itemize}`,
				`\begin{enumerate}`, `\item c`, `\item d`, `\end{enumerate}`},
		},
		{
			"block quote and transition",
			"Para.\n\n    Quoted.\n\n----\n",
			[]string{`\begin{quote}`, `Quoted.`, `\end{quote}`, `\hrulefill`},
		},
		{
			"literal block as verbatim",
			"Sample::\n\n    code here\n",
			[]string{`\begin{verbatim}`, "code here", `\end{verbatim}`},
		},
		{
			"reference with resolved refuri",
			"See `Python <https://python.org>`_ now.\n",
			[]string{`\href{https://python.org}{Python}`},
		},
		{
			"inline internal target and a same-document reference to it use hypertarget/hyperlink, not href",
			"See the _`term` and later `term`_.\n",
			[]string{`\hypertarget{term}{term}`, `\hyperlink{term}{term}`},
		},
		{
			"footnote reference links to its footnote by hypertarget",
			"See [1]_.\n\n.. [1] Text.\n",
			[]string{`\hyperlink{1}{[1]}`, `\hypertarget{1}{}`, `Text.`},
		},
		{
			"field list and definition list as description items",
			":name: value\n\nTerm\n    Def.\n",
			[]string{`\begin{description}`, `\item[{name}] value`, `\item[{Term}] Def.`},
		},
		{
			"a leading field list promotes typed bibliographic fields",
			":Author: Jane Doe\n:Authors: Jane Doe, John Smith\n:Version: 1.0\n\nBody.\n",
			[]string{`\begin{description}`, `\item[{author}] Jane Doe`, `\item[{authors}] Jane Doe, John Smith`, `\item[{version}] 1.0`},
		},
		{
			"simple table as tabular",
			"=====  =====\na      b\n=====  =====\n1      2\n=====  =====\n",
			[]string{`\begin{tabular}{ll}`, `a & b`, `1 & 2`, `\end{tabular}`},
		},
		{
			"special characters are escaped",
			"100% & $x$ #tag ~t ^c _u {b}.\n",
			[]string{`100\% \& \$x\$ \#tag \textasciitilde{}t \textasciicircum{}c \_u \{b\}.`},
		},
		{
			"comment renders as LaTeX line comments",
			".. a comment\n",
			[]string{"% a comment"},
		},
		{
			"directive renders as a labeled verbatim block, not silently dropped",
			".. note::\n\n   content\n",
			[]string{`[directive: note]`, "content"},
		},
		{
			"document wrapper",
			"Hello.\n",
			[]string{`\documentclass{article}`, `\usepackage{hyperref}`, `\begin{document}`, `\end{document}`},
		},
		{
			"citation and citation reference",
			"See [CIT2002]_.\n\n.. [CIT2002] Text.\n",
			[]string{`\hyperlink{cit2002}{[CIT2002]}`, `\hypertarget{cit2002}{}`, `Text.`},
		},
		{
			"line block uses the verse environment",
			"| one\n| two\n",
			[]string{`\begin{verse}`, `one \\`, `two \\`, `\end{verse}`},
		},
		{
			"substitution definition is invisible, unresolved reference falls back to its name",
			"A |sub| here.\n\n.. |sub| replace:: value\n",
			[]string{"A sub here."},
		},
		{
			"unresolved reference falls back to plain text with no href",
			"See `nowhere`_ now.\n",
			[]string{"See nowhere now."},
		},
		{
			"headerless simple table",
			"=====  =====\n1      one\n2      two\n=====  =====\n",
			[]string{`\begin{tabular}{ll}`, `1 & one`, `2 & two`},
		},
		{
			"grid table column span renders as \\multicolumn",
			"+-----+-----+-----+\n| a   | b   | c   |\n+-----+-----+-----+\n| wide      | d   |\n+-----+-----+-----+\n",
			[]string{`\begin{tabular}{lll}`, `a & b & c`, `\multicolumn{2}{l}{wide} & d`},
		},
		{
			"option list as a description environment, grouped options joined by comma",
			"-f, --file=FILE  Grouped short+long.\n-ovalue       Embedded.\n",
			[]string{`\begin{description}`, `\item[{-f, --file=FILE}] Grouped short+long.`, `\item[{-ovalue}] Embedded.`, `\end{description}`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(rst.Parse(tc.source))
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("Render(%q) missing %q\ngot:\n%s", tc.source, want, got)
				}
			}
		})
	}
}

func TestRenderBalancesEnvironments(t *testing.T) {
	source := "Top\n===\n\n- a\n\n1. b\n\n:field: value\n\nTerm\n    Def.\n\n" +
		"    Quoted.\n\n----\n\nCode::\n\n    x\n\n=====  =====\na      b\n=====  =====\n1      2\n=====  =====\n\n-f  Opt.\n"
	got := Render(rst.Parse(source))
	var stack []string
	i := 0
	for i < len(got) {
		if strings.HasPrefix(got[i:], `\begin{`) {
			end := strings.IndexByte(got[i:], '}')
			name := got[i+len(`\begin{`) : i+end]
			stack = append(stack, name)
			i += end + 1
			continue
		}
		if strings.HasPrefix(got[i:], `\end{`) {
			end := strings.IndexByte(got[i:], '}')
			name := got[i+len(`\end{`) : i+end]
			if len(stack) == 0 || stack[len(stack)-1] != name {
				t.Fatalf("mismatched \\end{%s}, stack=%v\nfull output:\n%s", name, stack, got)
			}
			stack = stack[:len(stack)-1]
			i += end + 1
			continue
		}
		i++
	}
	if len(stack) != 0 {
		t.Fatalf("unclosed environments at EOF: %v\nfull output:\n%s", stack, got)
	}
}
