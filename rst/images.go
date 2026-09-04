package rst

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.images (read directly):
// "image" (a required URI argument, no content permitted) and "figure"
// (same argument + almost the same option set, PLUS a required-unless-
// content-empty caption/legend body). Figure.run literally calls
// Image.run on itself first (`(image_node,) = Image.run(self)`) after
// popping its own four figure-only options (figwidth/figclass/figname/
// align) — buildImageNode below is the shared core both entry points
// funnel through, mirroring that structure.
//
// SCOPE: the ":target:" option (wraps the image in a <reference>) is not
// implemented — no corpus case exercises it, and it pulls in
// parse_target's own hyperlink-target machinery for comparatively little
// value; likewise Image's separate vertical-values :align: branch used
// only when the directive is invoked INSIDE a substitution definition
// (align_v_values instead of align_h_values) is not implemented, since no
// corpus case combines :align: with an embedded substitution image
// either — every corpus case validates against align_h_values, the only
// branch built. Figure's own ":figwidth: image" auto-detection (reads
// the image file's actual pixel width via PIL) is not implemented: this
// project does no file I/O for a directive's own sake, matching its
// existing scope notes elsewhere — real docutils itself silently skips
// this too when PIL or file-insertion access isn't available, so this
// degrades the same already-documented way, not a shortcut invented here.
// Directive-level option/argument TYPE validation beyond :align:/
// :loading: (an unrecognized option name, an invalid :height:/:width:/
// :scale: value) is the same general, already-flagged gap noted
// elsewhere (see v0.24.0's own README scope note) — not built here
// either; a malformed length/percentage value is stored best-effort
// rather than rejected.

var imageAlignHValues = []string{"left", "center", "right"}
var imageLoadingValues = []string{"embed", "link", "lazy"}
var cssLengthUnits = []string{"em", "ex", "ch", "rem", "vw", "vh", "vmin", "vmax", "cm", "mm", "Q", "in", "pt", "pc", "px"}

// runImageDirective implements a bare ".. image::" invocation (top-level
// dispatch from parseDirective) — presetAlt is always "" here; only the
// substitution-embedded path (parseSubstitutionDef) ever passes a preset
// default (the substitution's own name, matching run_directive's own
// "alt" option_preset, states.py, read directly).
func (p *parser) runImageDirective(lines []string, i, next int, args string, body []string, presetAlt string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	combined := make([]string, 0, 1+blanks+len(body))
	combined = append(combined, args)
	for k := 0; k < blanks; k++ {
		combined = append(combined, "")
	}
	combined = append(combined, body...)
	argument, options, content := parseDirectiveBlock(combined, true)
	return finishImageDirective("image", argument, options, content, presetAlt, lineno, blockText)
}

// finishImageDirective is the shared tail of an already-split image
// invocation — reused by runImageDirective (which derives argument/
// options/content itself from lines/i) and by parseSubstitutionDef's own
// embedded-directive path (which derives them from the substitution's
// own already-dedented, blank-preserved body directly, with no
// lines/i re-derivation needed — see its own doc comment).
func finishImageDirective(directiveName, argument string, options map[string]string, content []string, presetAlt string, lineno int, blockText string) []doctree.Node {
	if argument == "" {
		return []doctree.Node{directiveError(directiveName, "1 argument(s) required, 0 supplied", lineno, blockText)}
	}
	if len(content) > 0 && !allBlank(content) {
		return []doctree.Node{directiveError(directiveName, "no content permitted", lineno, blockText)}
	}
	img, errEl := buildImageNode(directiveName, argument, options, presetAlt, lineno, blockText)
	if errEl != nil {
		return []doctree.Node{errEl}
	}
	return []doctree.Node{img}
}

