package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Option lists (man-page-style "-f, --file=ARG  Description." items),
// modeled on docutils' Body.option_marker/option_list_item/
// parse_option_marker (states.py). Previously deferred (see git history)
// as complex relative to how rarely option lists appear outside CLI docs;
// picked up once field/definition lists and tables had proven out the
// shared marker+indented-continuation machinery this reuses directly
// (gatherListItemLines, the same helper field lists use).

// optionToken is one parsed "-f ARG" / "--long=ARG" / "-f" element of a
// (possibly comma-separated) option group.
type optionToken struct {
	Flag      string
	Arg       string
	Delimiter string // " ", "=", or "" ("-fARG"); "" (no arg) when Arg == ""
}

// matchOptionMarker recognizes an option-list marker at the start of a
// line: one or more comma-separated options (see parseOptionGroup) followed
// by two or more spaces or end of line, mirroring docutils' option_marker
// pattern. contentCol is where same-line description text would start (it
// may be len(line), meaning none).
func matchOptionMarker(line string) (opts []optionToken, contentCol int, ok bool) {
	markerEnd := optionMarkerEnd(line)
	if markerEnd < 0 {
		return nil, 0, false
	}
	marker := line[:markerEnd]
	opts, ok = parseOptionGroup(marker)
	if !ok || len(opts) == 0 {
		return nil, 0, false
	}
	col := markerEnd
	for col < len(line) && line[col] == ' ' {
		col++
	}
	return opts, col, true
}

// optionMarkerEnd scans line for the boundary between the option marker and
// any same-line description: two or more consecutive spaces outside a
// "<...>" bracketed argument, or end of line. Returns -1 if line is empty or
// starts with whitespace (never a marker).
func optionMarkerEnd(line string) int {
	if line == "" || line[0] == ' ' {
		return -1
	}
	depth := 0
	i := 0
	for i < len(line) {
		switch line[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth == 0 {
				j := i
				for j < len(line) && line[j] == ' ' {
					j++
				}
				if j-i >= 2 {
					return i
				}
				i = j
				continue
			}
		}
		i++
	}
	return len(line)
}

// parseOptionGroup splits marker on ", " outside "<...>" (docutils:
// `re.split(r', (?![^<]*>)', ...)`) and tokenizes each piece.
func parseOptionGroup(marker string) ([]optionToken, bool) {
	var opts []optionToken
	for _, piece := range splitOptionGroup(marker) {
		tok, ok := parseOptionToken(piece)
		if !ok {
			return nil, false
		}
		opts = append(opts, tok)
	}
	return opts, true
}

func splitOptionGroup(marker string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(marker); i++ {
		switch marker[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 && i+1 < len(marker) && marker[i+1] == ' ' {
				parts = append(parts, marker[start:i])
				start = i + 2
			}
		}
	}
	parts = append(parts, marker[start:])
	return parts
}

// looksLikeOptionFlag reports whether s starts like a real option flag —
// docutils' `shortopt`/`longopt` patterns: ('-'|'+') + one alphanumeric
// (shortopt), or ('--'|'/') + an alphanumeric-leading name (longopt). This
// is the check parseOptionToken was missing entirely: without it, an
// ordinary two-word paragraph like "First paragraph." tokenizes exactly like
// a "flag argument" pair and gets swallowed as an option list.
func looksLikeOptionFlag(s string) bool {
	switch {
	case strings.HasPrefix(s, "--"):
		return len(s) > 2 && isAlphaNumByte(s[2])
	case strings.HasPrefix(s, "/"):
		return len(s) > 1 && isAlphaNumByte(s[1])
	case len(s) > 1 && (s[0] == '-' || s[0] == '+'):
		return isAlphaNumByte(s[1])
	default:
		return false
	}
}

func isAlphaNumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// parseOptionToken tokenizes one option string ("-f", "-f FILE", "-ovalue",
// "--output", "--output=FILE", "-o <v1 v2>") into a flag/argument pair,
// mirroring docutils' parse_option_marker token-fixup rules exactly.
func parseOptionToken(s string) (optionToken, bool) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 || !looksLikeOptionFlag(tokens[0]) {
		return optionToken{}, false
	}
	delimiter := " "
	if flag, arg, ok := strings.Cut(tokens[0], "="); ok {
		tokens = append([]string{flag, arg}, tokens[1:]...)
		delimiter = "="
	} else if len(tokens[0]) > 2 && ((tokens[0][0] == '-' && !strings.HasPrefix(tokens[0], "--")) || tokens[0][0] == '+') {
		tokens = append([]string{tokens[0][:2], tokens[0][2:]}, tokens[1:]...)
		delimiter = ""
	}
	if len(tokens) > 1 && strings.HasPrefix(tokens[1], "<") && strings.HasSuffix(tokens[len(tokens)-1], ">") {
		tokens = []string{tokens[0], strings.Join(tokens[1:], " ")}
	}
	switch len(tokens) {
	case 1:
		return optionToken{Flag: tokens[0]}, true
	case 2:
		return optionToken{Flag: tokens[0], Arg: tokens[1], Delimiter: delimiter}, true
	default:
		return optionToken{}, false
	}
}

// parseOptionList consumes consecutive option-list items starting at
// lines[i]. An option marker with no following content at all (neither on
// its own line nor indented beneath it) is not really an option list item —
// docutils falls back to ordinary paragraph text (TransitionCorrection), so
// ok is false and the caller should try other block types instead.
func (p *parser) parseOptionList(lines []string, i int) (el *doctree.Element, next int, ok bool) {
	ol := doctree.NewElement(doctree.TagOptionList)
	start := i
	for i < len(lines) {
		opts, col, matched := matchOptionMarker(lines[i])
		if !matched {
			break
		}
		first := ""
		if len(lines[i]) > col {
			first = lines[i][col:]
		}
		bodyLines, n := gatherListItemLines(lines, i, col, first)
		if len(bodyLines) == 0 {
			break
		}
		group := doctree.NewElement(doctree.TagOptionGroup)
		for _, opt := range opts {
			group.Append(optionNode(opt))
		}
		desc := doctree.NewElement(doctree.TagDescription)
		p.parseBlockLines(bodyLines, desc, -1)
		item := doctree.NewElement(doctree.TagOptionListItem, group, desc)
		ol.Append(item)
		i = n
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	if i == start {
		return nil, start, false
	}
	return ol, i, true
}

func optionNode(opt optionToken) *doctree.Element {
	el := doctree.NewElement(doctree.TagOption)
	el.Append(doctree.NewElement(doctree.TagOptionString, &doctree.Text{Data: opt.Flag}))
	if opt.Arg != "" {
		arg := doctree.NewElement(doctree.TagOptionArgument, &doctree.Text{Data: opt.Arg})
		arg.SetAttr("delimiter", opt.Delimiter)
		el.Append(arg)
	}
	return el
}
