package rst

import (
	"regexp"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Field lists (":name: body", used pervasively for directive options and
// docstring-style parameter docs), modeled on docutils'
// Body.field_marker/field/parse_field_marker (states.py). Option lists
// (docutils' Body.option_marker, "-f, --file=ARG  Description.") were
// deferred from here initially — their marker grammar (comma-separated
// groups with a fiddly delimiter/argument-joining algorithm) is complex
// relative to how rarely they appear outside man-page-style CLI docs — but
// are now implemented in optionlist.go, once field/definition lists and
// tables had proven out the shared marker+indented-continuation machinery
// (gatherListItemLines) option lists reuse directly.

// matchFieldMarker recognizes ":name: rest" at the start of a line, docutils'
// own field_marker pattern (states.py, read directly):
//
//	:(?![: ])([^:\\]|\\.|:(?!([ `]|$)))*(?<! ):( +|$)
//
// A leading ':' not immediately followed by ':' or ' ', then the name runs
// up to the next ':' that is followed by a space or EOL (success) OR a
// backtick — an embedded ':' followed by ANYTHING ELSE (not space/EOL/
// backtick) is silently part of the name, and scanning continues past it;
// this is NOT the same as "skip ahead to the next later colon that fits" —
// a colon immediately followed by a backtick fails the WHOLE match right
// there, it is never treated as ordinary name content to be skipped over.
// Without that distinction, a prefix-form interpreted-text role right at
// the start of a line (":title:`x`") gets misread as a field marker whose
// "name" swallows the backtick and keeps hunting for a later matching
// colon — exactly the corpus failure this was caught from (verified
// against the foreign judge, not assumed from the regex alone).
//
// Two more pieces of the SAME pattern, both corpus-verified this round
// (test_field_lists.py's own edge-case fixture): "\\." (a backslash
// followed by ANY character, consumed as ONE atomic unit) means an
// escaped colon — “ \: “ — is ALWAYS name content, never a candidate
// close at all, regardless of what follows it; this needs its own check
// since the loop below would otherwise reach the escaped colon on its
// very next character and mistake it for a real one. And "(?<! )" — a
// space immediately before the closing colon disqualifies THIS colon as
// a close; since neither remaining alternative in the pattern can then
// consume it either (a colon followed by a space can't be "ordinary
// name content" — that alternative explicitly excludes a colon followed
// by space/backtick/EOL), the whole match fails outright, matching the
// backtick case just below it — not "keep scanning for a later colon,"
// since nothing in the pattern lets this character be skipped over.
func matchFieldMarker(line string) (name string, contentCol int, ok bool) {
	if len(line) < 2 || line[0] != ':' || line[1] == ':' || line[1] == ' ' {
		return "", 0, false
	}
	for j := 1; j < len(line); j++ {
		if line[j] == '\\' && j+1 < len(line) {
			j++ // the loop's own j++ advances past the escaped character too
			continue
		}
		if line[j] != ':' {
			continue
		}
		if j+1 == len(line) {
			if line[j-1] == ' ' {
				return "", 0, false
			}
			return line[1:j], j + 1, true
		}
		switch line[j+1] {
		case ' ':
			if line[j-1] == ' ' {
				return "", 0, false
			}
			return line[1:j], j + 2, true
		case '`':
			return "", 0, false
		}
	}
	return "", 0, false
}

// parseFieldList returns the built <field_list> plus, when the list is
// interrupted by a non-blank line that isn't itself a new field marker,
// a sibling "Field list ends without a blank line; unexpected
// unindent." WARNING — the same shared "chain a whole run of items
// through one nested parse, warn once at the very end" shape
// definition lists (v0.38.0), footnotes/citations (v0.35.0), and line
// blocks (v0.37.0) already have; unlike line_block's own first-line-
// based warning, this one uses the standard unindent_warning position
// (the line right after the list's own last consumed content),
// matching real docutils exactly.
//
// lineBase mirrors every other line-scanning function in this package —
// see consumeParagraph's own doc comment — so a field name's own
// inline-markup diagnostics carry a real absolute line number when
// reached from top-level context (previously always 0/unknown).
func (p *parser) parseFieldList(lines []string, i, lineBase int) (*doctree.Element, []*doctree.Element, int) {
	fl := doctree.NewElement(doctree.TagFieldList)
	bodyNext := i
	for i < len(lines) {
		name, col, ok := matchFieldMarker(lines[i])
		if !ok {
			break
		}
		first := ""
		if len(lines[i]) > col {
			first = lines[i][col:]
		}
		bodyLines, next := gatherListItemLines(lines, i, col, first)
		bodyNext = next
		field := doctree.NewElement(doctree.TagField)
		nameLineno := 0
		if lineBase >= 0 {
			nameLineno = i + lineBase + 1
		}
		nameNodes, nameMsgs := p.parseInline(name, nameLineno)
		field.Append(doctree.NewElement(doctree.TagFieldName, nameNodes...))
		body := doctree.NewElement(doctree.TagFieldBody)
		// real docutils' Body.field: "field_body = nodes.field_body(...,
		// *name_messages)" — the field NAME's own inline-markup messages
		// become the field_body's FIRST children, ahead of its parsed
		// content (states.py, read directly), not the field_name's own.
		for _, m := range nameMsgs {
			body.Append(m)
		}
		p.parseBlockLines(bodyLines, body)
		field.Append(body)
		fl.Append(field)
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	var messages []*doctree.Element
	if len(fl.Children) > 0 && !(bodyNext >= len(lines) || isBlankStr(lines[bodyNext-1])) {
		messages = append(messages, sectionMessage("2", "WARNING",
			"Field list ends without a blank line; unexpected unindent.",
			msgLine(bodyNext, lineBase), ""))
	}
	return fl, messages, i
}

// isDefinitionTermLine reports whether lines[i] opens a definition list
// item: an ordinary text line (not a bullet/enum/explicit-markup/
// title/transition line) immediately followed — no blank line — by an
// indented line. docutils decides this dynamically via its Text state
// examining exactly the second line (RSTState.Text.indent); this checks
// both lines directly since this parser is not state-machine-shaped.
func isDefinitionTermLine(lines []string, i int) bool {
	if isBlankStr(lines[i]) || leadingSpaces(lines[i]) != 0 {
		return false
	}
	// isEnumListStart, not the bare shape check isEnumLine: a line that
	// merely LOOKS enumerator-shaped but isn't a semantically valid one
	// (a malformed roman numeral like "iiii.", roman-charset letters
	// with no valid subtractive form) must still be free to open a
	// definition list term — real docutils' own Body.enumerator would
	// likewise fall through past it (is_enumerated_list_item requires a
	// valid ordinal), letting the SAME line reach Text-state definition-
	// list detection next.
	if isBulletLine(lines[i]) || isEnumListStart(lines, i) || isExplicitMarkupLine(lines[i]) {
		return false
	}
	if _, _, ok := matchFieldMarker(lines[i]); ok {
		return false
	}
	if isDoctestLine(lines[i]) || isLineBlockLine(lines[i]) {
		return false
	}
	// Only a line of at least 4 repeated characters is excluded as a
	// potential transition/title marker instead — shorter ("==", "--") is
	// ordinary text once too short to be either (see matchTitle/
	// consumeParagraph's own >=4 threshold, states.py's Body.line "elif
	// len(match.string.strip()) < 4" read directly), so it can still start
	// a definition list term like any other text line.
	if _, isLine := isUniformLine(lines[i]); isLine && len([]rune(trimTrailingSpace(lines[i]))) >= 4 {
		return false
	}
	return i+1 < len(lines) && !isBlankStr(lines[i+1]) && leadingSpaces(lines[i+1]) > 0
}

// classifierDelimiter is docutils' own Text.classifier_delimiter
// (states.py, read directly): one-or-more spaces, a colon, one-or-more
// spaces — the ONLY thing that turns a definition list term into a
// term-plus-classifier(s) shape ("term : classifier"). Deliberately
// requires real, unescaped whitespace on both sides, so an ordinary
// colon inside a term (a URL, a time, "a:b" with no surrounding space)
// is never mistaken for the delimiter.
var classifierDelimiter = regexp.MustCompile(` +: +`)

// splitTermClassifiers ports Text.term (states.py, read directly):
// termNodes is the term line's OWN already-inline-parsed content (so
// any embedded markup — a reference, an unclosed-markup problematic —
// is already resolved into real nodes, never re-examined for the
// delimiter itself); only a *doctree.Text run gets split by
// classifierDelimiter, with the FIRST resulting piece appended to
// whatever the CURRENT last element of the returned list is (the term
// itself, or the most recently opened classifier), and each subsequent
// piece opening a NEW <classifier> — a non-Text node (markup) is always
// appended to the CURRENT last element too, never inspected for a
// delimiter of its own, matching real docutils exactly: a classifier
// boundary can only ever fall inside a plain-text run.
//
// rawTerm is the term's own pre-inline-parse source text, consulted
// ONLY when termNodes turned out to be a single bare Text run (no
// embedded markup at all): real docutils' escape2null keeps an escaped
// character's marker byte in place right through inline_text, so
// Text.classifier_delimiter's regex naturally never matches a BACKSLASH-
// ESCAPED colon (“ \: “ — the marker sits between the space and the
// colon, breaking the pattern) — this project's own unescapeRunes,
// called while building termNodes, already collapsed that distinction
// away by the time a *doctree.Text reaches here. Re-deriving
// escape-awareness from rawTerm via escapeBackslashes (rune identity,
// the same technique validStartBoundary/findClose already use) restores
// it for the common, unambiguous case; a term containing OTHER inline
// markup alongside a real or escaped colon is rare enough, and
// re-deriving position correspondence through markup that can add or
// remove characters is fragile enough, that it's left to the plain
// (escape-unaware) split there instead — not corpus-verified either
// way, and the general Text-node split above is what real docutils
// itself ultimately reduces to once escaping is accounted for.
func splitTermClassifiers(termNodes []doctree.Node, rawTerm string) []doctree.Node {
	term := doctree.NewElement(doctree.TagTerm)
	nodeList := []*doctree.Element{term}
	if len(termNodes) == 1 {
		if _, ok := termNodes[0].(*doctree.Text); ok {
			for i, part := range splitEscapedClassifierText(escapeBackslashes(rawTerm)) {
				if i == 0 {
					nodeList[0].Append(&doctree.Text{Data: trimTrailingSpace(part)})
					continue
				}
				nodeList = append(nodeList, doctree.NewElement(doctree.TagClassifier, &doctree.Text{Data: part}))
			}
			out := make([]doctree.Node, len(nodeList))
			for i, e := range nodeList {
				out[i] = e
			}
			return out
		}
	}
	for _, n := range termNodes {
		text, ok := n.(*doctree.Text)
		if !ok {
			nodeList[len(nodeList)-1].Append(n)
			continue
		}
		parts := classifierDelimiter.Split(text.Data, -1)
		if len(parts) == 1 {
			nodeList[len(nodeList)-1].Append(&doctree.Text{Data: text.Data})
			continue
		}
		nodeList[len(nodeList)-1].Append(&doctree.Text{Data: trimTrailingSpace(parts[0])})
		for _, part := range parts[1:] {
			classifier := doctree.NewElement(doctree.TagClassifier, &doctree.Text{Data: part})
			nodeList = append(nodeList, classifier)
		}
	}
	out := make([]doctree.Node, len(nodeList))
	for i, e := range nodeList {
		out[i] = e
	}
	return out
}

// splitEscapedClassifierText is classifierDelimiter's own logic
// (one-or-more real spaces, a real ':', one-or-more real spaces) applied
// directly to an escaped-rune array (escapeBackslashes' own
// representation) instead of a plain string: an escaped rune never
// equals the literal ' '/':' it stands for (isEscapedRune's whole
// purpose, shared with every other marker/boundary check in this
// package), so a backslash-escaped colon or space simply can't
// participate in a match, exactly mirroring real docutils' own
// \x00-marker-breaks-the-regex behavior. Each returned piece is fully
// unescaped (unescapeRunes), same as any other final text.
func splitEscapedClassifierText(rs []rune) []string {
	var parts []string
	var cur []rune
	i := 0
	for i < len(rs) {
		if rs[i] == ' ' {
			j := i
			for j < len(rs) && rs[j] == ' ' {
				j++
			}
			if j < len(rs) && rs[j] == ':' {
				k := j + 1
				m := k
				for m < len(rs) && rs[m] == ' ' {
					m++
				}
				if m > k {
					parts = append(parts, unescapeRunes(cur))
					cur = nil
					i = m
					continue
				}
			}
		}
		cur = append(cur, rs[i])
		i++
	}
	parts = append(parts, unescapeRunes(cur))
	return parts
}

// parseDefinitionList returns the built <definition_list> plus, when the
// list is interrupted by a non-blank line that isn't itself a new term
// (real docutils' own Text.indent/unindent_warning, states.py, read
// directly, the SAME "chain a whole run of items through one nested
// parse, warn once at the very end" shape footnotes/citations and line
// blocks already have), a sibling "Definition list ends without a blank
// line; unexpected unindent." WARNING — unlike line_block's own
// first-line-based warning, this one reports the line right after the
// list's own last consumed content (the standard unindent_warning
// shape), matching the foreign judge exactly.
//
// lineBase mirrors every other line-scanning function in this package —
// see consumeParagraph's own doc comment.
func (p *parser) parseDefinitionList(lines []string, i, lineBase int) (*doctree.Element, []*doctree.Element, int) {
	dl := doctree.NewElement(doctree.TagDefinitionList)
	bodyNext := i
	for i < len(lines) && isDefinitionTermLine(lines, i) {
		term := trimTrailingSpace(lines[i])
		indent := leadingSpaces(lines[i+1])
		block, next := consumeIndentedBlock(lines, i+1, indent)
		bodyNext = next
		item := doctree.NewElement(doctree.TagDefinitionListItem)
		termLineno := 0
		if lineBase >= 0 {
			termLineno = i + lineBase + 1
		}
		termNodes, termMsgs := p.parseInline(term, termLineno)
		for _, n := range splitTermClassifiers(termNodes, term) {
			item.Append(n)
		}
		def := doctree.NewElement(doctree.TagDefinition)
		// real docutils' Text.definition_list_item: "dd = nodes.definition
		// ('', *messages)" — the term's own inline-markup messages become
		// the <definition>'s FIRST children, ahead of its own parsed
		// content (states.py, read directly).
		for _, m := range termMsgs {
			def.Append(m)
		}
		if strings.HasSuffix(term, "::") {
			// real docutils: "dd_lineno" is captured while already
			// positioned on the definition's own first (indented) line,
			// i.e. one past the term itself.
			def.Append(sectionMessage("1", "INFO",
				`Blank line missing before literal block (after the "::")? `+
					`Interpreted as a definition list item.`,
				msgLine(i+1, lineBase), ""))
		}
		p.parseBlockLines(block, def)
		item.Append(def)
		dl.Append(item)
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	var messages []*doctree.Element
	// real docutils' own get_indented: blank_finish is whether the line
	// JUST BEFORE the stopping point was blank — not whether the
	// stopping line itself is (consumeIndentedBlock's own trailing-blank
	// lines were already consumed and trimmed INTO the item's body, so
	// the stopping line is never blank by construction; checking it
	// directly would treat every list-followed-by-non-blank-text as an
	// abrupt unindent, even with a real blank line right before it).
	blankFinish := bodyNext >= len(lines) || isBlankStr(lines[bodyNext-1])
	if len(dl.Children) > 0 && !blankFinish {
		messages = append(messages, sectionMessage("2", "WARNING",
			"Definition list ends without a blank line; unexpected unindent.",
			msgLine(bodyNext, lineBase), ""))
	}
	return dl, messages, i
}