// buildImageNode builds an <image> element from an argument+options pair
// already split by parseDirectiveBlock — the core Image.run logic shared
// by a bare image directive and Figure.run's own delegation to it.
func buildImageNode(directiveName, argument string, options map[string]string, presetAlt string, lineno int, blockText string) (*doctree.Element, *doctree.Element) {
	el := doctree.NewElement(doctree.TagImage)
	el.SetAttr("uri", imageURI(argument))
	if v, ok := options["alt"]; ok {
		el.SetAttr("alt", v)
	} else if presetAlt != "" {
		el.SetAttr("alt", presetAlt)
	}
	if v, ok := options["height"]; ok {
		if m, valid := formatMeasure(v, false, ""); valid {
			el.SetAttr("height", m)
		}
	}
	if v, ok := options["width"]; ok {
		if m, valid := formatMeasure(v, true, ""); valid {
			el.SetAttr("width", m)
		}
	}
	if v, ok := options["scale"]; ok {
		if m, valid := formatPercentage(v); valid {
			el.SetAttr("scale", m)
		}
	}
	if v, ok := options["align"]; ok {
		chosen, valid := choiceValue(v, imageAlignHValues)
		if !valid {
			return nil, directiveError(directiveName, formatChoiceError("align", v, imageAlignHValues), lineno, blockText)
		}
		el.SetAttr("align", chosen)
	}
	if v, ok := options["loading"]; ok {
		chosen, valid := choiceValue(v, imageLoadingValues)
		if !valid {
			return nil, directiveError(directiveName, formatChoiceError("loading", v, imageLoadingValues), lineno, blockText)
		}
		el.SetAttr("loading", chosen)
	}
	if v, ok := options["class"]; ok {
		el.SetAttr("class", strings.Join(classOption(v), " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", makeID(name))
	}
	return el, nil
}

// imageURI mirrors directives.uri's "unescaped whitespace removed"
// behavior for the single-token case every corpus fixture actually
// exercises (no space, no backslash escape) — a real escaped-whitespace
// URI is out of scope, matching this function's own single-purpose name.
func imageURI(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func isCSSLengthUnit(u string) bool {
	for _, x := range cssLengthUnits {
		if u == x {
			return true
		}
	}
	return false
}

// formatMeasure mirrors nodes.parse_measure + directives.get_measure: a
// leading (optionally decimal, optionally negative) number followed by an
// optional unit. allowPercent widens the accepted unit set to include
// "%" (width/figwidth, unlike height); default, when non-empty, is
// appended when the value has NO unit at all (figwidth's own 'px'
// default — length_or_percentage_or_unitless(argument, 'px')).
func formatMeasure(s string, allowPercent bool, defaultUnit string) (string, bool) {
	s = strings.TrimSpace(s)
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	start := i
	sawDigit := false
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
		if s[i] != '.' {
			sawDigit = true
		}
		i++
	}
	if i == start || !sawDigit {
		return "", false
	}
	numPart := s[:i]
	unit := strings.TrimSpace(s[i:])
	if unit != "" && !isCSSLengthUnit(unit) && !(allowPercent && unit == "%") {
		return "", false
	}
	if unit == "" && defaultUnit != "" {
		unit = defaultUnit
	}
	return numPart + unit, true
}

// formatPercentage mirrors directives.percentage: an integer, an optional
// trailing "%" (and surrounding space) stripped before parsing — the
// returned string never carries a "%" itself, matching the corpus's own
// scale="50" (not "50%").
func formatPercentage(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return "", false
	}
	return strconv.Itoa(n), true
}

// choiceValue mirrors directives.choice: case-insensitive, trimmed
// membership check against a fixed value set.
func choiceValue(raw string, values []string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, x := range values {
		if v == x {
			return v, true
		}
	}
	return "", false
}

// formatValues mirrors directives.format_values: an Oxford-comma,
// double-quoted list ('"left", "center", or "right"').
func formatValues(values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return `"` + values[0] + `"`
	}
	quoted := make([]string, len(values)-1)
	for i, v := range values[:len(values)-1] {
		quoted[i] = `"` + v + `"`
	}
	return strings.Join(quoted, ", ") + `, or "` + values[len(values)-1] + `"`
}

// formatChoiceError mirrors assemble_option_dict's own wrapping of a
// ValueError from a choice() conversion function: "invalid option
// value: (option: "%s"; value: %r)\n%s" — %r of a plain string is
// single-quoted, matching Python's repr() for ASCII text with no quote
// characters of its own (every corpus value this project sees).
func formatChoiceError(optionName, value string, values []string) string {
	return `invalid option value: (option: "` + optionName + `"; value: '` + value + `')` + "\n" +
		`"` + value + `" unknown; choose from ` + formatValues(values)
}

// directiveError mirrors run_directive's own MarkupError wrapping:
// 'Error in "%s" directive:\n%s.' — detail carries no trailing period of
// its own; this appends the one and only period, even when detail spans
// multiple lines (see formatChoiceError).
func directiveError(directiveName, detail string, lineno int, blockText string) *doctree.Element {
	return sectionMessage("3", "ERROR", "Error in \""+directiveName+"\" directive:\n"+detail+".", lineno, blockText)
}

