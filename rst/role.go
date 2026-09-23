package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// The ".. role::" directive (roles.py's Role.run, read directly):
// ".. role:: NAME(BASE)" or bare ".. role:: NAME" registers a custom
// interpreted-text role, dispatched through roleElement from then on for
// the rest of the document. Options (":format: html", the only one this
// parser gives real meaning to, for a "raw"-based role) are simple
// ":key: value" body lines — real docutils' own option_spec machinery is
// far more general (arbitrary per-directive option types, validation),
// not replicated here; a role definition's own body only ever really
// carries "format" for the cases this project acts on differently.
//
// SCOPE: a role based on another CUSTOM role (chained, "widget(gadget)"
// where "gadget" is itself ".. role::"-defined earlier in the same
// document) resolves fine — knownRoleNames/p.roles below cover it — but
// the base is stored as given and only ever compared against
// roleTags/"raw" directly at USE time (roleElement), never actually
// walked back to gadget's own base; a role chain more than one level
// deep still just falls through to the same generic_custom_role shape
// real docutils gives it, not the base's own real behavior. A
// vanishingly rare compound of an already-uncommon construct.

// knownRoleNames mirrors docutils.parsers.rst.languages.en.roles' key set
// (read directly) — every alias name real docutils recognizes as a valid
// role base, whether or not this parser gives it any special dispatch of
// its own (roleElement only special-cases a handful; everything else here
// still counts as "known" for base-role VALIDATION purposes, matching
// what a real ".. role:: x(index)" or ".. role:: x(url)" would accept
// without error even though this parser then treats it as an ordinary
// generic_custom_role at use time — a real, but narrower and previously
// unnoticed, simplification than the "just never errors" one this file's
// own SCOPE note already flagged, not worth resolving further since no
// corpus fixture exercises any of these role names being USED afterward).
var knownRoleNames = map[string]bool{
	"ab": true, "abbreviation": true, "ac": true, "acronym": true,
	"anonymous-reference": true, "citation-reference": true, "code": true,
	"emphasis": true, "footnote-reference": true, "i": true, "index": true,
	"literal": true, "math": true, "named-reference": true, "pep": true,
	"pep-reference": true, "raw": true, "rfc": true, "rfc-reference": true,
	"strong": true, "sub": true, "subscript": true,
	"substitution-reference": true, "sup": true, "superscript": true,
	"t": true, "target": true, "title": true, "title-reference": true,
	"uri": true, "uri-reference": true, "url": true,
}

// isKnownRoleName reports whether name is either one of the built-in
// roles (knownRoleNames) or a PREVIOUSLY ".. role::"-defined custom role
// in this document — the same two-stage check registerRole's own base-
// role validation already needed (roles.role's local-registry-then-
// language-module lookup, read directly), reused by ".. default-role::"
// below for the identical validation.
func (p *parser) isKnownRoleName(name string) bool {
	lower := strings.ToLower(name)
	_, chained := p.roles[lower]
	return knownRoleNames[lower] || chained
}

// unknownRoleDiagnostics is the INFO+ERROR pair real docutils' roles.role
// produces for a name neither this project nor real docutils has ever
// heard of (read directly) — shared between ".. role::"'s own base-role
// validation and ".. default-role::"'s argument validation, both of
// which reach the exact same lookup.
func unknownRoleDiagnostics(name string, lineno int, blockText string) []doctree.Node {
	return []doctree.Node{
		sectionMessage("1", "INFO",
			"No role entry for \""+name+"\" in module \"docutils.parsers.rst.languages.en\".\nTrying \""+name+"\" as canonical role name.",
			lineno, ""),
		sectionMessage("3", "ERROR",
			`Unknown interpreted text role "`+name+`".`, lineno, blockText),
	}
}

// parseRoleArgs recognizes "NAME(BASE)" or bare "NAME" — the ".. role::"
// directive's own argument, docutils' argument_pattern
// (`simplename\s*(\(\s*simplename\s*\)\s*)?`).
func parseRoleArgs(args string) (name, base string, ok bool) {
	args = strings.TrimSpace(args)
	if args == "" {
		return "", "", false
	}
	if open := strings.IndexByte(args, '('); open >= 0 {
		if !strings.HasSuffix(args, ")") {
			return "", "", false
		}
		name = strings.TrimSpace(args[:open])
		base = strings.TrimSpace(args[open+1 : len(args)-1])
		if name == "" || base == "" {
			return "", "", false
		}
		return name, base, true
	}
	return args, "", true
}

