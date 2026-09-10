package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// directiveOptionSpec is each directive's own option_spec, transcribed
// from docutils' registry (the 43 entries of
// directives._directive_registry, read by importing each class and
// listing its option_spec keys). An option outside its directive's set
// is an ERROR naming it, and the directive produces nothing.
//
// Only the directives this package IMPLEMENTS are listed; an entry it
// has no semantics for never reaches a validator. A directive absent
// from this map is not validated at all, which is the previous
// behaviour and stays the default -- the map is the opt-in.
var directiveOptionSpec = map[string][]string{
	"admonition":     {"class", "name"},
	"attention":      {"class", "name"},
	"caution":        {"class", "name"},
	"code":           {"class", "name", "number-lines"},
	"compound":       {"class", "name"},
	"container":      {"name"},
	"danger":         {"class", "name"},
	"error":          {"class", "name"},
	"figure":         {"align", "alt", "class", "figclass", "figname", "figwidth", "height", "loading", "name", "scale", "target", "width"},
	"hint":           {"class", "name"},
	"image":          {"align", "alt", "class", "height", "loading", "name", "scale", "target", "width"},
	"important":      {"class", "name"},
	"line-block":     {"class", "name"},
	"list-table":     {"align", "class", "header-rows", "name", "stub-columns", "width", "widths"},
	"math":           {"class", "name"},
	"note":           {"class", "name"},
	"parsed-literal": {"class", "name"},
	"rubric":         {"class", "name"},
	"sidebar":        {"class", "name", "subtitle"},
	"table":          {"align", "class", "name", "width", "widths"},
	"tip":            {"class", "name"},
	"topic":          {"class", "name"},
	"warning":        {"class", "name"},
}

// unknownDirectiveOption returns the ERROR docutils raises for the FIRST
// option outside canonical's spec, or nil. optionLines must be only the
// directive's OPTION block: a directive's own CONTENT may start with
// something field-marker-shaped, and scanning past the options turns
// that into a rejected option (found the hard way in v0.96.0, where it
// cost two real-world files).
//
// written is the name AS SPELLED, since the message quotes it:
// ".. code-block::" says "code-block", not "code".
func unknownDirectiveOption(canonical, written string, optionLines []string, lineno int, blockText string) doctree.Node {
	allowed, ok := directiveOptionSpec[strings.ToLower(canonical)]
	if !ok {
		return nil
	}
	for _, l := range optionLines {
		key, _, ok := matchFieldMarker(l)
		if !ok {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(key))
		found := false
		for _, a := range allowed {
			if a == k {
				found = true
				break
			}
		}
		if !found {
			return sectionMessage("3", "ERROR",
				`Error in "`+written+`" directive:`+"\n"+`unknown option: "`+strings.TrimSpace(key)+`".`,
				lineno, blockText)
		}
	}
	return nil
}
