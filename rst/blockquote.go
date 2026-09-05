package rst

import (
	"github.com/go-docutils/docutils/doctree"
)

// parseBlockQuotes collects the whole indented run starting at lines[i] —
// docutils.parsers.rst.states.Body.indent + block_quote, ported. Returns
// every <block_quote> element the region splits into (see below) plus the
// index of the first line after it.
//
// A block quote's own indent is NOT known ahead of time the way a list
// item's or a field body's is (both have a marker whose column fixes it) —
// real docutils' own extraction (StringList.get_indented, read directly)
// collects EVERY line that is blank or indented at all relative to this
// context, computes the MINIMUM indent across all of them, and dedents
// every line by that minimum, not by the first line's own indent. That
// minimum-not-first split is what lets a deeper-indented sub-run end up
// NESTED once the dedented result is re-parsed (its residual indent is
// still positive), rather than cut off the moment a shallower line
// appears — this project's own consumeIndentedBlock (fixed-first-line
// indent, correct for a list/field body's KNOWN indent) previously stood
// in for this too, silently flattening a deeper-then-shallower run into
// sibling block_quotes instead of nesting the deeper one, a real,
// previously-undetected bug this function replaces it for.
//
// Real docutils' own block_quote() also splits the SAME collected region
// on an attribution line ("-- text" / "--- text" / an em-dash, followed by
// consistently-indented continuation lines) into MULTIPLE sibling
// block_quote elements, each optionally carrying a trailing <attribution>
// — ported into the while-loop below. Diagnostics real docutils emits for
// malformed shapes (an unindent with no blank line first, a too-short
// overline mistaken for an attribution attempt, ...) are deliberately NOT
// ported, the same scope boundary already applied to title-style
// consistency and table column-margin violations elsewhere in this
// parser — see the package SCOPE note.
func (p *parser) parseBlockQuotes(lines []string, i, lineBase int) ([]*doctree.Element, int) {
	indented, next := consumeIndentedRun(lines, i)
	var out []*doctree.Element
	// offset is how far into `indented` this iteration starts. Every entry
	// of `indented` corresponds one-for-one to lines[i+offset+k]
	// (consumeIndentedRun only dedents and trims TRAILING blanks, it never
	// drops a line in the middle), so a real absolute lineBase can be
	// handed down rather than the -1 "unknown" this used to pass — the
	// same derivation v0.44.0 made for topic/sidebar content, and the
	// reason a diagnostic raised INSIDE a block quote can now carry a
	// line number at all.
	offset := 0
	for len(indented) > 0 {
		bqLines, attrLines, remaining := splitAttribution(indented)
		bq := doctree.NewElement(doctree.TagBlockQuote)
		p.parseBlockLines(bqLines, bq, nestedLineBase(i+offset, lineBase))
		out = append(out, bq)
		if attrLines != nil {
			text := joinTrimmed(attrLines)
			attrNodes, attrMsgs := p.parseInline(text, 0)
			bq.Append(doctree.NewElement(doctree.TagAttribution, attrNodes...))
			// real docutils' Body.block_quote: "elements += messages" —
			// the attribution's own inline-markup messages are SIBLINGS of
			// the block_quote itself (states.py, read directly), not
			// children of the attribution.
			out = append(out, attrMsgs...)
		}
		offset += len(indented) - len(remaining)
		indented = remaining
		for len(indented) > 0 && isBlankStr(indented[0]) {
			indented = indented[1:]
			offset++
		}
	}
	// The same "ends without a blank line; unexpected unindent." warning
	// every other indented construct already carries — docutils raises it
	// from ONE shared RSTState.unindent_warning with nine call sites, of
	// which this was the last one still unported (only "Option list"
	// remains, and no corpus fixture reaches it). It fires when the
	// indented run ended because a non-blank UNINDENTED line followed
	// immediately, with no blank line between: unindent_warning reports
	// "one line below the current line", which is that offending line.
	if len(out) > 0 && next < len(lines) && !isBlankStr(lines[next]) && !isBlankStr(lines[next-1]) {
		out = append(out, sectionMessage("2", "WARNING",
			"Block quote ends without a blank line; unexpected unindent.",
			msgLine(next, lineBase), ""))
	}
	return out, next
}