// registerRole records one ".. role::" definition — see parser.roles and
// roleDef — or, ported this round (Role.run, read directly), returns the
// diagnostic real docutils raises instead, for each of its five distinct
// ERROR/INFO paths: no content at all, content that doesn't match
// argument_pattern, an unresolvable base role (an INFO plus an ERROR, the
// same pair roles.role's own two-stage lookup always produces for a name
// neither this project nor real docutils has ever heard of), and an
// invalid :class:/argument-derived class name (Directive.error wording
// differs between the two — see classOptionStrict's callers below).
//
// args/body are combined the same way runContainerDirective/
// runAdmonitionOrGeneric combine theirs, but — unlike those —
// deliberately NOT run through parseDirectiveBlock's own field-marker
// option-splitting: real docutils' run_directive only does that split at
// all when the directive declares required/optional arguments or an
// option_spec (parse_directive_block, read directly), and Role declares
// NONE of those (has_content=True is its only directive-level
// declaration) — so its own content is the ENTIRE dedented block,
// edge-trimmed but internally untouched, same-line text and all,
// content[0] being the "NAME(BASE)" text and content[1:] the raw
// ":key: value" option lines this function scans for itself below.
// Calling parseDirectiveBlock here instead would silently swallow a
// same-line-adjacent option line (no blank separator) into ITS OWN
// "options" return value, discarded by a caller that never asked for
// it — caught by the existing TestRoleDirective suite immediately, not
// the corpus (the raw/code-based-role cases there have no blank line
// between the argument and their first option).
func (p *parser) registerRole(lines []string, i, next, lineBase int, args string, body []string) []doctree.Node {
	lineno := msgLine(i, lineBase)
	blockText := strings.Join(lines[i:next], "\n")

	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	content := make([]string, 0, 1+blanks+len(body))
	content = append(content, args)
	for k := 0; k < blanks; k++ {
		content = append(content, "")
	}
	content = append(content, body...)
	for len(content) > 0 && isBlankStr(content[0]) {
		content = content[1:]
	}
	for len(content) > 0 && isBlankStr(content[len(content)-1]) {
		content = content[:len(content)-1]
	}

	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`"role" directive requires arguments on the first line.`, lineno, blockText)}
	}

	argsLine := content[0]
	name, base, ok := parseRoleArgs(argsLine)
	if !ok {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`"role" directive arguments not valid role names: "`+strings.TrimSpace(argsLine)+`".`, lineno, blockText)}
	}

	if base != "" && !p.isKnownRoleName(base) {
		return unknownRoleDiagnostics(base, lineno, blockText)
	}

	def := roleDef{base: strings.ToLower(base)}
	classGiven := false
	for _, line := range content[1:] {
		trimmed := strings.TrimLeft(line, " ")
		key, col, ok := matchFieldMarker(trimmed)
		if !ok {
			continue
		}
		val := strings.TrimSpace(trimmed[col:])
		switch {
		case strings.EqualFold(key, "format"):
			def.format = strings.ToLower(val)
		case strings.EqualFold(key, "language"):
			def.language = val
			def.hasLanguage = true
		case strings.EqualFold(key, "class"):
			classes, failed, ok := classOptionStrict(val)
			if !ok {
				return []doctree.Node{sectionMessage("3", "ERROR",
					"Error in \"role\" directive:\ninvalid option value: (option: \"class\"; value: '"+val+"')\ncannot make \""+failed+"\" into a class name.",
					lineno, blockText)}
			}
			def.classes = classes
			classGiven = true
		}
	}
	// docutils.parsers.rst.directives.misc.Role.run (read directly): "if
	// 'class' not in options: options['class'] = directives.class_option
	// (new_role_name)" — every custom role gets its own name as its
	// default class list unless the definition gives one explicitly.
	if !classGiven {
		classes, failed, ok := classOptionStrict(name)
		if !ok {
			return []doctree.Node{sectionMessage("3", "ERROR",
				"Invalid argument for \"role\" directive:\ncannot make \""+failed+"\" into a class name.",
				lineno, blockText)}
		}
		def.classes = classes
	}
	if p.roles == nil {
		p.roles = map[string]roleDef{}
	}
	p.roles[strings.ToLower(name)] = def
	return nil
}

// classOption and classOptionStrict are
// docutils.parsers.rst.directives.class_option (read directly), which is
// not a rule of its own at all:
//
//	names = argument.split()
//	for name in names:
//	    class_name = nodes.make_id(name)
//	    if not class_name:
//	        raise ValueError('cannot make "%s" into a class name' % name)
//
// One make_id per whitespace-separated token, and a ValueError when a
// token yields nothing. Both functions used to REIMPLEMENT make_id
// instead of calling it, each approximating a different subset, each
// with a doc comment naming what it left out and excusing it with "no
// corpus case exercises it". A corpus case did:
// ".. table::\n   :class: longtable, borderless" gave class
// "longtable-" here, because the comma became a hyphen and nothing
// stripped it from the end — make_id's own `-+$` rule, sitting unused
// in the other copy.
//
// The two now differ only in what they do with a token that makes no
// identifier at all: strict reports it (the "role" directive's own
// error path, corpus-verified in test_role.py[7]/[8]), lenient drops
// it. Dropping is closer to docutils than what lenient did before —
// ":class: 1" kept the class "1", which make_id cannot produce.
func classOption(s string) []string {
	var out []string
	for _, tok := range strings.Fields(s) {
		if id := makeID(tok); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func classOptionStrict(s string) (classes []string, failed string, ok bool) {
	for _, tok := range strings.Fields(s) {
		id := makeID(tok)
		if id == "" {
			return nil, tok, false
		}
		classes = append(classes, id)
	}
	return classes, "", true
}

// runDefaultRoleDirective implements DefaultRole.run (misc.py, read
// directly): ".. default-role:: NAME" changes what BARE interpreted text
// (“ `text` “, no explicit role prefix, no trailing hyperlink
// underscore) resolves to for the rest of the document — read by
// referenceOrPhrase via p.defaultRole. A bare ".. default-role::" (no
// argument at all) RESETS it back to real docutils' own standard
// default (title-reference) rather than leaving whatever was set before.
// The argument is expected on the SAME line only (every corpus fixture
// has it there; optional_arguments=1/final_argument_whitespace=False
// means it's a single token anyway, so no multi-line body-gathering is
// needed the way admonitions/topic/sidebar's own REQUIRED arguments do).
func (p *parser) runDefaultRoleDirective(lines []string, i, next, lineBase int, args string) []doctree.Node {
	roleName := strings.TrimSpace(args)
	if roleName == "" {
		p.defaultRole = ""
		return nil
	}
	if !p.isKnownRoleName(roleName) {
		lineno := msgLine(i, lineBase)
		blockText := strings.Join(lines[i:next], "\n")
		return unknownRoleDiagnostics(roleName, lineno, blockText)
	}
	p.defaultRole = strings.ToLower(roleName)
	return nil
}
