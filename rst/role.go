package rst

import "strings"

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
// where "gadget" is itself ".. role::"-defined) is not resolved — base is
// stored as given and only ever compared against roleTags/"raw" directly,
// so a chained base falls through to the same generic_custom_role
// behavior real docutils itself gives an UNRESOLVABLE base (a role
// registered against a name that isn't a real built-in), just without
// the error real docutils raises for THAT specific case. A vanishingly
// rare compound of an already-uncommon construct.

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
// roleDef. Body is the directive's own gathered, dedented body lines
// (gatherExplicitBody's output), scanned for ":key: value" lines the same
// way a field list's own marker line is recognized; "format" (raw-based
// roles), "language" and "class" (code-based roles) are the only three
// this parser gives real meaning to.
func (p *parser) registerRole(args string, body []string) {
	name, base, ok := parseRoleArgs(args)
	if !ok {
		return
	}
	def := roleDef{base: strings.ToLower(base)}
	classGiven := false
	for _, line := range body {
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
			def.classes = classOption(val)
			classGiven = true
		}
	}
	// docutils.parsers.rst.directives.misc.Role.run (read directly): "if
	// 'class' not in options: options['class'] = directives.class_option
	// (new_role_name)" — every custom role gets its own name as its
	// default class list unless the definition gives one explicitly.
	if !classGiven {
		def.classes = classOption(name)
	}
	if p.roles == nil {
		p.roles = map[string]roleDef{}
	}
	p.roles[strings.ToLower(name)] = def
}

// classOption mirrors docutils.parsers.rst.directives.class_option (via
// nodes.make_id, both read directly): splits on whitespace, then
// lowercases each token and replaces any character that isn't a letter,
// digit, or hyphen with a hyphen. make_id's fuller Unicode-normalization
// behavior (NFKD decomposition, leading-digit handling) isn't replicated —
// every role/class name reaching this parser in practice is already a
// plain ASCII identifier.
func classOption(s string) []string {
	var out []string
	for _, tok := range strings.Fields(s) {
		var b strings.Builder
		for _, r := range strings.ToLower(tok) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
				b.WriteRune(r)
			default:
				b.WriteByte('-')
			}
		}
		if b.Len() > 0 {
			out = append(out, b.String())
		}
	}
	return out
}
