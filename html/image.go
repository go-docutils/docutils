package html

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file gives the html writer the elements it had NO case for, and so
// rendered through "default: renderChildren". For most of those the text
// survived and only the container was lost -- which a content probe cannot see
// and which is a presentation question. For these it was not:
//
//   <image>   has no children at all, so it rendered as NOTHING. 130 of them
//             in 47 of the 1564 real-world corpus files, and a content probe
//             is blind to every one, an image carrying no text to miss.
//   <figure>  lost its image the same way.
//   <caption> and <rubric> hold INLINE children, so they rendered with no
//             element around them at all and their words ran into the next
//             block's: ".. rubric:: A rubric" before a paragraph came out as
//             "A rubricAn image inline:". The content probe is
//             whitespace-insensitive, so it cannot see that either.
//
// Tag choices come from docutils' _html_base.HTMLTranslator (read directly),
// within this package's own scope: no CSS classes, so docutils'
// class="align-right" on an image and class="legend" on a legend are NOT
// emitted, and the alignment they express is lost. That is a presentational
// attribute; the image itself is not.

// writeImage renders an <image> as visit_image does: alt DEFAULTS TO THE URI
// (the writer's own "alt = node.get('alt', uri)", not something the parser
// puts there), and the size becomes either plain width/height attributes or a
// style declaration depending on whether the measure carried a unit.
func writeImage(b *strings.Builder, el *doctree.Element) {
	uri := el.Attr("uri")
	alt := el.Attr("alt")
	if alt == "" {
		alt = uri
	}
	atts := map[string]string{"alt": alt, "src": uri}
	if l := el.Attr("loading"); l == "lazy" {
		atts["loading"] = "lazy"
	}
	for k, v := range imageSize(el) {
		atts[k] = v
	}
	keys := make([]string, 0, len(atts))
	for k := range atts {
		keys = append(keys, k)
	}
	// docutils' starttag emits its attributes sorted, and following that
	// keeps a diff against the reference readable.
	sort.Strings(keys)
	b.WriteString("<img")
	for _, k := range keys {
		b.WriteString(" " + k + `="` + escapeAttr(atts[k]) + `"`)
	}
	b.WriteString(" />")
}

// imageSize ports HTMLTranslator.image_size (read directly): a measure with a
// UNIT becomes a style declaration, a unitless one becomes the plain
// attribute, and :scale: multiplies whichever measures were declared.
//
// One branch is deliberately not ported. When :scale: is given and FEWER THAN
// TWO measures are, docutils reads the missing dimension out of the image FILE
// (read_size_with_PIL). This writer is handed a doctree and nothing else -- it
// has no path to resolve, no filesystem to resolve it against, and no image
// decoder -- so a lone :scale: produces no size attributes here. Named rather
// than approximated: guessing a pixel size would be worse than omitting it.
func imageSize(el *doctree.Element) map[string]string {
	type measure struct {
		value float64
		unit  string
	}
	measures := map[string]measure{}
	for _, dim := range []string{"width", "height"} {
		if v := el.Attr(dim); v != "" {
			if n, u, ok := parseMeasure(v); ok {
				measures[dim] = measure{n, u}
			}
		}
	}
	factor := 1.0
	if s := el.Attr("scale"); s != "" {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
			factor = n / 100
		}
	}
	out := map[string]string{}
	var declarations []string
	// width before height, so the style attribute reads in the order
	// docutils' own dimensions tuple does.
	for _, dim := range []string{"width", "height"} {
		m, ok := measures[dim]
		if !ok {
			continue
		}
		v := m.value * factor
		if m.unit != "" {
			declarations = append(declarations, dim+": "+formatG(v)+m.unit+";")
		} else {
			out[dim] = strconv.FormatInt(int64(roundHalfEven(v)), 10)
		}
	}
	if len(declarations) > 0 {
		out["style"] = strings.Join(declarations, " ")
	}
	return out
}

// parseMeasure splits "200px" into 200 and "px", "50%" into 50 and "%", and
// "200" into 200 and "". The parser has already validated the spelling
// (rst.formatMeasure), so this only has to split it.
func parseMeasure(s string) (float64, string, bool) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '-' || s[i] == '+') {
		i++
	}
	n, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[i:]), true
}

// formatG matches Python's "%g" for these values: no trailing zeros, and an
// integral value prints without a decimal point.
func formatG(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// roundHalfEven is Python's round() for a unitless size, which rounds a tie to
// the EVEN integer rather than away from zero -- so a scaled 2.5 is 2, not 3.
func roundHalfEven(v float64) float64 {
	f := float64(int64(v))
	d := v - f
	switch {
	case d > 0.5:
		return f + 1
	case d < 0.5:
		return f
	default:
		if int64(f)%2 == 0 {
			return f
		}
		return f + 1
	}
}

// writeFigure renders a <figure>, opening a <figcaption> around the caption
// and legend the way html5_polyglot does: the caption opens it (visit_caption,
// "if isinstance(node.parent, nodes.figure)"), or the legend does when there is
// no caption before it (visit_legend, "if not isinstance(node.previous_sibling
// (), nodes.caption)"), and depart_figure closes it. Doing it here rather than
// in the caption's own case is what lets one <figcaption> span both.
func writeFigure(b *strings.Builder, el *doctree.Element, headingLevel int) {
	attrs := ""
	if w := el.Attr("width"); w != "" {
		attrs = ` style="width: ` + escapeAttr(w) + `"`
	}
	b.WriteString("<figure" + attrs + ">")
	open := false
	for _, c := range el.Children {
		if ce, ok := c.(*doctree.Element); ok {
			if (ce.Tag == doctree.TagCaption || ce.Tag == doctree.TagLegend) && !open {
				b.WriteString("<figcaption>")
				open = true
			}
		}
		renderNode(b, c, headingLevel)
	}
	if open {
		b.WriteString("</figcaption>")
	}
	b.WriteString("</figure>")
}
