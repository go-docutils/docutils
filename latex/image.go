package latex

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file gives the latex writer the elements it had NO case for and so
// rendered through "default: renderChildren" -- see html/image.go for the same
// gap on the other side. An <image> has no children, so it came out as
// NOTHING: 130 images in 47 of the 1564 real-world corpus files, invisible to
// a content probe because an image carries no text to miss. It also explains
// an "\href{https://.../actions}{}" in the output -- a link with no clickable
// text at all, which is what a badge image inside a reference became.
//
// Everything here follows docutils' latex2e writer (read directly). Where that
// writer emits one of its own DU* macros, this one emits the FALLBACK
// DEFINITION docutils itself provides for it (PreambleCmds, read directly),
// since this writer has no preamble to put a macro in:
//
//	\DUrubric{#1}  ->  \subsubsection*{\emph{#1}}
//	DUlegend       ->  {\small ...}
//
// Taking docutils' own expansion rather than inventing one is the difference
// between following the reference and guessing at it.

// renderImage writes visit_image's own \includegraphics, with the alignment
// wrapper latex2e puts around a block-level image.
func renderImage(b *strings.Builder, el *doctree.Element) {
	opts := imageOptions(el)
	inc := "\\includegraphics"
	if opts != "" {
		inc += "[" + opts + "]"
	}
	inc += "{" + el.Attr("uri") + "}"
	// align_codes, visit_image (read directly). Only the three block-level
	// alignments are reachable from a block image; top/middle/bottom are the
	// inline ones and use \raisebox, which this writer does not emit because
	// it has no way to know it is inline.
	switch el.Attr("align") {
	case "center":
		b.WriteString("\n\\noindent\\makebox[\\linewidth][c]{" + inc + "}\n")
	case "left":
		b.WriteString("\n\\noindent{" + inc + "\\hfill}\n")
	case "right":
		b.WriteString("\n\\noindent{\\hfill" + inc + "}\n")
	default:
		b.WriteString("\n" + inc + "\n")
	}
}

// imageOptions builds \includegraphics' own key list the way latex2e does.
// Three details that only reading it gives, and that the HTML side does NOT
// share: a UNITLESS measure is in big points ("200" -> "width=200bp", not
// "width=200"), a PERCENTAGE is relative to \linewidth ("50%" ->
// "width=0.5\linewidth"), and :scale: is passed to graphicx as its own
// "scale=" key rather than multiplied into the measures -- which is why the
// reference prints "scale=0.5,width=0.5\linewidth" for an image carrying both.
// The keys come out in latex2e's own order: height, then scale, then width.
func imageOptions(el *doctree.Element) string {
	var keys []string
	if h := latexMeasure(el.Attr("height"), false); h != "" {
		keys = append(keys, "height="+h)
	}
	if s := el.Attr("scale"); s != "" {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
			keys = append(keys, "scale="+strconv.FormatFloat(n/100, 'g', -1, 64))
		}
	}
	if w := latexMeasure(el.Attr("width"), true); w != "" {
		keys = append(keys, "width="+w)
	}
	return strings.Join(keys, ",")
}

// latexMeasure turns a doctree measure into a graphicx length. relative says
// whether a percentage is a fraction of \linewidth (a width) or of
// \textheight (a height) -- latex2e uses \linewidth for width and, for a
// percentage height, leaves it to \textheight the same way.
func latexMeasure(v string, relative bool) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	num, unit := splitMeasure(v)
	if num == "" {
		return ""
	}
	switch unit {
	case "":
		// A bare number is TeX big points, not a unitless length: a length
		// with no unit is not a length at all and \includegraphics would
		// reject it.
		return num + "bp"
	case "%":
		f, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return ""
		}
		base := "\\textheight"
		if relative {
			base = "\\linewidth"
		}
		return strconv.FormatFloat(f/100, 'g', -1, 64) + base
	default:
		return num + unit
	}
}

func splitMeasure(s string) (num, unit string) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '-' || s[i] == '+') {
		i++
	}
	return s[:i], strings.TrimSpace(s[i:])
}
