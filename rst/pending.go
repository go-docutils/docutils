package rst

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// A <pending> node is what docutils leaves behind for a directive whose
// real work happens in a later TRANSFORM: ".. class::", ".. sectnum::",
// ".. target-notes::" and friends parse to a placeholder carrying the
// transform's name and its options, and the transform itself acts on the
// document afterwards.
//
// This project has no transform system and is not getting one — but that
// was never what these nodes needed. The corpus compares the PARSE tree,
// and at parse time a <pending> is simply a node with a formatted text
// child; nothing has to run. Reading nodes.pending.pformat directly (it
// is where the odd indentation below comes from) turned a group of
// mismatches filed for years under "needs a transform system" into a
// straightforward node construction.
//
// The format, from that method verbatim: a ".. internal attributes:"
// line, then ".transform:" indented 5, then ".details:" indented 5, then
// one line per detail indented 7 as "key: <repr>", with the details
// SORTED BY KEY and their values rendered the way Python's %r would.
func pendingNode(transform string, details []pendingDetail) *doctree.Element {
	lines := []string{
		".. internal attributes:",
		"     .transform: " + transform,
		"     .details:",
	}
	sort.Slice(details, func(a, b int) bool { return details[a].key < details[b].key })
	for _, d := range details {
		lines = append(lines, fmt.Sprintf("       %s: %s", d.key, d.value))
	}
	return doctree.NewElement(doctree.TagPending,
		&doctree.Text{Data: strings.Join(lines, "\n")})
}

// pendingDetail is one already-rendered detail: value carries Python repr
// form, since that is what the reference emits (a string is quoted, an
// integer is not, a list of strings prints as ['a', 'b']).
type pendingDetail struct{ key, value string }

// pyRepr renders a string the way Python's %r does. The quote-switching
// rule is real: a string holding a single quote and no double quote is
// printed in DOUBLE quotes and its single quote left bare; otherwise
// single quotes, with backslashes and any single quote escaped.
// Non-printing characters take their usual escapes.
//
// This used to be the naive "'"+s+"'" with a note saying no option value
// here could hold a quote. That was true of the directives that existed
// then; misc.TestDirective echoes an ARBITRARY argument back, so it is
// not true any more.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case quote:
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func pyReprList(items []string) string {
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = pyRepr(it)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// runClassDirective ports directives.misc.Class: the class names are
// normalized the same way every other :class: option is, and the
// directive's own name is recorded because "class" and its alias
// "rst-class" produce different details.
func (p *parser) runClassDirective(name, args string, body []string, bodyStart, lineBase int) []doctree.Node {
	// NOT parseDirectiveBlock: gatherExplicitBody has already trimmed the
	// blank line that separates a same-line argument from the content, so
	// that split would run past it and read the first content paragraph as
	// more class names. Class declares exactly ONE argument and NO options,
	// so the division needs no scanning: the argument is the same-line
	// text when there is any, else the first body line.
	argument, content := args, body
	if strings.TrimSpace(argument) == "" {
		argument, content = "", nil
		for i, l := range body {
			if !isBlankStr(l) {
				argument, content = l, body[i+1:]
				break
			}
		}
	}
	for len(content) > 0 && isBlankStr(content[0]) {
		content = content[1:]
	}
	var classes []string
	for _, f := range strings.Fields(argument) {
		classes = append(classes, makeID(f))
	}

	// WITH content, Class.run parses it and adds the classes to each
	// resulting top-level node, returning those nodes and NO <pending> at
	// all -- the transform exists only for the contentless form, whose job
	// is to reach the NEXT element instead. This used to discard the
	// content split entirely, so the body's own words were read as further
	// class names: ".. class:: c1 c2" over a paragraph produced classes
	// ['c1','c2','the','classes','are','applied', ...].
	if len(content) > 0 {
		container := doctree.NewElement(doctree.TagDocument)
		// content is a SUFFIX of body, so its first line is
		// bodyStart + however many body lines were consumed as the
		// argument or trimmed as blanks.
		p.parseBlockLines(content, container, nestedLineBase(bodyStart+len(body)-len(content), lineBase))
		out := make([]doctree.Node, 0, len(container.Children))
		for _, c := range container.Children {
			if ce, ok := c.(*doctree.Element); ok && len(classes) > 0 {
				ce.SetAttr("class", strings.Join(classes, " "))
			}
			out = append(out, c)
		}
		return out
	}

	details := []pendingDetail{{"directive", pyRepr(strings.ToLower(name))}}
	if len(classes) > 0 {
		details = append(details, pendingDetail{"class", pyReprList(classes)})
	}
	return []doctree.Node{pendingNode("docutils.transforms.misc.ClassAttribute", details)}
}

// runSectnumDirective ports directives.parts.SectNum. Only the options
// actually given appear in the details; depth and start are integers, so
// they print unquoted.
func runSectnumDirective(args string, body []string) []doctree.Node {
	combined := append([]string{args}, body...)
	_, options, _ := parseDirectiveBlock(combined, false)
	var details []pendingDetail
	for key, raw := range options {
		v := strings.TrimSpace(raw)
		switch strings.ToLower(key) {
		case "depth", "start":
			if _, err := strconv.Atoi(v); err == nil {
				details = append(details, pendingDetail{strings.ToLower(key), v})
			}
		case "prefix", "suffix":
			details = append(details, pendingDetail{strings.ToLower(key), pyRepr(v)})
		}
	}
	return []doctree.Node{pendingNode("docutils.transforms.parts.SectNum", details)}
}

// runTargetNotesDirective ports directives.references.TargetNotes, whose
// only option is :class:.
func runTargetNotesDirective(args string, body []string, blockText string, lineno int) []doctree.Node {
	combined := append([]string{args}, body...)
	_, options, _ := parseDirectiveBlock(combined, false)
	// TargetNotes declares exactly one option, :class:. docutils reports
	// anything else, and an option given with no value, as an
	// "Error in ... directive" naming the specific failure -- the general
	// per-directive option validation this package otherwise does not do,
	// ported here because target-notes' whole option_spec is one entry.
	for k, v := range options {
		switch {
		case !strings.EqualFold(k, "class"):
			return []doctree.Node{sectionMessage("3", "ERROR",
				`Error in "target-notes" directive:`+"\n"+`unknown option: "`+k+`".`, lineno, blockText)}
		case strings.TrimSpace(v) == "":
			return []doctree.Node{sectionMessage("3", "ERROR",
				`Error in "target-notes" directive:`+"\n"+
					`invalid option value: (option: "`+strings.ToLower(k)+`"; value: None)`+"\n"+
					`argument required but none supplied.`, lineno, blockText)}
		}
	}
	var details []pendingDetail
	if v, ok := options["class"]; ok {
		var classes []string
		for _, f := range strings.Fields(v) {
			classes = append(classes, makeID(f))
		}
		if len(classes) > 0 {
			details = append(details, pendingDetail{"class", pyReprList(classes)})
		}
	}
	return []doctree.Node{pendingNode("docutils.transforms.references.TargetNotes", details)}
}
