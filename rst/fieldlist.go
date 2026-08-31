package rst

import "github.com/go-docutils/docutils/doctree"

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

// matchFieldMarker recognizes ":name: rest" at the start of a line,
// docutils' field_marker pattern `:(?![: ])...(?<! ):( +|$)` simplified:
// a leading ':' not immediately followed by ':' or ' ', then the name
// runs up to the next ':' that is itself followed by a space or EOL.
func matchFieldMarker(line string) (name string, contentCol int, ok bool) {
	if len(line) < 2 || line[0] != ':' || line[1] == ':' || line[1] == ' ' {
		return "", 0, false
	}
	for j := 1; j < len(line); j++ {
		if line[j] != ':' {
			continue
		}
		if j+1 == len(line) {
			return line[1:j], j + 1, true
		}
		if line[j+1] == ' ' {
			return line[1:j], j + 2, true
		}
	}
	return "", 0, false
}

func (p *parser) parseFieldList(lines []string, i int) (*doctree.Element, int) {
	fl := doctree.NewElement(doctree.TagFieldList)
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
		field := doctree.NewElement(doctree.TagField)
		nameNodes, nameMsgs := p.parseInline(name, 0)
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
	return fl, i
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
	if isBulletLine(lines[i]) || isEnumLine(lines[i]) || isExplicitMarkupLine(lines[i]) {
		return false
	}
	if _, _, ok := matchFieldMarker(lines[i]); ok {
		return false
	}
	if isDoctestLine(lines[i]) || isLineBlockLine(lines[i]) {
		return false
	}
	if _, isLine := isUniformLine(lines[i]); isLine {
		return false
	}
	return i+1 < len(lines) && !isBlankStr(lines[i+1]) && leadingSpaces(lines[i+1]) > 0
}

func (p *parser) parseDefinitionList(lines []string, i int) (*doctree.Element, int) {
	dl := doctree.NewElement(doctree.TagDefinitionList)
	for i < len(lines) && isDefinitionTermLine(lines, i) {
		term := trimTrailingSpace(lines[i])
		indent := leadingSpaces(lines[i+1])
		block, next := consumeIndentedBlock(lines, i+1, indent)
		item := doctree.NewElement(doctree.TagDefinitionListItem)
		termNodes, termMsgs := p.parseInline(term, 0)
		item.Append(doctree.NewElement(doctree.TagTerm, termNodes...))
		def := doctree.NewElement(doctree.TagDefinition)
		// real docutils' Text.definition_list_item: "dd = nodes.definition
		// ('', *messages)" — the term's own inline-markup messages become
		// the <definition>'s FIRST children, ahead of its own parsed
		// content (states.py, read directly).
		for _, m := range termMsgs {
			def.Append(m)
		}
		p.parseBlockLines(block, def)
		item.Append(def)
		dl.Append(item)
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	return dl, i
}
