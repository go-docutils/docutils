package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.admonitions (read
// directly): the nine GENERIC admonitions (attention/caution/danger/
// error/hint/important/note/tip/warning — each just wraps its own
// content in a like-named node, case-insensitive directive name) plus
// the one non-generic "admonition" directive (a REQUIRED title argument,
// producing a <title> child and defaulting its own class to
// "admonition-<slug>" unless :class: overrides it).

var admonitionTags = map[string]string{
	"attention": doctree.TagAttention,
	"caution":   doctree.TagCaution,
	"danger":    doctree.TagDanger,
	"error":     doctree.TagErrorAdmonition,
	"hint":      doctree.TagHint,
	"important": doctree.TagImportant,
	"note":      doctree.TagNote,
	"tip":       doctree.TagTip,
	"warning":   doctree.TagWarningAdmonition,
}

// runAdmonitionDirective implements BaseAdmonition.run for one of the
// nine generic admonitions — tag is already resolved by the caller
// (case-insensitive directive-name lookup in admonitionTags).
func (p *parser) runAdmonitionDirective(tag string, lines []string, i, next int, args string, body []string) []doctree.Node {
	return p.runAdmonitionOrGeneric(tag, "", lines, i, next, args, body)
}

// runGenericAdmonitionDirective implements Admonition.run (the
// ".. admonition:: TITLE" directive specifically): a REQUIRED argument
// becomes the admonition's own <title>, inline-parsed like any other
// directive title, and — unless an explicit :class: option overrides it
// — the admonition's own default class becomes "admonition-<slug of the
// title>" (nodes.make_id, read directly).
func (p *parser) runGenericAdmonitionDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	return p.runAdmonitionOrGeneric(doctree.TagAdmonition, "REQUIRED", lines, i, next, args, body)
}

// runAdmonitionOrGeneric is the shared implementation: requireArg is ""
// for the nine generic admonitions (no argument at all — same-line text
// after "::" is the directive's own FIRST content line, not an
// argument) or "REQUIRED" for ".. admonition::" (exactly one argument,
// becoming a <title>).
func (p *parser) runAdmonitionOrGeneric(tag, requireArg string, lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	directiveName := lines[i][3:]
	if idx := strings.Index(directiveName, "::"); idx >= 0 {
		directiveName = strings.TrimSpace(directiveName[:idx])
	}

	// gatherExplicitBody's own body already dropped the blank line(s)
	// separating a same-line argument from the indented block below it
	// (it only cares about finding the dedent, not preserving them) —
	// but parse_directive_block's own algorithm (states.py, read
	// directly) needs that separator PRESERVED to find the argument/
	// content boundary at all, so it's reinserted here, exactly as many
	// as were actually there.
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
	argument, options, content := parseDirectiveBlock(combined, requireArg != "")
	if requireArg != "" && argument == "" {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Error in \""+directiveName+"\" directive:\n1 argument(s) required, 0 supplied.", lineno, blockText)}
	}
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "`+directiveName+`" directive; none found.`, lineno, blockText)}
	}

	el := doctree.NewElement(tag)
	if v, ok := options["class"]; ok {
		el.SetAttr("class", strings.Join(classOption(v), " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", makeID(name))
	}
	var titleMsgs []*doctree.Element
	if requireArg != "" {
		var title *doctree.Element
		title, titleMsgs = p.parseTableTitle(argument, lineno)
		el.Append(title)
		if _, hasClass := options["class"]; !hasClass {
			el.SetAttr("class", "admonition-"+makeID(argument))
		}
	}
	p.parseBlockLines(content, el)

	out := []doctree.Node{el}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	return out
}

// parseDirectiveBlock mirrors Body.parse_directive_block +
// parse_directive_options (states.py, read directly) — a more general
// option/content split than tabledirective.go's own parseDirectiveOptions
// (which only recognizes a CONTIGUOUS run of option lines at the very
// start): combined is the directive's own same-line argument text
// (line 0, possibly "") followed by the body's own dedented lines, WITH
// any separating blank line(s) preserved (see the caller, which
// reconstructs them — gatherExplicitBody's own body has already
// dropped them). The block splits at its own FIRST blank line into an
// "arg block" and "content"; the arg block is then scanned top-to-bottom
// for the FIRST field-marker-shaped line, and everything from there
// onward is consumed as :class:/:name: options (via gatherListItemLines,
// so a multi-line option value's own continuation works the same way a
// field list's body already does) — everything BEFORE that point stays
// as the real argument text when hasArgument, or folds back into
// content when the directive takes no argument at all (matching every
// admonition here except ".. admonition::" itself).
func parseDirectiveBlock(combined []string, hasArgument bool) (argument string, options map[string]string, content []string) {
	for len(combined) > 0 && isBlankStr(combined[len(combined)-1]) {
		combined = combined[:len(combined)-1]
	}
	// A single leading blank line — the directive's own line had nothing
	// after "::" — is trimmed BEFORE scanning for the argument/content
	// boundary (Body.parse_directive_block's own "if indented and not
	// indented[0].strip(): indented.trim_start()", read directly): without
	// this, an argument that starts wholly on the FOLLOWING line (no
	// same-line text at all, e.g. ".. |x| image::\n   uri.png") was
	// mistaken for a genuinely empty argument block, since combined[0]=""
	// looks identical to a real blank-line separator otherwise. Latent
	// until image's required-argument directive exposed it — no existing
	// admonition/topic/table corpus case has an omittable argument that
	// starts on a later line at all.
	if len(combined) > 0 && isBlankStr(combined[0]) {
		combined = combined[1:]
	}

	blankAt := -1
	for idx, l := range combined {
		if isBlankStr(l) {
			blankAt = idx
			break
		}
	}
	var argBlock, rest []string
	if blankAt >= 0 {
		argBlock = combined[:blankAt]
		rest = combined[blankAt+1:]
	} else {
		argBlock = combined
	}

	optStart := -1
	for idx, l := range argBlock {
		if _, _, ok := matchFieldMarker(l); ok {
			optStart = idx
			break
		}
	}
	var optBlock []string
	if optStart >= 0 {
		optBlock = argBlock[optStart:]
		argBlock = argBlock[:optStart]
	}
	options = parseFieldListBlock(optBlock)

	content = rest
	if len(argBlock) > 0 && !hasArgument {
		content = append(append([]string{}, argBlock...), rest...)
		argBlock = nil
	}
	for len(content) > 0 && isBlankStr(content[0]) {
		content = content[1:]
	}
	argument = strings.TrimSpace(strings.Join(argBlock, " "))
	return argument, options, content
}

// parseFieldListBlock parses lines as a plain ":key: value" run —
// reusing gatherListItemLines' own multi-line-continuation handling
// (already used by real field lists, fieldlist.go) so an option value
// spanning several lines joins the same way ("* value1\n* value2", not
// space-run-together).
func parseFieldListBlock(lines []string) map[string]string {
	options := map[string]string{}
	i := 0
	for i < len(lines) {
		key, col, ok := matchFieldMarker(lines[i])
		if !ok {
			i++
			continue
		}
		first := ""
		if len(lines[i]) > col {
			first = lines[i][col:]
		}
		valueLines, next := gatherListItemLines(lines, i, col, first)
		options[strings.ToLower(key)] = strings.TrimSpace(strings.Join(valueLines, "\n"))
		i = next
		for i < len(lines) && isBlankStr(lines[i]) {
			i++
		}
	}
	return options
}
