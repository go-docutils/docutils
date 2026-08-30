package rst

import (
	"strconv"

	"github.com/go-docutils/docutils/doctree"
)

// footnoteSymbols is docutils' own fixed sequence for "[*]_"-style
// footnotes (docutils/transforms/references.py's Footnotes.symbols: the
// first five are Chicago Manual of Style 14th ed. §12.51, the rest chosen
// arbitrarily by docutils itself).
var footnoteSymbols = []string{
	"*", "†", "‡", "§", "¶",
	"#", "♠", "♥", "♦", "♣",
}

// footnoteSymbolLabel returns the label for the index-th (0-based) symbol
// footnote: the base sequence, then doubled/tripled/... once it wraps
// ("**", "††", ... on the second pass), matching docutils'
// symbolize_footnotes (divmod(index, len(symbols))).
func footnoteSymbolLabel(index int) string {
	reps, i := index/len(footnoteSymbols), index%len(footnoteSymbols)
	s := ""
	for j := 0; j <= reps; j++ {
		s += footnoteSymbols[i]
	}
	return s
}

// resolveFootnoteNumbers assigns labels to auto-numbered ("[#]_") and
// symbol ("[*]_") footnotes and their references — docutils' Footnotes
// transform, drastically simplified (no "too many references"
// diagnostics; citations are excluded, since they are never auto — see
// parseFootnoteOrCitation). Two independent sequences, matching real
// docutils: numeric labels count every auto footnote in DEFINITION
// document order, named and unnamed sharing one counter; symbol labels
// count only "[*]_"-style footnotes (always unnamed), in their own
// separate sequence.
//
// A footnote already named ("[#realname]_") keeps that name, and its
// reference(s) resolve by it, same as any other named construct. An
// UNNAMED one (bare "[#]_" or "[*]_") gets a synthetic name
// ("footnote-N") so its reference can resolve the same refname-based way
// every writer already understands — matched POSITIONALLY within its own
// numeric-or-symbol pool: the Nth unnamed reference gets the Nth unnamed
// definition's synthetic name and label, textual order (verified against
// real docutils: this holds regardless of whether definitions appear
// before or after their references, same as anonymous targets).
func resolveFootnoteNumbers(doc *doctree.Element) {
	var numericDefs, symbolDefs []*doctree.Element
	collectAutoFootnoteDefs(doc, &numericDefs, &symbolDefs)
	usedNumbers := collectExplicitFootnoteNumbers(doc)

	labels := map[string]string{} // synthetic-or-real name -> assigned label
	var pendingNumeric, pendingSymbol []string
	synthetic := 0
	nextSynthetic := func() string {
		synthetic++
		return "footnote-" + strconv.Itoa(synthetic)
	}

	nextNum := 1
	for _, def := range numericDefs {
		for usedNumbers[strconv.Itoa(nextNum)] {
			nextNum++
		}
		label := strconv.Itoa(nextNum)
		nextNum++
		prependLabel(def, label)
		name := def.Attr("name")
		if name == "" {
			name = nextSynthetic()
			def.SetAttr("name", name)
			pendingNumeric = append(pendingNumeric, name)
		}
		labels[name] = label
	}
	for i, def := range symbolDefs {
		label := footnoteSymbolLabel(i)
		prependLabel(def, label)
		name := nextSynthetic()
		def.SetAttr("name", name)
		pendingSymbol = append(pendingSymbol, name)
		labels[name] = label
	}

	numericIdx, symbolIdx := 0, 0
	assignAutoFootnoteRefs(doc, labels, pendingNumeric, pendingSymbol, &numericIdx, &symbolIdx)
}

// collectExplicitFootnoteNumbers walks the tree collecting the digit
// labels of explicitly-numbered footnotes ("[1]_" — the only TagFootnote
// shape with no "auto" attr, see parseFootnoteOrCitation's isAllDigits
// branch), so the auto-numbering sequence below can skip any number
// already spoken for. Matches real docutils' own number_footnotes, which
// re-rolls a candidate label already present in document.nameids.
func collectExplicitFootnoteNumbers(n doctree.Node) map[string]bool {
	used := map[string]bool{}
	var walk func(doctree.Node)
	walk = func(n doctree.Node) {
		el, ok := n.(*doctree.Element)
		if !ok {
			return
		}
		if el.Tag == doctree.TagFootnote && el.Attr("auto") == "" {
			used[el.Attr("name")] = true
		}
		for _, c := range el.Children {
			walk(c)
		}
	}
	walk(n)
	return used
}

// collectAutoFootnoteDefs walks the tree collecting footnote definitions
// (never citations, which are never auto) with auto="1" or auto="*", in
// document order, into two separate lists.
func collectAutoFootnoteDefs(n doctree.Node, numeric, symbol *[]*doctree.Element) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagFootnote {
		switch el.Attr("auto") {
		case "1":
			*numeric = append(*numeric, el)
		case "*":
			*symbol = append(*symbol, el)
		}
	}
	for _, c := range el.Children {
		collectAutoFootnoteDefs(c, numeric, symbol)
	}
}

// prependLabel inserts a <label> holding text as el's FIRST child,
// matching where an explicit numeric/citation label already sits (see
// parseFootnoteOrCitation) — an auto footnote never has one yet, since
// only those two cases append one during parsing.
func prependLabel(el *doctree.Element, text string) {
	label := doctree.NewElement(doctree.TagLabel, &doctree.Text{Data: text})
	el.Children = append([]doctree.Node{label}, el.Children...)
}

// assignAutoFootnoteRefs walks the tree in document order; for each
// auto="1"/auto="*" footnote_reference with no refname of its own (an
// unnamed "[#]_"/"[*]_"), it consumes the next pending synthetic name from
// the matching pool and sets refname; either way (already-named or just
// assigned), it appends the resolved label as the reference's own visible
// text, matching real docutils' `ref += nodes.Text(label)`. A reference
// past the end of its pool (more unnamed refs than definitions) is simply
// left unresolved, same as any other unmatched reference.
func assignAutoFootnoteRefs(n doctree.Node, labels map[string]string, pendingNumeric, pendingSymbol []string, numericIdx, symbolIdx *int) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagFootnoteReference {
		switch el.Attr("auto") {
		case "1":
			name := el.Attr("refname")
			if name == "" {
				if *numericIdx < len(pendingNumeric) {
					name = pendingNumeric[*numericIdx]
					el.SetAttr("refname", name)
					*numericIdx++
				}
			}
			if label, ok := labels[name]; ok {
				el.Append(&doctree.Text{Data: label})
			}
		case "*":
			if *symbolIdx < len(pendingSymbol) {
				name := pendingSymbol[*symbolIdx]
				el.SetAttr("refname", name)
				*symbolIdx++
				el.Append(&doctree.Text{Data: labels[name]})
			}
		}
	}
	for _, c := range el.Children {
		assignAutoFootnoteRefs(c, labels, pendingNumeric, pendingSymbol, numericIdx, symbolIdx)
	}
}