// consumeIndentedRun collects every line from i that is blank or indented
// at all (leadingSpaces > 0), stopping at the first non-blank line with NO
// indentation, then dedents every collected line by the MINIMUM indent
// seen among the non-blank ones — docutils' StringList.get_indented.
func consumeIndentedRun(lines []string, i int) ([]string, int) {
	j := i
	minIndent := -1
	for j < len(lines) {
		if isBlankStr(lines[j]) {
			j++
			continue
		}
		ls := leadingSpaces(lines[j])
		if ls == 0 {
			break
		}
		if minIndent == -1 || ls < minIndent {
			minIndent = ls
		}
		j++
	}
	if minIndent == -1 {
		return nil, i
	}
	block := make([]string, 0, j-i)
	for k := i; k < j; k++ {
		if isBlankStr(lines[k]) {
			block = append(block, "")
			continue
		}
		block = append(block, lines[k][minIndent:])
	}
	for len(block) > 0 && isBlankStr(block[len(block)-1]) {
		block = block[:len(block)-1]
	}
	return block, j
}

// splitAttribution looks for the FIRST valid attribution boundary in an
// already-dedented indented block — docutils' split_attribution +
// check_attribution, ported. A boundary is: real content already seen,
// then a blank line, then a line matching attributionMarker, then zero or
// more further lines sharing ONE consistent indent up to the next blank
// or the end. Returns (block-quote lines, attribution lines, remaining
// lines-after-the-attribution) if found; attrLines is nil if no
// attribution boundary exists anywhere (the whole input is bqLines,
// remaining is nil).
func splitAttribution(indented []string) (bqLines, attrLines, remaining []string) {
	blank := -1
	nonblankSeen := false
	for i, raw := range indented {
		line := rtrim(raw)
		if line == "" {
			blank = i
			continue
		}
		if nonblankSeen && blank == i-1 {
			if contentStart, ok := matchAttributionMarker(line); ok {
				if end, indent, ok := checkAttributionShape(indented, i); ok {
					first := []rune(line)[contentStart:]
					a := make([]string, 0, end-i)
					a = append(a, string(first))
					for k := i + 1; k < end; k++ {
						l := indented[k]
						if len([]rune(l)) >= indent {
							a = append(a, string([]rune(l)[indent:]))
						} else {
							a = append(a, "")
						}
					}
					var rest []string
					if end < len(indented) {
						rest = indented[end:]
					}
					return indented[:i], a, rest
				}
			}
		}
		nonblankSeen = true
	}
	return indented, nil, nil
}

// checkAttributionShape verifies every line after the attribution's first
// (attributionStart) shares one consistent indent, up to the next blank
// line or the end of indented — docutils' check_attribution. ok is false
// for an inconsistent shape (not an attribution after all — the caller
// falls back to treating the marker line as ordinary content).
func checkAttributionShape(indented []string, attributionStart int) (end, indent int, ok bool) {
	indent = -1
	i := attributionStart + 1
	for ; i < len(indented); i++ {
		line := rtrim(indented[i])
		if line == "" {
			break
		}
		ls := leadingSpaces(indented[i])
		if indent == -1 {
			indent = ls
		} else if ls != indent {
			return 0, 0, false
		}
	}
	if indent == -1 {
		indent = 0
	}
	return i, indent, true
}

// matchAttributionMarker checks whether line starts with a block-quote
// attribution marker (two or three dashes — not one, not four or more —
// or a single em-dash) followed by zero or more spaces and then at least
// one more non-space character; contentStart is the rune index of that
// first content character. docutils' attribution_pattern
// `(---?(?!-)|—) *(?=[^ \n])`, ported without lookaround (Go's RE2
// backend has none): a run of exactly 2 or 3 dashes is the same
// constraint the "(?!-)" negative lookahead enforces (4+ dashes never
// matches at all, on either the 2- or 3-dash alternative, since Python's
// backtracking would fail both — verified against the foreign judge, not
// assumed, since getting this subtly wrong either way is exactly the kind
// of thing that looks right until a specific input proves otherwise).
func matchAttributionMarker(line string) (contentStart int, ok bool) {
	runes := []rune(line)
	i := 0
	if len(runes) > 0 && runes[0] == '—' {
		i = 1
	} else {
		n := 0
		for n < len(runes) && runes[n] == '-' {
			n++
		}
		if n != 2 && n != 3 {
			return 0, false
		}
		i = n
	}
	for i < len(runes) && runes[i] == ' ' {
		i++
	}
	if i >= len(runes) {
		return 0, false
	}
	return i, true
}

func rtrim(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}

func joinTrimmed(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return rtrimAll(out)
}

func rtrimAll(s string) string {
	i := len(s)
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\n') {
		i--
	}
	return s[:i]
}

// nestedLineBase shifts a lineBase down into a sub-slice starting at
// index start of the parent's own lines. An unknown parent base (-1)
// stays unknown: msgLine reports no line at all rather than a wrong one.
func nestedLineBase(start, lineBase int) int {
	if lineBase < 0 {
		return -1
	}
	return start + lineBase
}