// runFigureDirective implements Figure.run: pop the four figure-only
// options before delegating to buildImageNode for the rest (matching
// Image.option_spec.copy() plus figwidth/figclass/figname/align), wrap
// the resulting <image> in a <figure>, then — when there's a content
// body — extract an optional caption (its first paragraph) and legend
// (everything after) from it, matching real docutils' own nested-parse-
// then-classify loop exactly (states.py's Figure.run, read directly): a
// <target>/<pending> child is passed straight through as a <figure>
// child in place (a hyperlink target or a deferred-transform node placed
// ahead of the caption); the first <paragraph> becomes the <caption>; an
// empty <comment> suppresses the caption entirely; anything else in that
// position is a real ERROR ("Figure caption must be a paragraph or empty
// comment"), discarding whatever would have become the legend. This
// project has no <pending> node at all (no transform system — see the
// package's own scope notes elsewhere), so a content body opening with
// something like ".. class:: custom" before its caption is not
// reproduced byte-for-byte; the <target> pass-through alone still works.
func (p *parser) runFigureDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	combined := make([]string, 0, 1+blanks+len(body))
	combined = append(combined, args)
	for k := 0; k < blanks; k++ {
		combined = append(combined, "")
	}
	combined = append(combined, body...)
	argument, options, content := parseDirectiveBlock(combined, true)
	if argument == "" {
		return []doctree.Node{directiveError("figure", "1 argument(s) required, 0 supplied", lineno, blockText)}
	}

	figwidth := options["figwidth"]
	figclass := options["figclass"]
	figname := options["figname"]
	align := options["align"]
	delete(options, "figwidth")
	delete(options, "figclass")
	delete(options, "figname")
	delete(options, "align")

	imgEl, errEl := buildImageNode("figure", argument, options, "", lineno, blockText)
	if errEl != nil {
		return []doctree.Node{errEl}
	}

	figureEl := doctree.NewElement(doctree.TagFigure)
	figureEl.Append(imgEl)

	if figwidth != "" {
		if !strings.EqualFold(figwidth, "image") {
			if m, valid := formatMeasure(figwidth, true, "px"); valid {
				figureEl.SetAttr("width", m)
			}
		}
		// figwidth == "image": would need reading the actual image file's
		// pixel width (real docutils via PIL) — silently skipped, see the
		// package doc comment above.
	}
	if figclass != "" {
		figureEl.SetAttr("class", strings.Join(classOption(figclass), " "))
	}
	if figname != "" {
		name := normalizeName(figname)
		figureEl.SetAttr("name", name)
		figureEl.SetAttr("id", makeID(name))
	}
	if align != "" {
		chosen, valid := choiceValue(align, imageAlignHValues)
		if !valid {
			return []doctree.Node{directiveError("figure", formatChoiceError("align", align, imageAlignHValues), lineno, blockText)}
		}
		figureEl.SetAttr("align", chosen)
	}

	if len(content) > 0 && !allBlank(content) {
		tmp := doctree.NewElement("")
		p.parseBlockLines(content, tmp, -1)
		captionIdx := -1
		for j, c := range tmp.Children {
			ce, ok := c.(*doctree.Element)
			if !ok {
				return []doctree.Node{figureEl, sectionMessage("3", "ERROR",
					"Figure caption must be a paragraph or empty comment.", lineno, blockText)}
			}
			if ce.Tag == doctree.TagTarget {
				figureEl.Append(ce)
				continue
			}
			if ce.Tag == doctree.TagParagraph {
				figureEl.Append(doctree.NewElement(doctree.TagCaption, ce.Children...))
				captionIdx = j
				break
			}
			if ce.Tag == doctree.TagComment && len(ce.Children) == 0 {
				captionIdx = j
				break
			}
			return []doctree.Node{figureEl, sectionMessage("3", "ERROR",
				"Figure caption must be a paragraph or empty comment.", lineno, blockText)}
		}
		if captionIdx >= 0 && captionIdx+1 < len(tmp.Children) {
			figureEl.Append(doctree.NewElement(doctree.TagLegend, tmp.Children[captionIdx+1:]...))
		}
	}

	return []doctree.Node{figureEl}
}
