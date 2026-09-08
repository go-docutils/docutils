package rst

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file covers reST's "explicit markup" (lines starting with `.. `):
// comments, directives, hyperlink targets, footnotes, citations, and
// substitution definitions, modeled on docutils' Body.explicit_construct
// dispatch (states.py) which tries, in order, footnote, citation,
// hyperlink_target, substitution_def, directive, and falls back to
// comment.
//
// SCOPE (v1): most directives are captured structurally ONLY — name, a
// single-line argument string, and the raw (unparsed, not split into
// options vs. content) body text — never dispatched to per-directive
// semantics; a substitution definition is captured the same way, with
// its `|name|` attached as an extra attribute. "raw", "table", and
// "list-table" are the exceptions with real per-directive semantics (see
// Options.RawEnabled and tabledirective.go); there is still no general
// directive registry beyond those three. Directive/comment/footnote/
// citation/substitution body indentation is dedented by whatever the
// FIRST body line's own actual indent is (docutils' own
// StringList.get_indented, read directly) — not a fixed assumption tied
// to the ".. " marker's own 3-column width, which an earlier version of
// this code wrongly assumed. A hyperlink target may be named
// (a direct URI, or chased through however many INDIRECT hops — see
// bareIndirectTargetName), an INLINE target inside a paragraph (see
// inline.go's tryInlineTarget), or ANONYMOUS (`.. __: uri`, resolved by
// document-order position rather than by name — see resolveTargets),
// including an INDIRECT anonymous target (`.. __: othername_`, chased the
// same way a named indirect target is). An
// unresolved reference (no matching target) is left as a bare
// `reference` node with no `refuri` attribute, which is exactly what
// real docutils' own bare parse produces: the rewrite to `problematic`
// plus a trailing system-message section is its DanglingReferences +
// Messages TRANSFORMS, which run after the parser and which a caller may
// never run. Those are available here behind
// Options.ReportDanglingReferences, OFF by default (v0.66.0). Footnote/citation numbering and symbol assignment (docutils'
// note_autofootnote/note_symbol_footnote bookkeeping) IS implemented —
// see footnotenum.go's resolveFootnoteNumbers, run at the end of Parse
// alongside resolveTargets. Citations are never auto-numbered (see
// parseFootnoteOrCitation above), so this applies only to footnotes.

func isExplicitMarkupLine(s string) bool {
	if len(s) < 2 || s[0] != '.' || s[1] != '.' {
		return false
	}
	return len(s) == 2 || s[2] == ' '
}

// gatherExplicitBody collects the body of an explicit-markup construct
// — a directive, substitution definition, or (via the generic fallback
// capture) any other explicit-markup line — the SAME way
// gatherFootnoteBody already does for footnotes/citations/comments:
// real docutils' own directive dispatch (RSTState.run_directive, states.py,
// read directly) calls get_first_known_indented(match.end()) too, with
// NO minimum-indent floor at all — any positive indentation counts, and
// the TRUE MINIMUM across every continuation line (not just the first)
// is what gets stripped, exactly like collectLiteralIndented. This used
// to require the first body line be indented >=3 columns before
// recognizing a body at all — a floor with no basis in real docutils
// (confirmed directly in run_directive's own source, not assumed), and
// wrong for the corpus's own real-world style: a directive whose
// options/content are indented by only 2 columns (".. code::\n  :class:
// x\n\n  content") was rejected outright, its body left for the OUTER
// dispatch to wrongly re-parse as unrelated top-level content — exactly
// the same class of bug gatherFootnoteBody was built to fix for
// footnotes in the first place. Leading AND trailing blank body lines
// are trimmed (matching the previous consumeIndentedBlock-based
// implementation's own behavior, which every existing caller here
// already depends on — unlike gatherFootnoteBody's own callers, which
// don't need this since their content gets re-parsed and blank lines
// there are harmless no-ops): a blank line right after the marker,
// before the real indented block, is now COLLECTED by
// collectLiteralIndented (unlike the old skip-ahead-before-collecting
// loop) and must be trimmed back off explicitly, or it leaks into the
// body as a spurious leading blank — caught immediately by this
// project's own existing "raw" directive tests, not the corpus.
// blankFinish (added when topics/sidebars needed the same "Explicit
// markup ends without a blank line" diagnostic footnotes/citations/line
// blocks/definition lists/field lists/comments already have — see
// stoppedOnExplicitMarkup's own established pattern) is collectLiteralIndented's
// own return value, UNCHANGED by the leading/trailing blank-trim below
// (trimming which lines are INCLUDED in body doesn't change whether the
// block as a whole ended on a blank line).
func gatherExplicitBody(lines []string, i int) ([]string, bool, int) {
	body, _, blankFinish, next := collectLiteralIndented(lines, i+1, false)
	for len(body) > 0 && isBlankStr(body[0]) {
		body = body[1:]
	}
	for len(body) > 0 && isBlankStr(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	return body, blankFinish, next
}

// gatherFootnoteBody collects a footnote or citation's continuation
// lines — real docutils' Body.footnote/Body.citation both call
// get_first_known_indented(match.end()) (states.py, read directly),
// DISTINCT from gatherExplicitBody's fixed 3-column floor used for
// comments/directives/substitutions: a footnote's own body has no
// minimum indent requirement at all, and no fixed dedent column either
// — any positive indentation counts, and the TRUE MINIMUM across every
// continuation line (not just the first) is what gets stripped, exactly
// like collectLiteralIndented (the same underlying
// docutils.statemachine.StringList.get_indented, unknown indent for
// every line but the first, which parseFootnoteOrCitation's own
// firstLineRest already carries dedented). A real corpus case
// (".. [2] text\n  less-indented continuation") needs exactly this: the
// old fixed-3-column gatherExplicitBody rejected the shallower
// continuation outright, leaving it to spin off as an unrelated
// block_quote sibling instead of joining the footnote's own paragraph.
// blankFinish reports whether the body was followed by a blank line (or
// real EOF) rather than an abrupt unindented line — real docutils'
// Body.explicit_markup wraps EVERY explicit construct in the same
// "ends without a blank line; unexpected unindent" warning when it
// isn't, ported at parseFootnoteOrCitation's own call site.
func gatherFootnoteBody(lines []string, i int) (body []string, blankFinish bool, next int) {
	body, _, blankFinish, next = collectLiteralIndented(lines, i+1, false)
	return body, blankFinish, next
}

// parseExplicitMarkup returns every sibling node one explicit-markup
// construct produces — almost always exactly one, but a directive that
// builds its own diagnostics as SIBLINGS (real docutils' own
// "self.parent += extra_message" shape — see the table/list-table
// directives, tabledirective.go) needs more than one, and a role
// registration (see parseDirective) needs zero.
//
// A bare ".." (exactly 2 characters, nothing else on the line) is only
// an immediately-empty comment when the FOLLOWING line is itself blank
// or EOF — real docutils' own Body.comment checks
// `self.state_machine.is_next_line_blank()` before taking that
// shortcut, states.py read directly. When something non-blank follows
// (indented or not — a real corpus case: a bare ".." then an indented
// paragraph, a hyperlink-target-looking line, a citation-looking line,
// or a substitution-looking line, none of which get re-interpreted as
// their own construct), it falls through to the SAME general comment
// path a ".. " marker with no same-line content uses — a bare ".."
// was previously ALWAYS treated as an empty comment, wrongly leaving
// its own body to be picked up by the outer dispatch loop as an
// unrelated, WRONGLY RE-PARSED sibling construct.
//
// lineBase mirrors every other line-scanning function in this package —
// see consumeParagraph's own doc comment.
func (p *parser) parseExplicitMarkup(lines []string, i, lineBase int, parent *doctree.Element) ([]doctree.Node, int) {
	line := lines[i]
	if len(line) == 2 {
		if i+1 >= len(lines) || isBlankStr(lines[i+1]) {
			return []doctree.Node{doctree.NewElement(doctree.TagComment)}, i + 1
		}
		return p.parseComment(lines, i, lineBase, "")
	}
	rest := line[3:]

	if label, labelRest, ok := matchBracketLabel(rest); ok {
		return p.parseFootnoteOrCitation(lines, i, lineBase, label, labelRest)
	}
	if strings.HasPrefix(rest, "__:") {
		node, next := p.parseAnonymousTarget(lines, i, rest[3:])
		return []doctree.Node{node}, next
	}
	if len(rest) > 1 && rest[0] == '_' && rest[1] != ' ' {
		node, next := p.parseHyperlinkTarget(lines, i, rest[1:])
		return []doctree.Node{node}, next
	}
	if subName, subRest, bodyStartIdx, ok := matchPipeLabelMultiline(lines, i, rest); ok {
		if nodes, next, ok := p.parseSubstitutionDef(lines, i, bodyStartIdx, subName, subRest, parent); ok {
			return nodes, next
		}
		// Malformed substitution definition: fall through to comment,
		// matching docutils' fallback for any unmatched explicit
		// construct (explicit_construct's final `return self.comment(...)`).
	}
	if name, args, ok := matchDirectiveName(rest); ok {
		return p.parseDirective(lines, i, lineBase, name, args, parent)
	}
	return p.parseComment(lines, i, lineBase, rest)
}

// matchBracketLabel recognizes "[label] rest" at the start of s — the
// shared shape of footnote (`.. [1]`, `.. [#]`, `.. [#name]`, `.. [*]`)
// and citation (`.. [name]`) markers. The closing ']' must be followed
// by whitespace or end-of-string.
func matchBracketLabel(s string) (label, rest string, ok bool) {
	if len(s) == 0 || s[0] != '[' {
		return "", "", false
	}
	end := strings.IndexByte(s, ']')
	if end < 0 {
		return "", "", false
	}
	if end+1 < len(s) && s[end+1] != ' ' {
		return "", "", false
	}
	if !isValidBracketLabel(s[1:end]) {
		return "", "", false
	}
	return s[1:end], strings.TrimSpace(s[end+1:]), true
}

// isValidBracketLabel reports whether label is one real docutils would
// actually accept between the brackets of a footnote or citation
// marker: the UNION of its own two dispatch patterns (Body.explicit's
// constructs table, read directly) —
//
//	footnote: [0-9]+  |  "#"  |  "#"<simplename>  |  "*"
//	citation: <simplename>
//
// where simplename is the same `(?:(?!_)\w)+(?:[-._+:](?:(?!_)\w)+)*`
// grammar scanSimpleName already implements for inline names: runs of
// alphanumerics joined by single "-._+:" separators.
//
// Anything else — spaces, inline markup, an empty label — matches
// NEITHER construct, so real docutils falls all the way through its
// explicit-markup dispatch to the final comment fallback rather than
// inventing a citation out of it. This project used to accept ANY text
// up to the closing bracket, silently turning a line like
// ".. [citation label with spaces] this isn't a citation" into a real
// <citation> whose own body was that disclaimer — an over-acceptance
// bug the corpus fixture that names itself "this isn't a citation"
// exists precisely to catch. Returning false here lands such a line on
// parseExplicitMarkup's own already-present comment fallback, no new
// branch needed.
func isValidBracketLabel(label string) bool {
	if label == "*" || label == "#" {
		return true
	}
	name := label
	if strings.HasPrefix(label, "#") {
		name = label[1:]
	}
	return isSimpleName(name)
}

// isSimpleName reports whether the WHOLE of s is one docutils
// simplename — scanSimpleName (inline.go) scans a prefix, so this is
// just "did it consume everything".
func isSimpleName(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)
	return scanSimpleName(r, 0) == len(r)
}

