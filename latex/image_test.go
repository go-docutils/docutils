package latex_test

import (
	"strings"
	"testing"
)

// TestAnImageReachesTheLatex is the latex half of the same gap: 120 renderable
// images in 46 of the 1564 real-world corpus files came out as nothing, and the
// content probe was blind to every one because an image carries no text.
//
// Each expectation was taken from the reference writer run on that exact
// source. Two of them are why: a UNITLESS measure is big points ("200" ->
// "200bp", not "200"), and a PERCENTAGE is a fraction of \linewidth -- neither
// of which the HTML side does, so the same doctree attribute means two
// different things in the two writers.
func TestAnImageReachesTheLatex(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"plain", ".. image:: a.png\n", `\includegraphics{a.png}`},
		{"unitless width is big points", ".. image:: a.png\n   :width: 200\n", `\includegraphics[width=200bp]{a.png}`},
		{"a percentage is a fraction of \\linewidth", ".. image:: b.png\n   :width: 50%\n   :scale: 50\n", `\includegraphics[scale=0.5,width=0.5\linewidth]{b.png}`},
		{"height then width", ".. image:: c.png\n   :width: 4em\n   :height: 20\n", `\includegraphics[height=20bp,width=4em]{c.png}`},
		{"center is a makebox", ".. image:: p.png\n   :align: center\n", `\noindent\makebox[\linewidth][c]{\includegraphics{p.png}}`},
		{"right is an hfill before", ".. image:: p.png\n   :align: right\n", `\noindent{\hfill\includegraphics{p.png}}`},
		{"left is an hfill after", ".. image:: p.png\n   :align: left\n", `\noindent{\includegraphics{p.png}\hfill}`},
	}
	for _, c := range cases {
		got := render(t, c.src)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
	// \includegraphics comes from graphicx, so the preamble must name it --
	// otherwise every one of the cases above is a document that cannot compile.
	if got := render(t, ".. image:: a.png\n"); !strings.Contains(got, `\usepackage{graphicx}`) {
		t.Errorf("graphicx missing from the preamble: %s", got)
	}
}

// TestAnImageOnlyReferenceIsNotAnEmptyLink is how this defect first showed
// itself, before any probe was written for it: reading the compiled output of
// PEP's README turned up "\href{https://github.com/python/peps/actions}{}" --
// a link with NO clickable text, which is what a badge image inside a
// :target: reference became once the image was dropped.
func TestAnImageOnlyReferenceIsNotAnEmptyLink(t *testing.T) {
	got := render(t, ".. image:: badge.svg\n   :target: https://example.com/\n")
	if strings.Contains(got, `{}`) {
		t.Errorf("a reference whose only content is an image must not be an empty link: %s", got)
	}
	want := `\href{https://example.com/}{` + "\n" + `\includegraphics{badge.svg}`
	if !strings.Contains(got, want) {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// TestAFigureIsAFloat pins the figure structure, and the DU* expansions. Where
// latex2e emits one of its own macros this writer emits the FALLBACK
// DEFINITION docutils itself provides (PreambleCmds, read directly), since
// there is no preamble here to define one in:
//
//	\DUrubric{#1}  ->  \subsubsection*{\emph{#1}}
//	DUlegend       ->  {\small ...}
func TestAFigureIsAFloat(t *testing.T) {
	got := render(t, ".. figure:: f.png\n\n   The caption.\n\n   The legend.\n")
	for _, want := range []string{
		`\begin{figure}`,
		`\includegraphics{f.png}`,
		`\caption{The caption.}`,
		`{\small`,
		`\end{figure}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
	// \caption is only legal inside a float, so the order matters: the figure
	// must open before the caption appears.
	if strings.Index(got, `\begin{figure}`) > strings.Index(got, `\caption{`) {
		t.Errorf("\\caption before its float:\n%s", got)
	}
}

// TestARubricIsDocutilsOwnExpansion checks the expansion rather than a macro
// name, since that is the whole point of taking it from the reference.
func TestARubricIsDocutilsOwnExpansion(t *testing.T) {
	got := render(t, ".. rubric:: A rubric\n\nA paragraph.\n")
	if !strings.Contains(got, `\subsubsection*{\emph{A rubric}}`) {
		t.Errorf("want docutils' own \\DUrubric expansion in:\n%s", got)
	}
	if strings.Contains(got, "rubricA paragraph") {
		t.Errorf("the rubric ran into the next block:\n%s", got)
	}
}
