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
// (gatherExplicitBody's output), scanned for ":format: value" the same
// way a field list's own marker line is recognized.
func (p *parser) registerRole(args string, body []string) {
	name, base, ok := parseRoleArgs(args)
	if !ok {
		return
	}
	def := roleDef{base: strings.ToLower(base)}
	for _, line := range body {
		trimmed := strings.TrimLeft(line, " ")
		if key, col, ok := matchFieldMarker(trimmed); ok && strings.EqualFold(key, "format") {
			def.format = strings.ToLower(strings.TrimSpace(trimmed[col:]))
		}
	}
	if p.roles == nil {
		p.roles = map[string]roleDef{}
	}
	p.roles[strings.ToLower(name)] = def
}