// parseFootnoteOrCitation classifies a bracket-labeled explicit-markup
// block: "*" or a "#"-prefixed or all-digit label is a footnote
// (auto-symbol / auto-numbered / manually numbered); anything else is a
// citation, docutils' footnote vs. citation split. The manually-numbered
// footnote and citation cases get a real "id" attribute (docutils'
// note_explicit_target/set_id, via explicitTargetID — see its own doc
// comment) — auto footnotes/symbols are deliberately left untouched:
// their eventual id/label/numbering is real docutils' Footnotes
// TRANSFORM, not something Body.footnote itself does, and this project
// already runs a drastically-simplified version of that same transform
// eagerly at parse time (footnotenum.go's resolveFootnoteNumbers) rather
// than matching the corpus's own bare (pre-transform) parse — a
// deliberate, previously-confirmed divergence (the SAME "eager
// resolution vs. bare-parse ground truth" confound already established
// for hyperlink references), not something this fix reopens.
//
// Two diagnostics real docutils gives EVERY footnote/citation
// (states.py's Body.footnote/Body.citation, plus the shared
// Body.explicit_markup wrapper, all read directly) are ported here: an
// empty body ("Footnote content expected."/"Citation content
// expected.", nested INSIDE the element, right after its label) and a
// body that ends on a non-blank unindented line rather than a blank one
// ("Explicit markup ends without a blank line; unexpected unindent.", a
// SIBLING of the element, not nested — matching real docutils' own
// generic wrapper, which applies this uniformly to every explicit
// construct, not just footnotes/citations, though only this call site
// ports it so far).
func (p *parser) parseFootnoteOrCitation(lines []string, i, lineBase int, label, firstLineRest string) ([]doctree.Node, int) {
	body, blankFinish, next := gatherFootnoteBody(lines, i)
	content := append([]string{firstLineRest}, body...)
	for len(content) > 0 && isBlankStr(content[len(content)-1]) {
		content = content[:len(content)-1]
	}

	var el *doctree.Element
	contentKind := "Footnote"
	defer func() {
		// The duplicate-name pass runs after parsing and reports against
		// the construct's own line; a footnote/citation is block-level, so
		// noteNameLine's inline p.currentLine is never set for one.
		if el != nil {
			if p.nameLines == nil {
				p.nameLines = map[*doctree.Element]int{}
			}
			p.nameLines[el] = msgLine(i, lineBase)
		}
	}()
	switch {
	case label == "*":
		// An auto footnote's id is assigned at PARSE time
		// (document.note_symbol_footnote -> set_id), independently of the
		// numbering, which is a transform. Auto and symbol footnotes share
		// ONE positional counter: ".. [#] a" then ".. [*] c" gives
		// footnote-1 then footnote-2.
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("auto", "*")
		el.SetAttr("id", p.explicitTargetID("footnote", ""))
	case len(label) > 0 && label[0] == '#':
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("auto", "1")
		name := ""
		if n := label[1:]; n != "" {
			name = normalizeName(n)
			el.SetAttr("name", name)
		}
		el.SetAttr("id", p.explicitTargetID("footnote", name))
	case isAllDigits(label):
		el = doctree.NewElement(doctree.TagFootnote)
		el.SetAttr("name", label)
		el.SetAttr("id", p.explicitTargetID("footnote", label))
		el.Append(doctree.NewElement(doctree.TagLabel, &doctree.Text{Data: label}))
	default:
		contentKind = "Citation"
		el = doctree.NewElement(doctree.TagCitation)
		name := normalizeName(label)
		el.SetAttr("name", name)
		el.SetAttr("id", p.explicitTargetID("citation", name))
		el.Append(doctree.NewElement(doctree.TagLabel, &doctree.Text{Data: label}))
	}
	if len(content) > 0 {
		p.parseBlockLines(content, el, -1)
	} else {
		// next-1, NOT next: this warning carries the line the construct
		// itself ENDED on (the last line it actually consumed), where
		// the "ends without a blank line" one below carries the line
		// that INTERRUPTED it — two different questions that happen to
		// share the same `next`. Verified against the foreign judge
		// across all three shapes: blank-line-terminated (".. [c]\n\nX\n"
		// → the blank, line 2), EOF-terminated (".. [c]\n" → the marker
		// itself, line 1), and abruptly interrupted (".. [c]\nX\n" →
		// again the marker, line 1, while the unindent warning there
		// correctly says line 2). Was off by one in every one of them;
		// no footnote fixture in the corpus has an empty body, so the
		// identical bug on that side went uncaught until citations
		// surfaced it.
		el.Append(sectionMessage("2", "WARNING", contentKind+" content expected.", msgLine(next-1, lineBase), ""))
	}
	nodes := []doctree.Node{el}
	// Real docutils chains a whole RUN of explicit-markup constructs
	// through one nested "Explicit" state machine (Body.explicit_list,
	// read directly), which only raises this warning once the chain is
	// broken by something that ISN'T itself another explicit-markup
	// line — a body that stops abruptly right before a SIBLING ".. "
	// construct (no blank line needed between two adjacent footnotes)
	// is not abrupt at all, just the next construct starting.
	stoppedOnExplicitMarkup := next < len(lines) && isExplicitMarkupLine(lines[next])
	if !blankFinish && !stoppedOnExplicitMarkup {
		nodes = append(nodes, sectionMessage("2", "WARNING",
			"Explicit markup ends without a blank line; unexpected unindent.", msgLine(next, lineBase), ""))
	}
	return nodes, next
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// matchPipeLabel recognizes "|name| rest" at the start of s — a
// substitution definition marker. The name may not contain "|".
func matchPipeLabel(s string) (name, rest string, ok bool) {
	if len(s) == 0 || s[0] != '|' {
		return "", "", false
	}
	end := strings.IndexByte(s[1:], '|')
	if end < 0 {
		return "", "", false
	}
	end++ // index within s, not s[1:]
	// docutils' substitution pattern brackets the name with "(?![ ])" and
	// "(?<![ ])", so whitespace immediately INSIDE either pipe
	// disqualifies the whole construct and it falls through to the
	// comment fallback: ".. | bad name | bad data" is a <comment>. Spaces
	// WITHIN the name are fine -- "|good name|" is a real substitution.
	if name := s[1:end]; name == "" || name[0] == ' ' || name[len(name)-1] == ' ' {
		return "", "", false
	}
	return s[1:end], strings.TrimSpace(s[end+1:]), true
}

// matchPipeLabelMultiline extends matchPipeLabel for a "|name|" marker
// whose closing "|" isn't on the SAME line as the opening one — real
// docutils' Body.substitution_def (read directly) progressively
// re-matches its own substitution pattern against the marker's growing
// text, appending one more stripped, indented continuation line (joined
// by a single space) each time the closing "|" isn't found yet, until
// it is (or the block runs out, a malformed definition). bodyStartIdx
// is i unchanged for the fast (single-line) path, or the index of the
// LAST line consumed by the name when it spanned more than one — the
// caller needs it to know where the embedded directive's own body
// gathering should actually start (lines already consumed by the name
// itself must not be re-offered to it as body content). Doesn't
// replicate the escape-null handling real docutils' own escape2null
// gives the closing "|" (a "\|" inside the name shouldn't close the
// marker) — no corpus fixture combines that with a multi-line name.
func matchPipeLabelMultiline(lines []string, i int, firstLineRest string) (name, directiveRest string, bodyStartIdx int, ok bool) {
	if name, rest, ok := matchPipeLabel(firstLineRest); ok {
		return name, rest, i, true
	}
	if len(firstLineRest) == 0 || firstLineRest[0] != '|' {
		return "", "", 0, false
	}
	accumulated := firstLineRest[1:]
	j := i + 1
	for j < len(lines) && !isBlankStr(lines[j]) && leadingSpaces(lines[j]) > 0 {
		accumulated += " " + strings.TrimSpace(lines[j])
		if end := strings.IndexByte(accumulated, '|'); end >= 0 {
			return strings.TrimSpace(accumulated[:end]), strings.TrimSpace(accumulated[end+1:]), j, true
		}
		j++
	}
	return "", "", 0, false
}

// parseSubstitutionDef recognizes ".. |name| directive:: args" (the
// directive marker may start on a LATER body line entirely, when nothing
// follows "|name|" on the definition's own first line — see block's
// construction below) — the ONLY shape a substitution definition takes:
// its content is always an embedded directive invocation, most commonly
// "replace::" or "image::" (ported here) or "raw::" (already a real
// directive, just needed proper nesting — see below); "unicode::" is not
// ported (no corpus case needs it). Ports Body.substitution_def +
// SubstitutionDef (states.py, read directly): unlike parseDirective's own
// generic structural capture, a genuine substitution_definition NESTS
// whatever the embedded directive actually produced as real children —
// an <image>, a <raw>, or replace's own inline-parsed content — never
// flattening it onto attributes the way this project's earlier version
// did (a real, previously-shipped bug: it relabeled the embedded
// directive's OWN element in place rather than wrapping it).
//
// Returns ok=false only for a genuinely malformed marker with NO
// embedded-directive-shaped line anywhere in its own body at all AND no
// recognized body-text fallback either — matching docutils' own final
// "return self.comment(...)" when even the substitution-marker pattern
// itself never matches. bodyStartIdx is i unchanged when the "|name|"
// marker closed on its own first line, or a LATER index when
// matchPipeLabelMultiline had to consume additional lines to find the
// closing "|" — see that function's own doc comment; gatherExplicitBody
// below is anchored there specifically so a line already consumed as
// part of the (possibly multi-line) NAME is never re-offered to the
// embedded directive as its own body content.
func (p *parser) parseSubstitutionDef(lines []string, i, bodyStartIdx int, name, directiveRest string, parent *doctree.Element) (nodes []doctree.Node, nextOut int, okOut bool) {
	lineno := i + 1
	subname := normalizeWhitespace(name)
	body, blankFinish, next := gatherExplicitBody(lines, bodyStartIdx)
	// The SAME "Explicit markup ends without a blank line" warning every
	// other explicit construct already appends -- footnotes, citations,
	// comments, directives, topics. A substitution definition simply
	// discarded gatherExplicitBody's blankFinish, so it was the one
	// member of the family that stayed silent. Appended to whatever this
	// function returns, since it has several exits.
	defer func() {
		if okOut && !blankFinish && !(next < len(lines) && isExplicitMarkupLine(lines[next])) {
			nodes = append(nodes, sectionMessage("2", "WARNING",
				"Explicit markup ends without a blank line; unexpected unindent.", next+1, ""))
		}
	}()
	blockText := strings.Join(lines[i:next], "\n")

	// gatherExplicitBody's own body has already dropped the blank
	// line(s) separating directiveRest from the indented block below it
	// (see admonitions.go's runAdmonitionOrGeneric, the same reasoning,
	// reused a third time here) — reinserted so parseDirectiveBlock (or,
	// for "raw", the plain body-join below) can find the real boundary.
	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	block := make([]string, 0, 1+blanks+len(body))
	block = append(block, directiveRest)
	for k := 0; k < blanks; k++ {
		block = append(block, "")
	}
	block = append(block, body...)
	for len(block) > 0 && isBlankStr(block[len(block)-1]) {
		block = block[:len(block)-1]
	}
	if len(block) > 0 && block[0] == "" {
		block = block[1:]
	}

	// The same block, but with every continuation line's ORIGINAL
	// indentation intact. Body.substitution_def calls
	// get_first_known_indented(match.end(), strip_indent=False) -- the
	// only caller in states.py that passes strip_indent=False -- so
	// docutils' own block keeps its indentation and only its FIRST line
	// (the text after the "|name|" marker) is stripped. That block is
	// what an embedded directive's error message quotes, so a body
	// indented relative to the directive shows up indented there.
	// gatherExplicitBody dedents, which is right for PARSING and wrong
	// for QUOTING; rebuilding from the source lines keeps both.
	rawBlock := make([]string, 0, 1+next-(bodyStartIdx+1))
	rawBlock = append(rawBlock, directiveRest)
	if bodyStartIdx+1 < next {
		rawBlock = append(rawBlock, lines[bodyStartIdx+1:next]...)
	}
	for len(rawBlock) > 0 && isBlankStr(rawBlock[len(rawBlock)-1]) {
		rawBlock = rawBlock[:len(rawBlock)-1]
	}
	dirBlockText := strings.Join(rawBlock, "\n")

	if len(block) == 0 {
		return []doctree.Node{sectionMessage("2", "WARNING",
			`Substitution definition "`+subname+`" missing contents.`, lineno, ".. |"+name+"|")}, next, true
	}

	dirName, dirArgs, ok := matchDirectiveName(strings.TrimSpace(block[0]))
	if !ok {
		// Body.substitution_def's own "text" fallback: arbitrary body
		// content with no embedded-directive marker at all nested-parses
		// as ordinary block content, then keeps only its top-level
		// Inline/Text children — never any, since parseBlockLines always
		// produces block-level elements (a <paragraph>, a <bullet_list>,
		// ...) here, so this always ends up empty. Shortcuts straight to
		// the same "empty or invalid" outcome that emptiness produces
		// below, without actually invoking parseBlockLines for content
		// this project already knows can never survive the filter.
		return []doctree.Node{sectionMessage("2", "WARNING",
			`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)}, next, true
	}
	rest := block[1:]

	el := doctree.NewElement(doctree.TagSubstitutionDef)
	// Record the definition's own source line for the duplicate-name pass,
	// which runs after parsing and reports against it. A substitution
	// definition is block-level, so noteNameLine's inline p.currentLine is
	// never set for one.
	if p.nameLines == nil {
		p.nameLines = map[*doctree.Element]int{}
	}
	p.nameLines[el] = lineno
	el.SetAttr("name", subname)

	switch {
	case strings.EqualFold(dirName, "replace"):
		msgs, errEl := p.fillReplaceSubstitution(el, subname, dirArgs, rest, lineno, blockText, dirBlockText)
		if errEl != nil {
			return append(msgs, errEl), next, true
		}
		if len(msgs) > 0 {
			return append(msgs, el), next, true
		}
	case strings.EqualFold(dirName, "raw") && p.opts.RawEnabled && dirArgs != "":
		raw := doctree.NewElement(doctree.TagRaw)
		raw.SetAttr("format", strings.ToLower(strings.Join(strings.Fields(dirArgs), " ")))
		rawBody := trimLeadingBlanks(rest)
		for len(rawBody) > 0 && isBlankStr(rawBody[len(rawBody)-1]) {
			rawBody = rawBody[:len(rawBody)-1]
		}
		if len(rawBody) > 0 {
			raw.Append(&doctree.Text{Data: strings.Join(rawBody, "\n")})
		}
		el.Append(raw)
	case strings.EqualFold(dirName, "date"):
		// misc.Date declares no arguments and has_content=True, so the
		// text after "::" folds into the content block -- exactly what
		// parseDirectiveBlock's hasArgument=false path does.
		combined := append([]string{dirArgs}, rest...)
		_, _, content := parseDirectiveBlock(combined, false)
		el.Append(&doctree.Text{Data: dateDirectiveText(content)})
	case strings.EqualFold(dirName, "image"):
		combined := append([]string{dirArgs}, rest...)
		argument, options, content := parseDirectiveBlock(combined, true)
		// The directive's OWN block, not the whole substitution line: an
		// "Error in ..." message quotes the directive, while the
		// "empty or invalid" warning that follows quotes the substitution.
		// Same pairing as the replace directive's content checks.
		nodes := finishImageDirective("image", argument, options, content, subname, lineno, dirBlockText)
		if len(nodes) != 1 {
			return []doctree.Node{sectionMessage("2", "WARNING",
				`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)}, next, true
		}
		if img, ok := nodes[0].(*doctree.Element); ok && img.Tag == doctree.TagImage {
			el.Append(img)
		} else {
			// A failed image directive produces its own ERROR, and the
			// substitution it was standing in for is then empty -- both
			// messages, in that order.
			return []doctree.Node{nodes[0], sectionMessage("2", "WARNING",
				`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)}, next, true
		}
	default:
		// Any OTHER directive name: either a real but non-inline directive
		// (real docutils runs it, then filters its non-Inline result out,
		// ending up empty the same way the "text" fallback above does) or
		// one this package has never heard of. "empty or invalid" is the
		// right eventual outcome either way.
		//
		// An UNKNOWN name also draws the same diagnostics pair the
		// top-level directive dispatch raises (v0.68.0), which this branch
		// used to opt out of with a comment about "already-established
		// leniency" -- a note that outlived the leniency it described. The
		// pair comes FIRST, and its <literal_block> is the DIRECTIVE
		// block, not the whole substitution line the "empty or invalid"
		// warning quotes.
		var out []doctree.Node
		if p.opts.ReportUnknownDirectives && !isImplementedDirective(dirName) {
			out = append(out, unknownDirectiveBlockDiagnostics(dirName, dirBlockText, lineno)...)
		}
		return append(out, sectionMessage("2", "WARNING",
			`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)), next, true
	}

	if len(el.Children) == 0 {
		return []doctree.Node{sectionMessage("2", "WARNING",
			`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)}, next, true
	}
	return []doctree.Node{el}, next, true
}

// fillReplaceSubstitution implements the "replace" directive (misc.py's
// Replace.run, read directly) — valid ONLY inside a substitution
// definition (real docutils raises an "Invalid context" error otherwise;
// this project only ever reaches this path from inside one, so that
// restriction holds for free rather than needing an explicit check).
// Its same-line argument text plus any body lines are joined and
// inline-parsed as ONE paragraph's worth of content, appended directly
// as el's own children — then checked against the same three
// prohibited-content categories real docutils checks for ANY embedded
// directive's result (disallowed_inside_substitution_definitions, read
// directly): an anonymous reference, an auto-numbered/auto-symbol
// footnote reference, or anything carrying its own name/id (an inline
// target); a <problematic> anywhere in the result (most commonly an
// unclosed inline-markup start-string) is checked FIRST, before either
// of those, matching substitution_def's own findall loop order. msgs is
// every inline-markup diagnostic parseInline itself generated (an
// unclosed emphasis/strong/literal start-string, etc.) — real docutils
// attaches these as SIBLINGS of whatever construct produced them (the
// v0.20.1 round's own placement rule, reused here for the same reason:
// this is genuinely a paragraph-equivalent construct), always returned
// regardless of whether errEl is also non-nil, since the corpus shows
// them appearing BEFORE either outcome.
//
// SCOPE: real docutils' Replace.run assert_has_content()'s + a genuine
// multi-paragraph check (self.content joined and re-nested-parsed,
// erroring "may contain a single paragraph only" when it splits into
// more than one) are both collapsed here into the SAME "empty or
// invalid" warning parseSubstitutionDef already gives an embedded
// directive with no usable result — a real, deliberate simplification,
// not an oversight: this project's own text-then-parseInline shape
// never actually detects a paragraph break within replace's own content
// the way a full nested_parse would, and the two real, more specific
// diagnostics that distinction unlocks are low corpus value on their
// own (test_directives/test_replace.py, 2 of its 5 cases).
func (p *parser) fillReplaceSubstitution(el *doctree.Element, subname, dirArgs string, rest []string, lineno int, blockText, dirBlock string) (msgs []doctree.Node, errEl *doctree.Element) {
	text := dirArgs
	body := trimLeadingBlanks(rest)
	for len(body) > 0 && isBlankStr(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	if len(body) > 0 {
		if text != "" {
			text += "\n"
		}
		text += strings.Join(body, "\n")
	}
	if strings.TrimSpace(text) == "" {
		// Replace.run's own required-content check (misc.py). The ERROR
		// quotes the DIRECTIVE block, while the "empty or invalid"
		// warning that follows quotes the whole substitution line.
		return []doctree.Node{sectionMessage("3", "ERROR",
				`Content block expected for the "replace" directive; none found.`, lineno, dirBlock)},
			sectionMessage("2", "WARNING",
				`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)
	}
	// Replace.run rejects anything but ONE paragraph. An internal blank
	// line between content lines is a second paragraph, and the directive
	// fails rather than joining them -- this used to concatenate both into
	// a single substitution, which the README described as a deliberate
	// simplification.
	// The blank line can sit between the SAME-LINE argument and the body
	// (".. |n| replace:: para 1" + blank + "para 2"), which the body's own
	// leading-blank trim has already removed by here -- so the check runs
	// over the combined shape, not over body alone.
	if hasBlankSeparatedParagraphs(append([]string{dirArgs}, rest...)) {
		return []doctree.Node{sectionMessage("3", "ERROR",
				`Error in "replace" directive: may contain a single paragraph only.`, lineno, "")},
			sectionMessage("2", "WARNING",
				`Substitution definition "`+subname+`" empty or invalid.`, lineno, blockText)
	}
	inlineNodes, inlineMsgs := p.parseInline(text, lineno)
	for _, m := range inlineMsgs {
		msgs = append(msgs, m)
	}
	if containsProblematic(inlineNodes) {
		// Verified directly against the foreign judge: the WARNING
		// messages parseInline already generated (msgs, above) carry NO
		// "backref" here, unlike their normal shape everywhere else in
		// this package — the problematic nodes they describe are about
		// to be reparented into the block_quote reconstruction below,
		// not left in their original paragraph position, so the usual
		// 1:1 warning<->problematic backlink real docutils' own
		// system_message machinery adds elsewhere doesn't apply the same
		// way in this specific reconstruction.
		for _, m := range msgs {
			if el, ok := m.(*doctree.Element); ok {
				delete(el.Attrs, "backref")
			}
		}
		msg := doctree.NewElement(doctree.TagSystemMessage,
			doctree.NewElement(doctree.TagParagraph, &doctree.Text{Data: "Problematic content in substitution definition"}),
			doctree.NewElement(doctree.TagLiteralBlock, &doctree.Text{Data: blockText}),
			doctree.NewElement(doctree.TagBlockQuote, doctree.NewElement(doctree.TagParagraph, inlineNodes...)))
		msg.SetAttr("level", "3")
		msg.SetAttr("type", "ERROR")
		if lineno != 0 {
			msg.SetAttr("line", strconv.Itoa(lineno))
		}
		return msgs, msg
	}
	if illegal := disallowedInSubstitution(inlineNodes); illegal != "" {
		return msgs, sectionMessage("3", "ERROR", illegal+" are not supported in a substitution definition.", lineno, blockText)
	}
	for _, n := range inlineNodes {
		el.Append(n)
	}
	return msgs, nil
}

// containsProblematic mirrors substitution_def's own leading check (real
// docutils checks this BEFORE disallowedInSubstitution, in the same
// findall loop) — any <problematic> anywhere in the embedded directive's
// result (an unclosed inline-markup start-string, most commonly) makes
// the whole substitution definition invalid.
func containsProblematic(nodes []doctree.Node) bool {
	for _, n := range nodes {
		el, ok := n.(*doctree.Element)
		if !ok {
			continue
		}
		if el.Tag == doctree.TagProblematic {
			return true
		}
		if containsProblematic(el.Children) {
			return true
		}
	}
	return false
}

// disallowedInSubstitution mirrors
// Body.disallowed_inside_substitution_definitions, walked recursively (a
// real docutils substitution_node.findall(...) walks the WHOLE nested
// tree, not just top-level children) and in document order, since the
// check returns on the FIRST offending node encountered — not the first
// offending category.
func disallowedInSubstitution(nodes []doctree.Node) string {
	for _, n := range nodes {
		if msg := disallowedInSubstitutionNode(n); msg != "" {
			return msg
		}
	}
	return ""
}

func disallowedInSubstitutionNode(n doctree.Node) string {
	el, ok := n.(*doctree.Element)
	if !ok {
		return ""
	}
	if el.Tag == doctree.TagReference && el.Attr("anonymous") == anonymousAttrValue {
		return "Anonymous references"
	}
	if el.Tag == doctree.TagFootnoteReference && (el.Attr("auto") == "1" || el.Attr("auto") == "*") {
		return "References to auto-numbered and auto-symbol footnotes"
	}
	if el.Tag == doctree.TagTarget && el.Attr("name") != "" {
		return "Targets (names and identifiers)"
	}
	for _, c := range el.Children {
		if msg := disallowedInSubstitutionNode(c); msg != "" {
			return msg
		}
	}
	return ""
}

// trimLeadingBlanks drops every leading blank line — gatherExplicitBody's
// own convention (it skips leading blanks to FIND a body's indent,
// never preserving them), needed here because block's own construction
// (parseSubstitutionDef, above) preserves them instead, for
// parseDirectiveBlock's benefit; "raw" and "replace" don't go through
// parseDirectiveBlock at all, so they need this applied explicitly.
func trimLeadingBlanks(lines []string) []string {
	i := 0
	for i < len(lines) && isBlankStr(lines[i]) {
		i++
	}
	return lines[i:]
}

// normalizeWhitespace mirrors docutils.nodes.whitespace_normalize_name:
// unlike target/reference names, a substitution name is case-sensitive.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// matchDirectiveName recognizes "name:: arguments" at the start of rest.
//
// The name is docutils' own `simplename` -- alphanumeric runs joined by
// "-._+:" -- which scanSimpleName already implements for ROLES (v0.54.0).
// This function had its own character-class loop, missing ":" entirely,
// so a namespaced directive like ".. rst:directive:: x" fell through to
// the comment fallback instead of being reported as an unknown directive.
// The two sibling scans had drifted apart exactly the way the email and
// URI end-boundary tests had (v0.87.0).
//
// Using the shared grammar also makes a LEADING "_" stop being a name
// character, matching simplename's own "(?<!_)(?:(?!_)\w)+": ".. _x:: y"
// is not a directive in docutils either.
func matchDirectiveName(rest string) (name, args string, ok bool) {
	runes := []rune(rest)
	end := scanSimpleName(runes, 0)
	if end == 0 {
		return "", "", false
	}
	j := len(string(runes[:end]))
	k := j
	if k < len(rest) && rest[k] == ' ' {
		k++
	}
	if !strings.HasPrefix(rest[k:], "::") {
		return "", "", false
	}
	return rest[:j], strings.TrimSpace(rest[k+2:]), true
}

// parseDirective captures a directive structurally — except "raw"
// (`.. raw:: FORMAT`, optionally several space-separated formats, e.g.
// `.. raw:: html latex`), the one directive this parser gives real
// semantics: when p.opts.RawEnabled (see Options), its body passes
// through completely unprocessed as a <raw format="..."> node instead,
// content never touched by parseInline/parseBlockLines at all — the
// entire point of "raw". Disabled (or given no format argument, which
// real docutils treats as a directive-level error this parser doesn't
// generally validate for anyway), it falls back to the same structural
// capture any other unimplemented directive gets.
func (p *parser) parseDirective(lines []string, i, lineBase int, name, args string, parent *doctree.Element) ([]doctree.Node, int) {
	body, blankFinish, next := gatherExplicitBody(lines, i)
	if strings.EqualFold(name, "replace") || strings.EqualFold(name, "date") {
		// Real docutils' Replace.run (misc.py, read directly) is only
		// ever invoked FROM WITHIN a substitution definition's own
		// dispatch (SubstitutionDef.run calls run_directive with the
		// directive class directly, never through the normal
		// state-machine directive lookup a bare ".. replace::" goes
		// through) — reached here at all means ".. replace::" was used
		// as an ordinary top-level directive, which real docutils
		// rejects outright ("Invalid context"). This project's own
		// fillReplaceSubstitution (above) is the ONLY other "replace"
		// handling, called directly by parseSubstitutionDef, never
		// through this function — so this case can only ever fire for
		// the invalid-context shape, matching real docutils exactly.
		//
		// "date" is the second directive of that shape: misc.Date.run
		// opens by refusing any state that is not a SubstitutionDef,
		// with the same sentence and the same ERROR level.
		lineno := i + 1
		blockText := strings.Join(lines[i:next], "\n")
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Invalid context: the "`+strings.ToLower(name)+`" directive can only be used within a substitution definition.`, lineno, blockText)}, next
	}
	if name == "raw" && p.opts.RawEnabled && args != "" {
		el := doctree.NewElement(doctree.TagRaw)
		el.SetAttr("format", strings.ToLower(strings.Join(strings.Fields(args), " ")))
		if len(body) > 0 {
			el.Append(&doctree.Text{Data: strings.Join(body, "\n")})
		}
		return []doctree.Node{el}, next
	}
	if strings.EqualFold(name, "class") || strings.EqualFold(name, "rst-class") {
		return p.runClassDirective(name, args, body, bodyStartIndex(lines, i), lineBase), next
	}
	if strings.EqualFold(name, "sectnum") || strings.EqualFold(name, "section-numbering") {
		return runSectnumDirective(args, body), next
	}
	if strings.EqualFold(name, testDirectiveName) {
		blanks := 0
		for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
			blanks++
		}
		out := runTestDirective(name, args, body, blanks,
			strings.Join(trimTrailingBlankLines(lines[i:next]), "\n"), msgLine(i, lineBase))
		// The SAME warning every other explicit construct appends when
		// its body ends on an unindent rather than a blank line.
		if !blankFinish && !(next < len(lines) && isExplicitMarkupLine(lines[next])) {
			out = append(out, sectionMessage("2", "WARNING",
				"Explicit markup ends without a blank line; unexpected unindent.", msgLine(next, lineBase), ""))
		}
		return out, next
	}
	if strings.EqualFold(name, "target-notes") {
		return runTargetNotesDirective(args, body, strings.Join(trimTrailingBlankLines(lines[i:next]), "\n"), msgLine(i, lineBase)), next
	}
	if strings.EqualFold(name, "title") {
		// ".. title:: text" sets the document's own title ATTRIBUTE and
		// leaves no node behind at all (directives.parts.DocTitle).
		if t := strings.TrimSpace(args); t != "" {
			p.docTitle = t
		}
		return nil, next
	}
	if name == "role" {
		// Registers a custom interpreted-text role for the rest of the
		// document (see role.go) — invisible bookkeeping, same as a
		// hyperlink target: real docutils' own Role.run leaves NOTHING in
		// the tree, not even a comment (verified against the foreign
		// judge — an earlier version of this code returned a <comment>
		// element here, contradicting its own doc comment; caught only
		// once ":code:"/PEP/RFC role support made the surrounding
		// paragraph's own content correct enough for this stray sibling
		// to become the ONLY remaining diff) — UNLESS the definition
		// itself is malformed, in which case Role.run raises a real
		// diagnostic (registerRole's own doc comment).
		return p.registerRole(lines, i, next, args, body), next
	}
	if name == "table" {
		return p.runTableDirective(lines, i, next, lineBase, args, body), next
	}
	if name == "list-table" {
		return p.runListTableDirective(lines, i, next, lineBase, args, body), next
	}
	// Directive names are matched case-INSENSITIVELY — real docutils'
	// own directive registry does the same (directives.directive,
	// states.py, read directly) — ".. Attention::"/".. WARNING::" work
	// exactly like ".. attention::"/".. warning::".
	if tag, ok := admonitionTags[strings.ToLower(name)]; ok {
		return p.runAdmonitionDirective(tag, lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "admonition") {
		return p.runGenericAdmonitionDirective(lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "compound") {
		return p.runAdmonitionOrGeneric(doctree.TagCompound, "", lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "container") {
		return p.runContainerDirective(lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "header") {
		return p.runHeaderOrFooterDirective(true, lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "footer") {
		return p.runHeaderOrFooterDirective(false, lines, i, next, lineBase, args, body), next
	}
	if strings.EqualFold(name, "default-role") {
		return p.runDefaultRoleDirective(lines, i, next, args), next
	}
	if strings.EqualFold(name, "line-block") {
		return p.runLineBlockDirective(lines, i, lineBase, next, args, body), next
	}
	if strings.EqualFold(name, "math") {
		return p.runMathDirective(lines, i, next, args, body), next
	}
	if strings.EqualFold(name, "topic") {
		return p.runTopicOrSidebar(doctree.TagTopic, lines, i, lineBase, next, args, body, blankFinish, parent), next
	}
	if strings.EqualFold(name, "sidebar") {
		return p.runTopicOrSidebar(doctree.TagSidebar, lines, i, lineBase, next, args, body, blankFinish, parent), next
	}
	if strings.EqualFold(name, "image") {
		return p.runImageDirective(lines, i, next, args, body, ""), next
	}
	if strings.EqualFold(name, "figure") {
		return p.runFigureDirective(lines, i, next, lineBase, args, body), next
	}
	if isCodeDirectiveName(name) {
		return p.runCodeDirective(name, lines, i, next, args, body), next
	}
	if strings.EqualFold(name, "rubric") {
		return p.runRubricDirective(lines, i, next, args, body), next
	}
	if strings.EqualFold(name, "parsed-literal") {
		return p.runParsedLiteralDirective(lines, i, next, args, body), next
	}
	if strings.EqualFold(name, "meta") {
		// Meta declares no arguments and no option_spec at all, so real
		// docutils folds ANY same-line remainder into its own content
		// rather than treating it as an argument — a same-line
		// ".. meta:: :name: value" invocation is not corpus-tested and
		// not implemented here; args is deliberately ignored, matching
		// this project's own established "don't chase an untested,
		// unlikely-in-practice shape" scope discipline elsewhere.
		p.runMetaDirective(lines, i, body, i+1, strings.Join(lines[i:next], "\n"))
		return nil, next
	}
	if p.opts.ReportUnknownDirectives && !isImplementedDirective(name) {
		return unknownDirectiveDiagnostics(name, lines, i, next, lineBase), next
	}
	el := doctree.NewElement(doctree.TagDirective)
	el.SetAttr("name", name)
	if args != "" {
		el.SetAttr("arguments", args)
	}
	if len(body) > 0 {
		el.Append(&doctree.Text{Data: strings.Join(body, "\n")})
	}
	return []doctree.Node{el}, next
}

// bodyStartIndex is the absolute index of body[0] as gatherExplicitBody
// returns it: the first non-blank line under the directive, that
// function having trimmed the leading blanks. A caller whose content is
// a SUFFIX of body (the class and table directives, which split options
// off themselves rather than through parseDirectiveBlock) turns a
// content index into a source line with it.
func bodyStartIndex(lines []string, i int) int {
	j := i + 1
	for j < len(lines) && isBlankStr(lines[j]) {
		j++
	}
	return j
}

// unknownDirectiveDiagnostics ports the pair docutils raises for a
// directive name it cannot resolve: directives.directive() reports the
// lookup failure as an INFO, then Body.unknown_directive reports the
// ERROR. Both name the DEFAULT language module explicitly, which is why
// the corpus's German admonition fixtures land here rather than in any
// localization gap: docutils' own default language is English, so
// ".. Achtung::" is simply an unknown directive to it too.
//
// The ERROR's <literal_block> is the directive's WHOLE source, marker
// line and original indentation included — unknown_directive calls
// get_first_known_indented(0, strip_indent=False), not the dedenting
// form every implemented directive's body goes through.
func unknownDirectiveDiagnostics(name string, lines []string, i, next, lineBase int) []doctree.Node {
	// Body.unknown_directive quotes get_first_known_indented(0)'s block,
	// which runs through EVERY blank line under the directive; only the
	// one that TERMINATES the block is dropped. So one blank line after
	// the directive leaves nothing behind, two leave one blank inside the
	// quoted block, three leave two -- checked against the reference for
	// each of those. Trimming them all, as this did, is right for a
	// single blank and wrong for every larger run: 68 real-world files
	// (a PEP convention of two blank lines before a section) differed on
	// exactly this.
	end := next
	for end < len(lines) && isBlankStr(lines[end]) {
		end++
	}
	// Nothing is trimmed: joining the list keeps one "\n" per blank, and
	// Text.pformat's own splitlines() drops exactly one trailing empty
	// element at PRINT time (see doctree.Dump) -- which is where the
	// asymmetry lives. Trimming here as well removed it twice.
	return unknownDirectiveBlockDiagnostics(name,
		strings.Join(lines[i:end], "\n"), msgLine(i, lineBase))
}

// unknownDirectiveBlockDiagnostics is the same pair for a caller that
// already has the directive's own source block — a substitution
// definition, whose block starts AFTER the "|name| " marker rather than
// at the ".." line.
func unknownDirectiveBlockDiagnostics(name, block string, lineno int) []doctree.Node {
	info := sectionMessage("1", "INFO",
		`No directive entry for "`+name+`" in module "docutils.parsers.rst.languages.en".`+
			"\n"+`Trying "`+name+`" as canonical directive name.`, lineno, "")
	err := sectionMessage("3", "ERROR", `Unknown directive type "`+name+`".`, lineno, block)
	return []doctree.Node{info, err}
}

// parseComment ports Body.comment (states.py, read directly): like
// footnote/citation, a comment's body uses
// get_first_known_indented(match.end()) — gatherFootnoteBody, reused
// here verbatim (the SAME mechanism, not gatherExplicitBody's fixed
// 3-column floor, which this used to call and which required the body
// to start on a SEPARATE line at least 3 columns deep; a comment's own
// body has no such floor, any positive indent counts) — trailing blank
// entries are trimmed before joining (real docutils' own "while
// indented and not indented[-1].strip(): indented.trim_end()"), since
// unlike a footnote/citation's body — which gets re-parsed, so a
// trailing blank line is a harmless no-op — a comment's body is stored
// as raw, verbatim text and a leftover trailing blank would show up as
// a real trailing newline in it.
func (p *parser) parseComment(lines []string, i, lineBase int, rest string) ([]doctree.Node, int) {
	body, blankFinish, next := gatherFootnoteBody(lines, i)
	for len(body) > 0 && isBlankStr(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	text := strings.TrimSpace(rest)
	if len(body) > 0 {
		if text != "" {
			text += "\n"
		}
		text += strings.Join(body, "\n")
	}
	var el *doctree.Element
	if text == "" {
		el = doctree.NewElement(doctree.TagComment)
	} else {
		el = doctree.NewElement(doctree.TagComment, &doctree.Text{Data: text})
	}
	nodes := []doctree.Node{el}
	stoppedOnExplicitMarkup := next < len(lines) && isExplicitMarkupLine(lines[next])
	if !blankFinish && !stoppedOnExplicitMarkup {
		nodes = append(nodes, sectionMessage("2", "WARNING",
			"Explicit markup ends without a blank line; unexpected unindent.", msgLine(next, lineBase), ""))
	}
	return nodes, next
}

// parseHyperlinkTarget recognizes ".. _name: uri", where uri may
// continue on subsequent body lines (concatenated with no added
// whitespace, matching how docutils reconstructs a wrapped URI). When the
// value is itself a bare "othername_" reference rather than a URI, this is
// an INDIRECT target (docutils' parse_target/is_reference): "refname" is
// set instead of "refuri", and resolveTargets chases through it to find
// the final URI. Every NAMED explicit target gets an "id" too (real
// docutils' own Target.run always calls set_id on it — verified directly
// against the foreign judge, even for the bare, unreferenced, no-
// substitution case) via explicitTargetID, the SAME footnote-1/citation-1-
// style positional-fallback helper footnote/citation ids already use —
// this function's own doc comment already anticipated hyperlink targets
// needing it ("currently only footnote.go/explicit.go's footnote/citation
// dispatch calls this"), just never wired up until now: previously
// missing entirely, a real, previously-unnoticed gap (only 11 of 579
// corpus fixtures happen to expect ids= on a <target> at all, and every
// one of THOSE reaches a different code path — an inline target or an
// embedded URI, both of which already set id correctly — so this
// specific construct's own gap stayed invisible until a fixture combined
// it with a substitution reference).
func (p *parser) parseHyperlinkTarget(lines []string, i int, rest string) (doctree.Node, int) {
	body, _, next := gatherExplicitBody(lines, i)
	// The name ends at the first colon FOLLOWED BY whitespace or the end
	// of the line -- not at the first colon of any kind. A reference name
	// may itself contain colons (".. _figure:caption:" is the single name
	// "figure:caption"), and so may the URI ("http://..."), so neither
	// "first colon" nor "last colon" is right. Checked against the
	// reference for "figure:caption:", "a:b:c: http://x", "name: uri" and
	// a bare "plain:".
	name, uri := rest, ""
	if phrase, after, ok := splitPhraseTargetName(rest); ok {
		// A PHRASE target: Body.patterns.target opens with an optional
		// backquote and, when one is used, the name runs to its match --
		// so the backquotes are delimiters, not part of the name, and a
		// colon INSIDE them does not terminate it. ".. _`with: colon`:"
		// is the single name "with: colon".
		name, uri = phrase, after
	} else if idx := targetNameEnd(rest); idx >= 0 {
		name = rest[:idx]
		uri = rest[idx+1:]
	}
	name = strings.TrimSpace(name)
	uri = strings.TrimSpace(uri)
	for _, l := range body {
		uri += strings.TrimSpace(l)
	}
	normalized := normalizeName(name)
	el := doctree.NewElement(doctree.TagTarget)
	el.SetAttr("name", normalized)
	el.SetAttr("id", p.explicitTargetID("target", normalized))
	if indirect, ok := bareIndirectTargetName(uri); ok {
		el.SetAttr("refname", normalizeName(indirect))
	} else if uri != "" {
		// A target with NO uri at all ("..  _figure:caption:") carries
		// neither attribute -- it is an internal anchor. Setting an empty
		// refuri produced refuri="" where docutils omits it entirely.
		el.SetAttr("refuri", uri)
	}
	return el, next
}

// parseAnonymousTarget recognizes ".. __: uri" — a target with no name at
// all. Unlike a named target, it resolves by DOCUMENT-ORDER POSITION: the
// Nth anonymous reference (bare "x__", or "`x`__" — see tryBareReference/
// referenceOrPhrase) matches the Nth anonymous target, textual order,
// regardless of any text either one carries (verified against real
// docutils: this holds whether the targets appear before or after their
// references). resolveTargets implements the matching. The value may
// itself be a bare "othername_" reference rather than a URI — an INDIRECT
// anonymous target (".. __: othername_") — chased the same way a named
// indirect target is (see bareIndirectTargetName); verified against real
// docutils that an anonymous reference still consumes its document-order
// slot even when the chase fails to resolve (it just ends up with no
// refuri set, this codebase's usual treatment of an unresolved reference,
// rather than docutils' own refid-to-the-target-itself fallback, which
// depends on error-reporting machinery not implemented here).
// isAnonymousTargetLine matches docutils' Body pattern `__( +|$)`: the
// SHORTHAND spelling of an anonymous hyperlink target, with no ".. "
// prefix at all. It is a Body-state transition of its own, listed
// between explicit_markup and line, so it only ever fires at the start
// of a block -- a "__ " inside a paragraph is ordinary text, which
// consumeParagraph already handles by ignoring a continuation line's
// shape.
func isAnonymousTargetLine(s string) bool {
	if !strings.HasPrefix(s, "__") {
		return false
	}
	rest := s[2:]
	return rest == "" || strings.HasPrefix(rest, " ")
}

func (p *parser) parseAnonymousTarget(lines []string, i int, rest string) (doctree.Node, int) {
	body, _, next := gatherExplicitBody(lines, i)
	uri := strings.TrimSpace(rest)
	for _, l := range body {
		uri += strings.TrimSpace(l)
	}
	el := doctree.NewElement(doctree.TagTarget)
	el.SetAttr("anonymous", anonymousAttrValue)
	// note_anonymous_target calls set_id like every other target, so an
	// anonymous one is "target-1", "target-2", ... -- neither spelling
	// carried an id here at all.
	el.SetAttr("id", p.autoID("target"))
	if indirect, ok := bareIndirectTargetName(uri); ok {
		el.SetAttr("refname", normalizeName(indirect))
	} else if uri != "" {
		// A target with NO uri at all ("..  _figure:caption:") carries
		// neither attribute -- it is an internal anchor. Setting an empty
		// refuri produced refuri="" where docutils omits it entirely.
		el.SetAttr("refuri", uri)
	}
	return el, next
}

// bareIndirectTargetName reports whether uri, taken as a WHOLE, is a bare
// "othername_" reference (docutils' parse_target: the target's value ends
// in "_" AND the entire value matches the simplename grammar —
// scanSimpleName in inline.go). A real URI ending in "_", like
// ".../foo_", does NOT match, since "/" isn't a valid simplename
// separator. The backtick-quoted phrase form ("`other name`_") is not
// implemented here: rare for a target's own value, unlike a reference's.
func bareIndirectTargetName(uri string) (string, bool) {
	if len(uri) < 2 || uri[len(uri)-1] != '_' {
		return "", false
	}
	runes := []rune(uri[:len(uri)-1])
	if len(runes) == 0 {
		return "", false
	}
	if scanSimpleName(runes, 0) != len(runes) {
		return "", false
	}
	return string(runes), true
}

// normalizeName mirrors docutils.nodes.fully_normalize_name: case- and
// whitespace-insensitive matching between a reference and its target.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// resolveTargets walks the tree once to collect every hyperlink target —
// named ones by normalized name, split into "direct" (has its own refuri,
// or is an inline internal target with none, resolved to a same-document
// anchor) and "indirect" (its value is itself another target's name,
// chased through resolveIndirect until a direct one is reached), and
// anonymous ones (".. __: uri") into a separate document-order list — then
// walks it again setting refuri on every reference: by name for a named
// one, or by consuming the next unused anonymous target in document order
// for an anonymous one. A NAMED reference (bare, backtick-quoted, or
// embedded-indirect-alias) whose name matches no target anywhere is
// rewritten to a <problematic> in place, docutils' own DanglingReferences
// transform, simplified: no duplicate/ambiguous-name diagnostics, and the
// <problematic>'s content is the reference's own VISIBLE text rather than
// real docutils' verbatim source slice (rawsource) — this parser doesn't
// track original source text on a reference node at all (see the package
// SCOPE note on ids/source attributes), so "broken_" resynthesizes as
// "broken", the same category of simplification already accepted
// elsewhere in this project (go-richdoc/rst's own Raw* reconstruction
// helpers) rather than a new one invented for this feature. An ANONYMOUS
// reference is checked differently, matching real docutils'
// AnonymousHyperlinks.apply (transforms/references.py, read directly, not
// guessed from behavior — an earlier pass at this got the direction of
// the check wrong): the count of anonymous references against the count
// of anonymous targets is a single, whole-document condition (`!=`, not
// merely "too many refs"), checked once — if they don't match EXACTLY,
// EVERY anonymous reference in the document becomes <problematic>,
// regardless of which side has the surplus, all sharing ONE message
// (docutils' own "Anonymous hyperlink mismatch" error).
// resolveTargets links every reference to its target's URI and rewrites
// dangling/mismatched ones to <problematic>, collecting its OWN
// dangling-reference/anonymous-mismatch messages into one trailing
// <section class="system-messages"> — these, unlike parseInline's
// paragraph/title-time messages (already attached at their point of
// origin by their own callers, see parser.go/inline.go), are genuinely
// parentless: real docutils' DanglingReferencesVisitor builds them via
// document.reporter.error with no tree insertion at all (transforms/
// references.py, read directly), so they fall to transforms.universal.
// Messages' "loose messages" wrap, which this function's own
// systemMessagesSection replicates. initMsgCount seeds the id counter
// with whatever markupProblematic (inline.go) already assigned during
// parsing, so both sources share ONE continuous "problematic-N"/
// "system-message-N" sequence instead of two independently-numbered
// ones — real docutils' own Messages transform merges document.
// parse_messages (ids assigned first, parsing finishes before any
// transform runs) with document.transform_messages (this function's own,
// assigned next).
func resolveTargets(doc *doctree.Element, initMsgCount int, reportDangling, resolve bool) {
	direct := map[string]string{}
	indirect := map[string]string{}
	var anonTargets []anonTarget
	collectTargets(doc, direct, indirect, &anonTargets)
	targets := map[string]string{}
	for name := range direct {
		targets[name] = direct[name]
	}
	for name := range indirect {
		if uri, ok := resolveIndirect(name, direct, indirect, 0); ok {
			targets[name] = uri
		}
	}
	anonURIs := make([]string, len(anonTargets))
	for i, t := range anonTargets {
		switch {
		case t.refuri != "":
			anonURIs[i] = t.refuri
		case t.refname != "":
			if uri, ok := resolveIndirect(t.refname, direct, indirect, 0); ok {
				anonURIs[i] = uri
			}
		}
	}
	anonIndex := 0
	var messages []*doctree.Element
	msgCount := initMsgCount
	var anonMismatch *doctree.Element
	if n := countAnonymousReferences(doc); reportDangling && n != len(anonURIs) {
		msgCount++
		anonMismatch = doctree.NewElement(doctree.TagSystemMessage,
			doctree.NewElement(doctree.TagParagraph, &doctree.Text{
				Data: fmt.Sprintf(`Anonymous hyperlink mismatch: %d references but %d targets.`, n, len(anonURIs)),
			}))
		anonMismatch.SetAttr("id", "system-message-"+strconv.Itoa(msgCount))
		messages = append(messages, anonMismatch)
	}
	linkReferences(doc, targets, anonURIs, anonMismatch, &anonIndex, &messages, &msgCount, reportDangling, resolve)
	if len(messages) > 0 {
		doc.Append(systemMessagesSection(messages))
	}
}

// countAnonymousReferences counts every anonymous reference (bare "x__",
// backtick-quoted "`x`__", or a substitution used as one, "|x|__" — see
// inline.go's tryMarker) anywhere in the document, needed BEFORE the main
// walk since the mismatch check below is whole-document, not incremental.
func countAnonymousReferences(n doctree.Node) int {
	el, ok := n.(*doctree.Element)
	if !ok {
		return 0
	}
	count := 0
	if el.Tag == doctree.TagReference && el.Attr("anonymous") == anonymousAttrValue {
		count++
	}
	for _, c := range el.Children {
		count += countAnonymousReferences(c)
	}
	return count
}

// anonTarget is one ".. __: ..." target in document order: either a direct
// URI or, for an indirect anonymous target, the name it chases.
type anonTarget struct {
	refuri, refname string
}

func collectTargets(n doctree.Node, direct, indirect map[string]string, anonTargets *[]anonTarget) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return
	}
	if el.Tag == doctree.TagTarget {
		switch {
		case el.Attr("anonymous") == anonymousAttrValue:
			*anonTargets = append(*anonTargets, anonTarget{refuri: el.Attr("refuri"), refname: el.Attr("refname")})
		case el.Attr("name") != "":
			name := el.Attr("name")
			switch {
			case el.Attr("refuri") != "":
				direct[name] = el.Attr("refuri")
			case el.Attr("refname") != "":
				indirect[name] = el.Attr("refname")
			default:
				// An inline internal target ("_`text`") carries no URI of
				// its own — it IS the destination, a same-document anchor
				// a reference resolves to by pointing at its own SLUGIFIED
				// id (tryInlineTarget's own "id" attribute, matching every
				// other target kind), falling back to the raw name only in
				// the vanishingly rare case makeID produces an empty slug
				// (e.g. a name with no ASCII alphanumeric content at all).
				id := el.Attr("id")
				if id == "" {
					id = name
				}
				direct[name] = "#" + id
			}
		}
	}
	if el.Tag != doctree.TagFootnote && el.Tag != doctree.TagCitation && el.Tag != doctree.TagTarget {
		// ANY directive/section carrying both a "name" and an "id" is an
		// implicit same-document hyperlink target, not just explicit
		// <target> elements or section titles — real docutils' own
		// Directive.add_name (states.py, read directly) calls
		// document.note_explicit_target for EVERY directive that accepts
		// a :name: option (note, table, code, container, ... — verified
		// directly against the foreign judge across several of them, not
		// just container, which is what surfaced this), registering the
		// node itself as a target, not just decorating it with an id/name
		// attribute pair. Footnotes/citations are excluded: they set
		// "name" for their own numbering/labeling purposes (footnotenum.go)
		// via an entirely separate resolution path (resolveFootnoteNumbers),
		// not because real docutils excludes them — this project has never
		// corpus-tested a plain `` `label`_ `` reference to a footnote's
		// own name, so folding them in here is unverified and left alone.
		// TagTarget is ALSO excluded — added when parseHyperlinkTarget
		// started setting "id" on every named target (v0.46.0): a
		// <target> already has its own MORE SPECIFIC handling directly
		// above (refuri direct, refname indirect, or "#"+id for the bare
		// inline-target case) — this generic rule blindly overwriting it
		// with "#"+id unconditionally was a real regression this fix
		// caught immediately via the existing test suite (a target's own
		// refuri got clobbered into a same-document self-reference).
		// Duplicate names ARE diagnosed now (dupnames.go, v0.57.0+), and
		// that pass runs BEFORE this one — so by the time collectTargets
		// walks the tree, every element invalidated by a name collision
		// has already had its "name" moved to "dupname" and simply isn't
		// seen here. Last-write-wins below therefore only ever applies to
		// names that survived the transition table.
		if name := el.Attr("name"); name != "" && el.Attr("id") != "" {
			direct[name] = "#" + el.Attr("id")
		}
	}
	for _, c := range el.Children {
		collectTargets(c, direct, indirect, anonTargets)
	}
}

// resolveIndirect chases an indirect target's refname chain
// ("a" -> "b" -> "c" -> a real refuri) to its final URI. depth guards
// against a cycle ("a" -> "b" -> "a") — checked against the foreign judge:
// real docutils does NOT report this as an error at all, it resolves
// SILENTLY to an odd same-document self-reference (the reference ends up
// pointing at its own cycle's first target by id) that this project
// doesn't replicate; this simplified version just leaves a cycle
// unresolved, same as any other name with no matching target, which
// (since problematic-rewriting was added) now surfaces it as a dangling
// reference — a real practical difference from real docutils' silence,
// but arguably a more honest one for a document that has an actual bug in
// it.
func resolveIndirect(name string, direct, indirect map[string]string, depth int) (string, bool) {
	if depth > 20 {
		return "", false
	}
	if uri, ok := direct[name]; ok {
		return uri, true
	}
	if next, ok := indirect[name]; ok {
		return resolveIndirect(next, direct, indirect, depth+1)
	}
	return "", false
}

// linkReferences resolves every reference under parent. It walks by
// PARENT rather than by node (unlike the rest of this file's tree walks)
// because a dangling reference needs to REPLACE itself in its parent's own
// child slice, not just gain an attribute. anonMismatch is non-nil exactly
// when resolveTargets found the anonymous reference and target counts
// don't match document-wide — in that case EVERY anonymous reference
// becomes
// <problematic>, sharing anonMismatch as their one <system_message>
// (docutils' own "Anonymous hyperlink mismatch", a whole-document
// condition, not a per-reference one — see resolveTargets); otherwise
// anonymous references resolve by position exactly as before.
func linkReferences(parent *doctree.Element, targets map[string]string, anonTargets []string, anonMismatch *doctree.Element, anonIndex *int, messages *[]*doctree.Element, msgCount *int, reportDangling, resolve bool) {
	for i, c := range parent.Children {
		el, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		if el.Tag == doctree.TagReference {
			switch {
			case el.Attr("anonymous") == anonymousAttrValue:
				if anonMismatch != nil {
					parent.Children[i] = problematicAnonymousReference(el, anonMismatch, msgCount)
					continue // the replacement has no children of its own to recurse into
				}
				if *anonIndex < len(anonTargets) {
					if uri := anonTargets[*anonIndex]; uri != "" && resolve {
						el.SetAttr("refuri", uri)
					}
					*anonIndex++
				}
			case el.Attr("refname") != "":
				if uri, found := targets[normalizeName(el.Attr("refname"))]; found {
					if resolve {
						el.SetAttr("refuri", uri)
					}
				} else if reportDangling {
					parent.Children[i] = problematicReference(el, messages, msgCount)
					continue // the replacement has no children of its own to recurse into
				}
			}
		}
		linkReferences(el, targets, anonTargets, anonMismatch, anonIndex, messages, msgCount, reportDangling, resolve)
	}
}

// problematicAnonymousReference builds the <problematic> for one
// mismatched anonymous reference, pointing at the single SHARED msg — and
// grows msg's own "backref" attribute to list every problematic that
// points to it, real docutils' own "See backrefs attribute for IDs"
// convention (space-joined here rather than a real attribute list, this
// project's doctree.Element only carries string-valued attributes).
func problematicAnonymousReference(ref *doctree.Element, msg *doctree.Element, msgCount *int) *doctree.Element {
	*msgCount++
	prbID := "problematic-" + strconv.Itoa(*msgCount)
	prb := doctree.NewElement(doctree.TagProblematic, &doctree.Text{Data: doctree.AsText(ref)})
	prb.SetAttr("id", prbID)
	prb.SetAttr("refid", msg.Attr("id"))
	if existing := msg.Attr("backref"); existing == "" {
		msg.SetAttr("backref", prbID)
	} else {
		msg.SetAttr("backref", existing+" "+prbID)
	}
	return prb
}

// problematicReference builds the <problematic>/<system_message> pair for
// one dangling named reference, cross-linked by id/refid/backref the same
// way real docutils' pair is (see resolveTargets) — appends the message to
// *messages for resolveTargets to collect into a trailing section, and
// returns the <problematic> to replace the reference with in place.
func problematicReference(ref *doctree.Element, messages *[]*doctree.Element, msgCount *int) *doctree.Element {
	*msgCount++
	n := strconv.Itoa(*msgCount)
	prb := doctree.NewElement(doctree.TagProblematic, &doctree.Text{Data: doctree.AsText(ref)})
	prb.SetAttr("id", "problematic-"+n)
	prb.SetAttr("refid", "system-message-"+n)
	msg := doctree.NewElement(doctree.TagSystemMessage,
		doctree.NewElement(doctree.TagParagraph, &doctree.Text{Data: `Unknown target name: "` + ref.Attr("refname") + `".`}))
	msg.SetAttr("id", "system-message-"+n)
	msg.SetAttr("backref", "problematic-"+n)
	*messages = append(*messages, msg)
	return prb
}

// systemMessagesSection collects every dangling-reference message into one
// trailing section, docutils' own Messages transform: "loose" system
// messages not otherwise attached to the tree get a dedicated section of
// their own, appended at the very end of the document.
func systemMessagesSection(messages []*doctree.Element) *doctree.Element {
	sec := doctree.NewElement(doctree.TagSection)
	sec.SetAttr("class", "system-messages")
	sec.Append(doctree.NewElement(doctree.TagTitle, &doctree.Text{Data: "Docutils System Messages"}))
	for _, m := range messages {
		sec.Append(m)
	}
	return sec
}

// anonymousAttrValue is "1", not "true". The attribute is a BOOLEAN in
// docutils (reference['anonymous'] = True), and nodes.Element.starttag
// renders a bool as str(int(value)) — so every pseudoxml dump, and the
// corpus with it, spells it anonymous="1". Thirteen fixtures differed
// from this parser on nothing else at all.
const anonymousAttrValue = "1"

// isCodeDirectiveName covers "code" and the two ALIASES docutils'
// English language module gives it, "code-block" and "sourcecode"
// (languages/en.py, read directly). That module is a layer this project
// had no equivalent of at all: a written directive name is looked up
// there FIRST and only the canonical name it returns reaches the
// registry, which is why the lookup-failure INFO names the module by
// path ("No directive entry for ... in module
// docutils.parsers.rst.languages.en").
//
// Of its four directive aliases these two were missing; "rst-class" and
// "section-numbering" were already handled at their own call sites, and
// deliberately are NOT routed through here -- runClassDirective needs
// the name as WRITTEN, since "class" and "rst-class" produce different
// details. All eleven ROLE aliases were already in roleTags.
//
// Invisible in the docutils testsuite corpus, which never writes
// ".. code-block::"; 130 of the real-world corpus's files do.
func isCodeDirectiveName(name string) bool {
	switch strings.ToLower(name) {
	case "code", "code-block", "sourcecode":
		return true
	}
	return false
}

// isImplementedDirective reports whether this package has real semantics
// for a directive name, however that dispatch turned out for the
// particular invocation. Reaching the generic capture with an
// IMPLEMENTED name means the directive failed its own validation --
// ".. raw::" with no format argument, say -- and docutils reports that as
// `Error in "raw" directive: ...`, NOT as an unknown directive type. This
// package does not implement per-directive argument/option validation
// (see the README), so such an invocation still falls back to the
// structural capture; what it must not do is claim the name is unknown.
func isImplementedDirective(name string) bool {
	if _, ok := admonitionTags[strings.ToLower(name)]; ok {
		return true
	}
	switch strings.ToLower(name) {
	case testDirectiveName, "code-block", "sourcecode":
		return true
	}
	switch strings.ToLower(name) {
	case "admonition", "class", "code", "compound", "container", "date",
		"default-role", "figure", "footer", "header", "image", "line-block",
		"list-table", "math", "meta", "parsed-literal", "raw", "replace",
		"role", "rst-class", "rubric", "section-numbering", "sectnum",
		"sidebar", "table", "target-notes", "title", "topic":
		return true
	}
	return false
}

// hasBlankSeparatedParagraphs reports whether body holds a blank line
// with real content on BOTH sides -- i.e. more than one paragraph. Its
// leading and trailing blanks have already been trimmed by the caller.
func hasBlankSeparatedParagraphs(body []string) bool {
	seen := false
	for _, l := range body {
		if isBlankStr(l) {
			if seen {
				return true
			}
			continue
		}
		seen = true
	}
	return false
}

// splitPhraseTargetName recognizes the BACKQUOTED form of a hyperlink
// target name and returns the name without its quotes, plus whatever
// follows the terminating colon. Body.patterns.target makes the quote
// optional and pairs it with a back-reference, so an opened quote must
// be closed and everything between is the name -- colons included.
//
// The quotes were being kept IN the name (name="`tmp_path handling`"),
// which is how pytest and sphinx write every phrase target; the id was
// already right, since make_id drops them anyway.
func splitPhraseTargetName(rest string) (name, after string, ok bool) {
	if !strings.HasPrefix(rest, "`") {
		return "", "", false
	}
	for i := 1; i < len(rest); i++ {
		if rest[i] == '\\' {
			i++ // an escaped character cannot close the quote
			continue
		}
		if rest[i] != '`' {
			continue
		}
		tail := rest[i+1:]
		tail = strings.TrimPrefix(tail, " ")
		if !strings.HasPrefix(tail, ":") {
			return "", "", false
		}
		return rest[1:i], tail[1:], true
	}
	return "", "", false
}

// targetNameEnd returns the index of the colon terminating a hyperlink
// target's name, or -1 if there is none.
func targetNameEnd(rest string) int {
	for i := 0; i < len(rest); i++ {
		if rest[i] != ':' {
			continue
		}
		if i+1 == len(rest) || rest[i+1] == ' ' || rest[i+1] == '\t' {
			return i
		}
	}
	return -1
}
