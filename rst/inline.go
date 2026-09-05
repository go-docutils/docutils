package rst

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/go-docutils/docutils/doctree"
)

// inliner parses reStructuredText inline markup within a single block of
// text (a paragraph, title, etc.), modeled on docutils.parsers.rst.states.
// Inliner.parse.
//
// SCOPE (v1): strong (**x**), emphasis (*x*), literal (two backticks
// around x), interpreted text with a role — prefix (:role:`x`) or
// suffix (`x`:role:) — for the small fixed set of docutils' built-in
// GENERIC roles (emphasis, strong, literal, subscript/sub,
// superscript/sup, title-reference/title/t, abbreviation/ab,
// acronym/ac, math; see roleTags), plus docutils' three other
// always-registered, non-generic roles — code (real class-list/
// highlight-language logic, no Pygments equivalent — see
// codeRoleClasses/codeRoleElement), pep-reference/pep and rfc-reference/
// rfc (numeric validation + hyperlink generation — see pepRole/rfcRole)
// — plus a bare `x` with no role at all (docutils' DEFAULT role,
// title_reference), plus (docutils/rst v0.16.0+, see role.go) a
// ".. role::"-registered custom role, aliasing a roleTags entry, "code"
// (with its own :language:/:class: options), or — its base is "raw" —
// this parser's one inline raw construct (Options.RawEnabled gates it
// exactly like the block-level "raw" directive does). Any role name
// docutils itself would NOT recognize still falls back to a generic
// <inline role="name"> rather than docutils' own error-and-rewrite-to-
// `problematic` behavior for a role it truly cannot resolve, deliberately
// not replicated: unknown-role VALIDATION (as opposed to a role this
// parser gives real behavior to, like the ones above) exists to serve
// `raw` and the roles above correctly, not to police every role name a
// document might use for an extension (Sphinx and friends) this parser
// has never heard of and isn't trying to validate against. Named/
// anonymous reference both bare (x_, x__) and backtick-quoted (`x`_,
// `x`__) including an embedded URI or indirect name-alias target (`text
// <https://example.com>`_, `text <alias_>`_ — a NAMED one also emits the
// implicit <target> sibling real docutils creates alongside it, so
// another reference to the same display text elsewhere can resolve to
// it too; an ANONYMOUS one never does, resolving directly off its own
// refuri/refname instead of joining document-order anonymous-target
// matching — Inliner.phrase_ref, states.py, read directly), substitution
// reference (|x|), footnote/citation reference ([1]_ / [#]_ / [#name]_ /
// [*]_ / [name]_), and backslash escapes; and standalone URI
// (scheme://...) and email (user@host) recognition — no backtick
// quoting or trailing `_` needed at all, matching docutils' own
// implicit_inline fallback (though only a "scheme://" URI is
// recognized; a bare "scheme:path" with no "//" — mailto:, news:, and
// friends outside backtick-quoted/embedded-link contexts — is a real,
// separate, not-yet-ported gap, test_inline_markup.py's own
// standalone_hyperlink group). Standalone PEP/RFC recognition (pep-123,
// RFC 123 as bare TEXT, no :PEP:/:RFC: role markup at all) is
// deliberately NOT implemented, and not merely deferred: docutils' own
// pep_references/rfc_references settings default to None (off) —
// verified against Parser().parse() on "pep-8, PEP 257, RFC 2822" with
// default settings, which produced plain text, no references at all.
// Implementing this unconditionally would make this parser MORE
// aggressive than docutils' own default, a real divergence rather than
// a gap filled. Content between markers is treated as plain text and is
// not re-parsed for further inline markup, matching docutils' actual
// (not merely documented)
// behavior: nested inline markup does not match.
//
// parseInline also returns every <system_message> a start-string-without-
// end-string failure produced while parsing THIS text (see
// markupProblematic) — real docutils' RSTState.inline_text returns the
// identical pair (textnodes, messages), and every one of its own six call
// sites (Body.paragraph, new_subsection [title], parse_attribution,
// Body.field, Body.line_block_line, Text.term — states.py, read directly)
// attaches those messages to the tree AT THE POINT OF ORIGIN — as a
// sibling of the paragraph/title/attribution/etc., never collected into
// one document-trailing section. That trailing "Docutils System Messages"
// section is docutils' OWN transforms.universal.Messages, and it wraps
// only "loose" messages (msg.parent is None) — a message already inserted
// as a sibling at its origin has a parent and is explicitly excluded
// (`loose_messages = [msg for msg in messages if not msg.parent]`, read
// directly). So parseInline's caller is responsible for placing these
// messages itself, matching whichever of the six patterns above applies
// to it; only a genuinely parentless message (explicit.go's dangling-
// reference/anonymous-mismatch ones, which real docutils' own
// DanglingReferencesVisitor generates via document.reporter.error with no
// tree insertion at all) still belongs in resolveTargets' trailing
// systemMessagesSection.
//
// lineno is the 1-indexed absolute source line text starts on, or 0 if
// unknown at this call site — see the parser.currentLine field doc
// comment for exactly which call sites can supply a real value today.
func (p *parser) parseInline(text string, lineno int) ([]doctree.Node, []*doctree.Element) {
	savedMessages := p.messages
	savedLine := p.currentLine
	p.messages = nil
	p.currentLine = lineno
	defer func() { p.messages = savedMessages; p.currentLine = savedLine }()

	runes := escapeBackslashes(text)
	var out []doctree.Node
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, &doctree.Text{Data: buf.String()})
			buf.Reset()
		}
	}

	i := 0
	n := len(runes)
	for i < n {
		if node, consumed, ok := p.tryFootnoteRef(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := tryBareReference(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := p.tryInlineTarget(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if nodes, consumed, ok := p.tryInterpretedOrPhraseRef(runes, i); ok {
			flush()
			out = append(out, nodes...)
			i += consumed
			continue
		}
		if node, consumed, ok := p.tryMarker(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if node, consumed, ok := tryStandaloneURI(runes, i); ok {
			flush()
			out = append(out, node)
			i += consumed
			continue
		}
		if !isDroppedEscape(runes[i]) {
			buf.WriteRune(unescapeRune(runes[i]))
		}
		i++
	}
	flush()
	return out, p.messages
}

// escapedBase marks a backslash-escaped character: shifted into the
// Supplementary Private Use Area-A so it can never collide with a real
// marker rune ('*', '`', '|', ...) during matching, mirroring what
// docutils.utils.escape2null does with a NUL-byte prefix. Escaped runes
// are restored to their literal form wherever they end up as output text
// (see unescapeRune / unescapeRunes) — the point is only that they must
// never be RECOGNIZED as markup delimiters.
const escapedBase = 0xF0000

func escapeRune(r rune) rune { return escapedBase + r }

func isEscapedRune(r rune) bool { return r >= escapedBase && r < escapedBase+0x10000 }

func unescapeRune(r rune) rune {
	if isEscapedRune(r) {
		return r - escapedBase
	}
	return r
}

// isDroppedEscape reports whether r is an escaped space or newline —
// docutils.nodes.unescape's own documented rule ("Backslash-escaped spaces
// are also removed"): a backslash-escaped space/newline is a pure
// end-boundary marker (see validEndBoundaryAfterOptionalUnderscores'/
// findClose's isEscapedRune check — the SAME escaped rune is what makes
// "*emphasis*\ (more text)" a valid closing boundary in the first place),
// dropped entirely from the final text rather than rendered as a literal
// space/newline; any OTHER escaped character keeps its literal form
// (unescapeRune's ordinary behavior). Verified against the foreign judge:
// found only once this project's own boundary-rule fix started letting
// this exact construct close as markup at all — previously it always fell
// through as plain text, where the (still-present but never-exercised)
// bug had no observable effect.
func isDroppedEscape(r rune) bool {
	return isEscapedRune(r) && (unescapeRune(r) == ' ' || unescapeRune(r) == '\n')
}

func unescapeRunes(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if isDroppedEscape(r) {
			continue
		}
		b.WriteRune(unescapeRune(r))
	}
	return b.String()
}

// restoreEscapes reverses escapeBackslashes exactly — an escaped rune
// becomes "\" plus its literal character again, instead of unescapeRunes'
// bare-character-only reduction. For content this parser never treats as
// reST at all (an inline raw role's content — see roleElement), the
// backslash itself is part of the real payload, not reST escape syntax to
// strip.
func restoreEscapes(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if isEscapedRune(r) {
			b.WriteByte('\\')
			b.WriteRune(unescapeRune(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeBackslashes returns text as runes with every backslash-escaped
// character (`\X`) replaced by its escaped form, so later marker matching
// can simply refuse to treat an escaped rune as a delimiter.
func escapeBackslashes(s string) []rune {
	src := []rune(s)
	out := make([]rune, 0, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) {
			out = append(out, escapeRune(src[i+1]))
			i++
			continue
		}
		out = append(out, src[i])
	}
	return out
}

// marker describes one inline-markup delimiter pair. Backtick-quoted
// text (interpreted text / phrase references) is NOT in this table —
// its content needs role/underscore lookahead beyond "does the closing
// delimiter match", so it's handled separately by
// tryInterpretedOrPhraseRef.
type marker struct {
	open string
	tag  string
}

var markers = []marker{
	{"**", doctree.TagStrong},
	{"``", doctree.TagLiteral},
	{"*", doctree.TagEmphasis},
	{"|", doctree.TagSubstitutionRef},
}

// tryMarker attempts to match an inline-markup construct starting at
// runes[i]. Returns the produced node, the number of runes consumed, and
// whether a match was found. Once a marker's open string AND start
// boundary both match, real docutils commits to that one construct — its
// compiled regex dispatches once, with no "try the next alternative"
// fallback — so a close-string failure past that point ends the loop
// with a <problematic> (markupProblematic) rather than falling through
// to try unrelated markers, INCLUDING substitution_reference ("|"):
// verified against the foreign judge ("|sub|ref", no closing "|" with a
// valid boundary anywhere) that real docutils DOES warn there too — an
// earlier version of this parser special-cased substitution_reference
// out of this rule based on more limited testing that happened not to
// exercise this exact shape; that divergence was wrong, not a real
// leniency choice, and is corrected here.
//
// The "**"-vs-"*" ambiguity gets its own explicit precedence check,
// separate from markers' own list order: real docutils' compiled
// regex alternation lists `\*\*` before `\*(?!\*)` — the negative
// lookahead means a lone "*" NEVER even attempts to match when it's
// actually the start of a "**" run, regardless of whether "**" itself
// goes on to satisfy its own start boundary. Iterating markers in order
// and simply `continue`-ing past a boundary failure gets this wrong: at
// "(**)", "**"'s own boundary correctly rejects (quoted_start — an
// empty parenthesized pair), but the loop would otherwise fall through
// and let "*" (single) match at the SAME position, silently reinterpreting
// an invalid two-character attempt as a valid shorter one — something
// real docutils' regex, having committed to "**" as the only candidate
// for these two characters, never does.
func (p *parser) tryMarker(runes []rune, i int) (doctree.Node, int, bool) {
	for _, m := range markers {
		ol := len([]rune(m.open))
		if !hasPrefixAt(runes, i, m.open) {
			continue
		}
		if m.tag == doctree.TagEmphasis && hasPrefixAt(runes, i, "**") {
			return nil, 0, false
		}
		// docutils' substitution start-string is `\|(?!\|)`: a "|"
		// followed by another "|" is not a start-string at all. Without
		// this, "first | then || and finally |||" opened a substitution
		// reference at the first "|" of the "||" pair and swallowed the
		// rest of the line, where real docutils leaves every pipe as
		// plain text.
		if m.tag == doctree.TagSubstitutionRef && hasPrefixAt(runes, i, "||") {
			return nil, 0, false
		}
		if !validStartBoundary(runes, i, ol) {
			continue
		}
		// find matching close, honoring end-string-suffix boundary rule.
		// Substitution reference is special-cased the same way backquote
		// is (findCloseBackquote): real docutils' own substitution_ref
		// pattern is `(\|_{0,2})` — 0, 1, or 2 trailing underscores are
		// PART of the same close match end_string_suffix applies after,
		// so "|sub|_ text" is valid (boundary checked after the "_",
		// against the space, not immediately after "|" against the "_"
		// itself) — verified against the foreign judge, the same class of
		// bug the backquote case hit first.
		var closeAt, closeLen int
		var ok bool
		switch m.tag {
		case doctree.TagSubstitutionRef:
			closeAt, closeLen, ok = findCloseSubstitution(runes, i+ol)
		case doctree.TagLiteral:
			closeAt, closeLen, ok = findCloseLiteral(runes, i+ol)
		default:
			closeAt, closeLen, ok = findClose(runes, i+ol, m.open)
		}
		if !ok {
			return p.markupProblematic(m.tag, m.open), ol, true
		}
		// literal's content is backslash-RESTORED, not stripped — real
		// docutils' Inliner.literal calls inline_obj with
		// restore_backslashes=True, the ONLY marker in this table that
		// does (Inliner.emphasis/strong don't pass it at all, defaulting
		// False); nodes.unescape(text, restore_backslashes=True) replaces
		// the escape marker with a literal backslash instead of dropping
		// it (states.py/nodes.py, both read directly — confirmed against
		// the foreign judge: "``\literal``" keeps its backslash, visible,
		// in the rendered <literal> content, unlike every other marker).
		var content string
		if m.tag == doctree.TagLiteral {
			content = restoreEscapes(runes[i+ol : closeAt])
			// When the close was found ON an escaped backquote (see
			// findCloseLiteral), the backslash that escaped it still
			// belongs to the CONTENT: docutils' escape2null leaves the
			// marker byte inside the text it slices, and unescape(...,
			// restore_backslashes=True) turns it back into a backslash.
			// "``literal\``" is <literal>literal\</literal>.
			if isEscapedRune(runes[closeAt]) {
				content += `\`
			}
		} else {
			content = unescapeRunes(runes[i+ol : closeAt])
		}
		if content == "" {
			continue // start-string immediately followed by end-string: no match
		}
		total := (closeAt + closeLen) - i
		el := doctree.NewElement(m.tag, &doctree.Text{Data: content})
		if m.tag == doctree.TagSubstitutionRef {
			el.SetAttr("refname", normalizeWhitespace(content))
			// A substitution reference used AS a hyperlink (|x|_ / |x|__):
			// docutils wraps it in a <reference> pointing at a target with
			// the same name (or, doubled, an anonymous one) — the
			// substitution's own text becomes the reference's visible
			// content, same as any other reference, but no "name" attribute
			// (there is no separate display text to remember; unlike
			// `text`_, the substitution IS the display).
			if after := i + total; after < len(runes) && runes[after] == '_' {
				ref := doctree.NewElement(doctree.TagReference, el)
				if after+1 < len(runes) && runes[after+1] == '_' {
					ref.SetAttr("anonymous", "true")
					return ref, total + 2, true
				}
				ref.SetAttr("refname", normalizeName(content))
				return ref, total + 1, true
			}
		}
		return el, total, true
	}
	return nil, 0, false
}

// markupProblematic builds the <problematic>/<system_message> pair for one
// inline-markup start-string with no matching end-string — real docutils'
// Inliner.inline_obj, the SAME cross-linked id/refid/backref shape
// resolveTargets' own problematicReference already uses for a dangling
// reference (explicit.go), just triggered from a different, genuinely
// separate code path: this one fires DURING inline parsing itself, not as
// a whole-document post-pass, since docutils' own equivalent (Inliner) is
// itself part of the parser, not a transform. kind names the failed
// construct (a doctree tag value, already lowercase and matching
// docutils' node class name verbatim: "emphasis", "strong", "literal") for
// the "Inline KIND start-string without end-string." message text; marker
// is the literal open-string that becomes the <problematic>'s own visible
// content (just the marker itself, e.g. "*" or "**" — not the rest of the
// line, matching real docutils exactly, not this project's own richer
// verbatim-text convention used elsewhere).
// markupProblematic's <system_message> carries level="2" type="WARNING"
// unconditionally — real docutils' inline_obj builds it via
// self.reporter.warning(...), read directly, and "start-string without
// end-string" is the ONLY message text this function ever produces, so
// there is no other (level, type) pair to consider. line is set only when
// p.currentLine is known (nonzero) — see parseInline's own doc comment on
// why this project doesn't yet track it for every calling context.
func (p *parser) markupProblematic(kind, marker string) *doctree.Element {
	return p.problematicMessage("2", "WARNING", marker, "Inline "+kind+" start-string without end-string.")
}

// problematicMessage builds one <problematic>/<system_message> pair —
// the id/refid/backref cross-linking shape every one of this parser's
// inline-time diagnostics shares (docutils.parsers.rst.states.Inliner.
// problematic + inliner.reporter.{warning,error}, read directly) — with
// rawtext as the <problematic>'s own visible content and message as the
// <system_message>'s text. level/msgType are the caller's own concern
// (markupProblematic's start-string case is always a WARNING; a role's
// own validation failure — see roleError, codeRoleElement — can be
// either, per docutils' own choice for that specific role).
func (p *parser) problematicMessage(level, msgType, rawtext, message string) *doctree.Element {
	p.msgCount++
	n := strconv.Itoa(p.msgCount)
	prb := doctree.NewElement(doctree.TagProblematic, &doctree.Text{Data: rawtext})
	prb.SetAttr("id", "problematic-"+n)
	prb.SetAttr("refid", "system-message-"+n)
	msg := doctree.NewElement(doctree.TagSystemMessage,
		doctree.NewElement(doctree.TagParagraph, &doctree.Text{Data: message}))
	msg.SetAttr("id", "system-message-"+n)
	msg.SetAttr("backref", "problematic-"+n)
	msg.SetAttr("level", level)
	msg.SetAttr("type", msgType)
	if p.currentLine != 0 {
		msg.SetAttr("line", strconv.Itoa(p.currentLine))
	}
	p.messages = append(p.messages, msg)
	return prb
}

// referenceOrPhrase builds either a title_reference (bare `text`, no
// trailing underscore — docutils' DEFAULT role) or a `text`_ / `text`__
// reference node (checking the text immediately after the closing
// backquote for the trailing underscore(s)), and returns how many extra
// runes that consumed. contentRunes is the STILL-ESCAPED form (before
// the general unescapeRunes pass) — needed so splitEmbeddedLink can
// apply the embedded-link-specific escape rule (Inliner.phrase_ref,
// states.py, read directly): within an embedded <URI or alias>, an
// escaped space/newline becomes a literal SPACE rather than being
// dropped the way the general unescape rule drops it everywhere else.
func (p *parser) referenceOrPhrase(contentRunes []rune, afterClose int, runes []rune) ([]doctree.Node, int) {
	content := unescapeRunes(contentRunes)
	if !(afterClose < len(runes) && runes[afterClose] == '_') {
		if p.defaultRole != "" {
			return []doctree.Node{p.roleElement(p.defaultRole, contentRunes)}, 0
		}
		return []doctree.Node{doctree.NewElement(doctree.TagTitleReference, &doctree.Text{Data: content})}, 0
	}
	anonymous := false
	extra := 1
	if afterClose+1 < len(runes) && runes[afterClose+1] == '_' {
		anonymous = true
		extra = 2
	}
	display, kind, targetRunes, hasEmbedded := splitEmbeddedLink(contentRunes)

	var targetValue string
	if hasEmbedded {
		if kind == "uri" {
			targetValue = adjustEmbeddedURI(joinEmbeddedURI(targetRunes))
		} else {
			targetValue = normalizeName(unescapeRunes(targetRunes))
		}
	}
	text := content
	if hasEmbedded {
		text = display
	}
	if hasEmbedded && text == "" {
		// Omitted reference text ("`<uri>`_"/"`<alias_>`__"): the
		// alias/URI text itself becomes the reference's own display
		// text too (real docutils: "if not text: text = alias").
		if kind == "uri" {
			text = joinEmbeddedURI(targetRunes)
		} else {
			text = normalizeWhitespace(unescapeRunes(targetRunes))
		}
	}
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: text})
	el.SetAttr("name", normalizeWhitespace(text))

	switch {
	case hasEmbedded && kind == "uri":
		el.SetAttr("refuri", targetValue)
	case hasEmbedded && kind == "name":
		el.SetAttr("refname", targetValue)
	case anonymous:
		el.SetAttr("anonymous", "true")
	default:
		el.SetAttr("refname", normalizeName(text))
	}

	if !hasEmbedded || anonymous {
		// A bare reference (no embedded link) or an ANONYMOUS ("__")
		// one WITH an embedded link both skip the implicit-target
		// sibling: real docutils only ever appends the target node in
		// the singly-underscored (named) branch (Inliner.phrase_ref's
		// own "if rawsource[-2:] == '__'" split, read directly) — an
		// anonymous embedded-link reference resolves directly off its
		// own refuri/refname, never through document-order anonymous-
		// target matching the way a bare "text__" does.
		return []doctree.Node{el}, extra
	}

	target := doctree.NewElement(doctree.TagTarget)
	// Uses `text`, not `display`: when the reference text was omitted
	// (the "if hasEmbedded && text == ''" fallback above), `display`
	// itself stays empty — the target's own name must still be the
	// resolved display text (real docutils: target['names'].append(
	// normalize_name(unescaped)), computed AFTER that same fallback
	// reassigns `unescaped`, read directly).
	displayName := normalizeName(text)
	target.SetAttr("name", displayName)
	target.SetAttr("id", makeID(displayName))
	if kind == "uri" {
		target.SetAttr("refuri", targetValue)
	} else {
		target.SetAttr("refname", targetValue)
	}
	return []doctree.Node{el, target}, extra
}

// splitEmbeddedLink recognizes docutils' "embedded URI or alias" form
// of a phrase reference: `display text <target>`_, where <target> is
// either a URI (kind "uri") or, when it ends in an UNESCAPED "_" and
// doesn't look like a URI itself, the name of another target defined
// elsewhere (kind "name"). Operates on runes still in escaped form
// (escapeBackslashes' private-use-area encoding) so an escaped "<"/">"
// is never mistaken for a real delimiter — unlike docutils' own
// \x00-marker-based regex, this falls out for free from rune identity,
// no separate "is this escaped" check needed. The "<" must be preceded
// by one or more literal spaces/newlines, OR start the content, and NOT
// be immediately followed by whitespace; the ">" must NOT be
// immediately preceded by whitespace (docutils' embedded_link pattern,
// read directly) — a multi-line phrase reference (the "<" landing right
// after a real line-wrap newline, or the target text itself spanning a
// line break) is a real, corpus-verified shape, not an edge case.
func splitEmbeddedLink(contentRunes []rune) (display, kind string, targetRunes []rune, ok bool) {
	if len(contentRunes) == 0 || contentRunes[len(contentRunes)-1] != '>' {
		return "", "", nil, false
	}
	// A ">" immediately preceded by whitespace disqualifies the whole
	// match (non_whitespace_escape_before) — real docutils also
	// excludes an escaped char there, which (as above) can't arise
	// from a plain '>' match on these runes anyway.
	if len(contentRunes) >= 2 && unicode.IsSpace(contentRunes[len(contentRunes)-2]) {
		return "", "", nil, false
	}
	idx := -1
	for j := len(contentRunes) - 2; j >= 0; j-- {
		if contentRunes[j] == '<' {
			idx = j
			break
		}
	}
	if idx < 0 {
		return "", "", nil, false
	}
	if idx+1 < len(contentRunes)-1 && unicode.IsSpace(contentRunes[idx+1]) {
		return "", "", nil, false
	}
	before := idx
	for before > 0 && unicode.IsSpace(contentRunes[before-1]) {
		before--
	}
	if before > 0 && !unicode.IsSpace(contentRunes[before]) {
		// Nothing but non-whitespace runs right up to "<" — no
		// preceding space/newline and not the start of the content
		// either, so this "<" doesn't open an embedded link at all
		// (docutils: "no preceding whitespace" falls through to plain
		// anonymous/bare-text handling instead).
		return "", "", nil, false
	}
	targetRunes = contentRunes[idx+1 : len(contentRunes)-1]
	if len(targetRunes) == 0 {
		return "", "", nil, false
	}
	display = strings.TrimRight(unescapeRunes(contentRunes[:before]), " ")
	last := targetRunes[len(targetRunes)-1]
	if last == '_' && !looksLikeEmbeddedURIScheme(targetRunes) {
		return display, "name", targetRunes[:len(targetRunes)-1], true
	}
	return display, "uri", targetRunes, true
}

// looksLikeEmbeddedURIScheme reports whether targetRunes starts with a
// URI scheme ("letter (letter|digit|+|-|.)* :") — real docutils tests
// the full aliastext against its own broad uri pattern to distinguish a
// URI that happens to end in "_" from an alias name; this project's own
// standalone-URI recognition (tryURIScheme) is already a simplification
// of that same pattern, and no corpus case needs anything richer than a
// scheme-prefix check here.
func looksLikeEmbeddedURIScheme(runes []rune) bool {
	if len(runes) == 0 || !unicode.IsLetter(runes[0]) {
		return false
	}
	i := 1
	for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) ||
		runes[i] == '+' || runes[i] == '-' || runes[i] == '.') {
		i++
	}
	return i < len(runes) && runes[i] == ':'
}

// joinEmbeddedURI mirrors phrase_ref's own "remove unescaped whitespace"
// treatment of an embedded URI/email target (states.py's
// split_escaped_whitespace + the per-part ”.join(part.split()), read
// directly): split at escaped-whitespace-rune boundaries — an escaped
// space/newline becomes exactly one literal space in the result — then
// strip every OTHER (real) whitespace rune within each part entirely, a
// line-wrap landing inside the "<...>" vanishing with no replacement.
// Any other escape is restored to its literal character normally.
func joinEmbeddedURI(targetRunes []rune) string {
	var parts []string
	var cur strings.Builder
	for _, r := range targetRunes {
		if isDroppedEscape(r) {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		cur.WriteRune(unescapeRune(r))
	}
	parts = append(parts, cur.String())
	return strings.Join(parts, " ")
}

// adjustEmbeddedURI mirrors Inliner.adjust_uri: an embedded target that
// looks like a bare email address gets "mailto:" prefixed.
func adjustEmbeddedURI(uri string) string {
	if strings.Contains(uri, "@") && !strings.Contains(uri, "://") {
		return "mailto:" + uri
	}
	return uri
}

// tryInterpretedOrPhraseRef handles every backtick-quoted construct:
// role-prefixed (:role:`x`), role-suffixed (`x`:role:), a plain phrase
// reference or bare title_reference (referenceOrPhrase, no role at
// all). docutils.parsers.rst.states.Inliner.interpreted_or_phrase_ref.
func (p *parser) tryInterpretedOrPhraseRef(runes []rune, i int) ([]doctree.Node, int, bool) {
	backtickAt := i
	prefixRole := ""
	if runes[i] == ':' {
		role, afterColon, ok := tryRoleName(runes, i+1)
		if !ok || afterColon >= len(runes) || runes[afterColon] != '`' {
			return nil, 0, false
		}
		prefixRole = role
		backtickAt = afterColon
	}
	if runes[backtickAt] != '`' {
		return nil, 0, false
	}
	if backtickAt+1 < len(runes) && runes[backtickAt+1] == '`' {
		// A single backtick immediately followed by another is the
		// START of a double-backtick literal (``x``), not interpreted
		// text — docutils' backquote pattern is '`(?!`)'. Let tryMarker
		// handle it instead.
		return nil, 0, false
	}
	if !validStartBoundary(runes, i, backtickAt-i+1) {
		return nil, 0, false
	}
	closeAt, closeLen, ok := findCloseBackquote(runes, backtickAt+1)
	if !ok {
		if prefixRole != "" {
			// A ":role:`unclosed still ends up exactly right: real
			// docutils' <problematic> here covers only the bare backquote
			// (matchstart/matchend bind to the BACKQUOTE match, not the
			// role prefix before it), with the ":role:" text staying
			// ordinary plain text ahead of it. This function's own
			// (node, consumed, ok) contract can't say "some of this span
			// is plain text, then a node" in one call — but it doesn't
			// need to: returning false here just makes parseInline's
			// caller consume ":role:" one rune at a time as plain text
			// (its normal fallback for "no match"), and the NEXT call,
			// once i reaches the backquote itself with no role prefix in
			// front of it, falls through to the branch below and produces
			// the identical <problematic> real docutils does — verified
			// against the foreign judge, not assumed.
			return nil, 0, false
		}
		return []doctree.Node{p.markupProblematic("interpreted text or phrase reference", "`")}, 1, true
	}
	contentRunes := runes[backtickAt+1 : closeAt]
	if len(contentRunes) == 0 {
		return nil, 0, false
	}
	afterClose := closeAt + closeLen

	// A role may appear as a prefix or a suffix, never both. docutils'
	// end pattern matches the closing backquote, an OPTIONAL suffix role,
	// and an OPTIONAL reference suffix ("_"/"__") in one go, then rejects
	// the two illegal combinations before resolving the role at all —
	// which is why neither diagnostic below carries the unknown-role
	// messages this parser deliberately doesn't emit (README): they are
	// SYNTAX errors, reported for a perfectly well-known role too,
	// checked directly against the reference for :emphasis: in every
	// position.
	suffixRole, end := "", afterClose
	if afterClose < len(runes) && runes[afterClose] == ':' {
		if role, afterRole, ok := tryRoleName(runes, afterClose+1); ok {
			suffixRole, end = role, afterRole
		}
	}
	if prefixRole != "" && suffixRole != "" {
		return []doctree.Node{p.problematicMessage("2", "WARNING", string(runes[i:end]),
			"Multiple roles in interpreted text (both prefix and suffix present; only one allowed).")}, end - i, true
	}

	// docutils tests rawsource[-1:] == "_", so a reference suffix after a
	// role — in EITHER position — is the "Mismatch" warning rather than a
	// phrase reference. The message names the position the role was in.
	if role, position := prefixRole+suffixRole, positionOfRole(prefixRole, suffixRole); role != "" {
		if refEnd, isRef := scanReferenceSuffix(runes, end); isRef {
			return []doctree.Node{p.problematicMessage("2", "WARNING", string(runes[i:refEnd]),
				"Mismatch: both interpreted text role "+position+" and reference suffix.")}, refEnd - i, true
		}
		return []doctree.Node{p.roleElement(role, contentRunes)}, end - i, true
	}

	nodes, extra := p.referenceOrPhrase(contentRunes, afterClose, runes)
	return nodes, (afterClose - i) + extra, true
}

// positionOfRole names which side the role was found on, for the
// "Mismatch: both interpreted text role %s and reference suffix."
// message. Exactly one of the two is ever non-empty by the time this is
// called — the both-present case is the separate warning above.
func positionOfRole(prefixRole, suffixRole string) string {
	if prefixRole != "" {
		return "prefix"
	}
	return "suffix"
}

// scanReferenceSuffix reports whether a reference suffix ("_" or "__")
// starts at i, returning the index right after it.
func scanReferenceSuffix(runes []rune, i int) (int, bool) {
	if i >= len(runes) || runes[i] != '_' {
		return i, false
	}
	if i+1 < len(runes) && runes[i+1] == '_' {
		return i + 2, true
	}
	return i + 1, true
}

// tryRoleName recognizes a ":name" immediately followed by a closing
// ":" starting at `from` (the position right after the opening ":"),
// returning the role name and the position right after the closing ":".
// A role name is docutils' own `simplename` (states.py: the role group is
// `(?P<role>:%(simplename)s:)?`), NOT the letters/digits/hyphen subset an
// earlier version of this scanned for: ".", "_", "+" and ":" are all
// legal too, so ":very.long-role_name:`x`" used to be missed entirely and
// left as plain text with a bare <title_reference> beside it. scanSimpleName
// is greedy over ":" as well, which is what real docutils does — ":a:b:`x`"
// is the single role "a:b", not "a" — verified against the reference.
func tryRoleName(runes []rune, from int) (name string, after int, ok bool) {
	j := scanSimpleName(runes, from)
	if j == from || j >= len(runes) || runes[j] != ':' {
		return "", 0, false
	}
	return string(runes[from:j]), j + 1, true
}

// roleTags maps docutils' built-in role canonical/alias names
// (docutils.parsers.rst.languages.en.roles, English only — this parser
// doesn't support other languages' role names) to the doctree tag they
// produce. A role not in this table falls back to a generic <inline>.
//
// Most entries here are the GENERIC roles (emphasis/strong/literal/...),
// which really do just alias an existing markup tag. "math" is one of
// docutils' other always-registered roles (roles.py's math_role) and is
// simplified the same way, always producing a dedicated <math> node
// (never <inline>, unlike every other role here) holding the raw,
// unparsed TeX source. "code" (roles.py's code_role) and "pep"/"rfc"
// (pep_reference_role/rfc_reference_role) are NOT simple tag aliases —
// each has its own real validation/class-list logic — see roleElement's
// own dispatch for those, codeRoleElement, pepRole, rfcRole.
var roleTags = map[string]string{
	"emphasis":        doctree.TagEmphasis,
	"strong":          doctree.TagStrong,
	"literal":         doctree.TagLiteral,
	"subscript":       doctree.TagSubscript,
	"sub":             doctree.TagSubscript,
	"superscript":     doctree.TagSuperscript,
	"sup":             doctree.TagSuperscript,
	"title-reference": doctree.TagTitleReference,
	"title":           doctree.TagTitleReference,
	"t":               doctree.TagTitleReference,
	"abbreviation":    doctree.TagAbbreviation,
	"ab":              doctree.TagAbbreviation,
	"acronym":         doctree.TagAcronym,
	"ac":              doctree.TagAcronym,
	"math":            doctree.TagMath,
}

// roleElement dispatches one interpreted-text role invocation: a built-in
// GENERIC role (roleTags) first, then — docutils/rst v0.16.0+ — a
// ".. role::"-registered one (see role.go), which either aliases a
// roleTags entry the same way a built-in does, or (its base is "raw") is
// this parser's one INLINE raw construct, mirroring the block-level "raw"
// directive: p.opts.RawEnabled gates it exactly the same way. Every one
// of those custom-role shapes carries a "class" attribute (real docutils'
// roles.py set_implicit_options: EVERY role function implicitly supports
// a ":class:" option, defaulted by the "role" directive itself to the
// role's own name — see registerRole/classOption, role.go), which this
// function attaches wherever it produces a node for a role FOUND in
// p.roles, whether or not it aliases a roleTags tag. A role name never
// registered via ".. role::" at all — genuinely unknown, whether or not
// it LOOKS like it could be a real Sphinx/extension role this parser has
// just never heard of — still falls back to a generic
// <inline role="name">, matching this parser's existing lenient
// behavior; real docutils instead errors and rewrites to `problematic`
// here, not replicated (see the package doc comment). That "role="
// attribute is this parser's OWN invented shorthand for that one
// unvalidated fallback case specifically — never confuse it with the
// "class=" a REGISTERED custom role (even a bare one with no base at
// all, docutils' generic_custom_role) always carries instead.
//
// contentRunes is the role's content BEFORE the usual backslash-unescape
// pass (see escapeBackslashes/unescapeRunes) — needed here, not resolved
// by the caller, because a raw-based role's content is never reST at all,
// so a backslash in it is literal target-format syntax (`\textbf`), not a
// reST escape sequence to strip: unescapeRunes would have silently eaten
// it, confirmed against the foreign judge (`:mytex:`\textbf{bold}“ with
// a "latex"-based custom role must keep the backslash; unescapeRunes
// alone turned it into bare "textbf{bold}", breaking the LaTeX it was
// supposed to be).
func (p *parser) roleElement(role string, contentRunes []rune) *doctree.Element {
	name := strings.ToLower(role)
	switch name {
	case "pep", "pep-reference":
		return p.pepRole(role, contentRunes)
	case "rfc", "rfc-reference":
		return p.rfcRole(role, contentRunes)
	case "code":
		classes, _ := codeRoleClasses("code", "", false, nil)
		return p.codeRoleElement(role, classes, "", false, contentRunes)
	case "raw":
		// The BUILT-IN "raw" role invoked directly (no ".. role::" at
		// all, so no :format: option can ever reach it) always errors —
		// roles.py's raw_role, read directly. When raw is DISABLED, real
		// docutils errors here too ("raw (and derived) roles disabled"),
		// but this parser's own established, deliberately-simplified
		// convention for a disabled raw role is the generic <inline
		// role="..."> fallback (see TestRawRoleDisabledFallsBackToGeneric)
		// — kept consistent by only taking this path when enabled.
		if p.opts.RawEnabled {
			text := restoreEscapes(contentRunes)
			return p.problematicMessage("3", "ERROR", ":"+role+":`"+text+"`",
				"No format (Writer name) is associated with this role: \""+role+"\".\n"+
					"The \"raw\" role cannot be used directly.\n"+
					"Instead, use the \"role\" directive to create a new role with an associated format.")
		}
	}
	if tag, ok := roleTags[name]; ok {
		return doctree.NewElement(tag, &doctree.Text{Data: unescapeRunes(contentRunes)})
	}
	if def, ok := p.roles[name]; ok {
		if def.base == "raw" && !p.opts.RawEnabled {
			// Same established simplification as the direct-"raw" case
			// above: fall through to the generic role="name" shape at
			// the bottom, not real docutils' own ERROR+problematic.
		} else if def.base == "raw" {
			el := doctree.NewElement(doctree.TagRaw, &doctree.Text{Data: restoreEscapes(contentRunes)})
			el.SetAttr("format", def.format)
			if len(def.classes) > 0 {
				el.SetAttr("class", strings.Join(def.classes, " "))
			}
			return el
		} else if def.base == "code" {
			classes, lang := codeRoleClasses(name, def.language, def.hasLanguage, def.classes)
			return p.codeRoleElement(role, classes, lang, def.hasLanguage, contentRunes)
		} else if def.base != "" {
			if tag, ok := roleTags[def.base]; ok {
				el := doctree.NewElement(tag, &doctree.Text{Data: unescapeRunes(contentRunes)})
				if len(def.classes) > 0 {
					el.SetAttr("class", strings.Join(def.classes, " "))
				}
				return el
			}
		} else {
			// A bare custom role — no base at all — is docutils' own
			// generic_custom_role: <inline class="...">, never
			// <inline role="...">.
			el := doctree.NewElement(doctree.TagInline, &doctree.Text{Data: unescapeRunes(contentRunes)})
			if len(def.classes) > 0 {
				el.SetAttr("class", strings.Join(def.classes, " "))
			}
			return el
		}
	}
	el := doctree.NewElement(doctree.TagInline, &doctree.Text{Data: unescapeRunes(contentRunes)})
	el.SetAttr("role", name)
	return el
}

// codeRoleClasses replicates docutils.parsers.rst.roles.code_role's own
// class-list/highlight-language derivation (roles.py, read directly):
// roleName supplies the DEFAULT highlight language for anything but the
// literal built-in "code" role itself (a custom role's own local name,
// e.g. "python" for ".. role:: python(code)"); an explicit ":language:"
// option (hasLanguage) always overrides that default outright, including
// to the empty string via the literal value "none" (case-insensitively),
// which disables highlighting entirely. extraClasses is the role
// definition's own (already-defaulted-to-its-own-name, see registerRole)
// ":class:" option value.
func codeRoleClasses(roleName, language string, hasLanguage bool, extraClasses []string) (classes []string, resolvedLanguage string) {
	lang := ""
	if roleName != "code" {
		lang = roleName
	}
	if hasLanguage {
		lang = language
	}
	if strings.EqualFold(lang, "none") {
		lang = ""
	}
	classes = append([]string{"code"}, extraClasses...)
	if lang != "" && !containsString(classes, lang) {
		classes = append(classes, lang)
	}
	return classes, lang
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// codeRoleElement replicates docutils.parsers.rst.roles.code_role's own
// content handling (roles.py, read directly): raw, backslash-PRESERVING
// content (nodes.unescape(text, True) — the same restoreEscapes convention
// this parser already uses for a raw-based role's content, since code
// content is never reST either), wrapped in <literal class="...">. When a
// non-empty highlight language is resolved, real docutils attempts
// Pygments tokenization; this parser has no Pygments equivalent and never
// will (pure Go, no external process), so it always takes docutils' own
// LexerError path — which itself branches on whether ":language:" was
// EXPLICITLY given (hasLanguage) rather than merely implied by the custom
// role's own name: explicit means Pygments was actually asked for and
// visibly failed (a WARNING + <problematic>, "Cannot analyze code.
// Pygments package not found."); implicit means the language was only a
// DEFAULT guess, so docutils itself falls back silently to the plain,
// unclassified content — the same shape as the language=="" case.
// Verified against the foreign judge's LIVE (Pygments-less) re-parse, not
// the docutils testsuite's own baked-in fixture text (captured on a
// machine that DOES have Pygments installed, showing real tokenization —
// see the sweep tool's own ExpectedDefault field/doc comment for why that
// field, not the baked-in one, is this project's actual ground truth).
func (p *parser) codeRoleElement(role string, classes []string, language string, hasLanguage bool, contentRunes []rune) *doctree.Element {
	rawtext := restoreEscapes(contentRunes)
	if language != "" && hasLanguage {
		return p.problematicMessage("2", "WARNING", ":"+role+":`"+rawtext+"`",
			"Cannot analyze code. Pygments package not found.")
	}
	el := doctree.NewElement(doctree.TagLiteral, &doctree.Text{Data: rawtext})
	el.SetAttr("class", strings.Join(classes, " "))
	return el
}

// pepRole replicates docutils.parsers.rst.roles.pep_reference_role
// (roles.py, read directly): the interpreted text must be an integer
// 0-9999, producing a <reference> to the PEP's page on success or an
// ERROR + <problematic> otherwise. Only the prefix form (":PEP:`n`") is
// reconstructed into the <problematic>'s rawtext — no corpus case
// currently exercises the suffix form ("`n`:PEP:") failing, which would
// need the original delimiter order reconstructed differently.
func (p *parser) pepRole(role string, contentRunes []rune) *doctree.Element {
	text := unescapeRunes(contentRunes)
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 || n > 9999 {
		return p.problematicMessage("3", "ERROR", ":"+role+":`"+text+"`",
			`PEP number must be a number from 0 to 9999; "`+text+`" is invalid.`)
	}
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: "PEP " + text})
	el.SetAttr("refuri", "https://peps.python.org/pep-"+pad4(n))
	return el
}

// rfcRole replicates docutils.parsers.rst.roles.rfc_reference_role
// (roles.py, read directly): text is an integer >= 1, optionally followed
// by "#section"; same prefix-form-only <problematic> scope as pepRole.
func (p *parser) rfcRole(role string, contentRunes []rune) *doctree.Element {
	text := unescapeRunes(contentRunes)
	numPart, section, hasSection := text, "", false
	if idx := strings.IndexByte(text, '#'); idx >= 0 {
		numPart, section = text[:idx], text[idx+1:]
		hasSection = true
	}
	n, err := strconv.Atoi(numPart)
	if err != nil || n < 1 {
		return p.problematicMessage("3", "ERROR", ":"+role+":`"+text+"`",
			`RFC number must be a number greater than or equal to 1; "`+text+`" is invalid.`)
	}
	refuri := "https://tools.ietf.org/html/rfc" + strconv.Itoa(n) + ".html"
	if hasSection {
		refuri += "#" + section
	}
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: "RFC " + strconv.Itoa(n)})
	el.SetAttr("refuri", refuri)
	return el
}

// pad4 zero-pads n to at least 4 digits ("pep-%04d" % pepnum, states.py
// read directly). Callers only ever pass an already-range-checked
// 0-9999 value.
func pad4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// tryBareReference recognizes a reference with no backtick delimiters
// at all: word_ / word__ (docutils' 'whole' construct: a `simplename`
// immediately followed by one or two trailing underscores — see
// scanSimpleName for exactly what a simplename allows, including an
// internal single underscore, contrary to what an earlier version of this
// comment claimed without checking).
func tryBareReference(runes []rune, i int) (doctree.Node, int, bool) {
	if !isAlphaNumRune(runes[i]) || !validStartBoundary(runes, i, 0) {
		return nil, 0, false
	}
	j := scanSimpleName(runes, i)
	if j == i || j >= len(runes) || runes[j] != '_' {
		return nil, 0, false
	}
	anonymous := false
	end := j + 1
	if end < len(runes) && runes[end] == '_' {
		anonymous = true
		end++
	}
	if end < len(runes) {
		next := runes[end]
		if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
			return nil, 0, false
		}
	}
	name := unescapeRunes(runes[i:j])
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: name})
	el.SetAttr("name", normalizeWhitespace(name))
	if anonymous {
		el.SetAttr("anonymous", "true")
	} else {
		el.SetAttr("refname", normalizeName(name))
	}
	return el, end - i, true
}

// tryInlineTarget recognizes an inline internal target, "_`text`" — a
// target INSIDE a paragraph (as opposed to the block-level ".. _name: uri"
// hyperlink target explicit.go's parseHyperlinkTarget handles), docutils'
// own target pattern in Inliner.patterns. Unlike a block target, this one
// keeps its content as visible text (docutils: <target ids="..."
// names="...">text</target>) — the name a reference resolves against comes
// from that text, normalized the same way as any other reference name.
// A start-string with no valid end-string (the closing backquote found,
// but not itself followed by a valid end boundary — e.g. “ _`this`_ “,
// where the trailing "_" right after the close is neither whitespace
// nor a closer/delimiter, so real docutils keeps searching for a LATER
// close that never comes) becomes a <problematic> with the "Inline
// target start-string without end-string." warning, the same
// commit-once behavior tryMarker's own non-substitution branch already
// has — this used to return "no match" silently instead, leaving the
// whole construct as unremarked plain text.
func (p *parser) tryInlineTarget(runes []rune, i int) (doctree.Node, int, bool) {
	if runes[i] != '_' || i+1 >= len(runes) || runes[i+1] != '`' {
		return nil, 0, false
	}
	if !validStartBoundary(runes, i, 2) {
		return nil, 0, false
	}
	closeAt, closeLen, ok := findClose(runes, i+2, "`")
	if !ok {
		return p.markupProblematic("target", "_`"), 2, true
	}
	content := unescapeRunes(runes[i+2 : closeAt])
	if content == "" {
		return nil, 0, false
	}
	el := doctree.NewElement(doctree.TagTarget, &doctree.Text{Data: content})
	name := normalizeName(content)
	el.SetAttr("name", name)
	el.SetAttr("id", makeID(name))
	return el, (closeAt + closeLen) - i, true
}

// scanSimpleName scans the longest match of docutils' own "simplename"
// grammar (states.py: `(?:(?!_)\w)+(?:[-._+:](?:(?!_)\w)+)*`) starting at
// i, returning the index right after it (or i itself for no match at
// all): one or more ALPHANUMERIC runs, joined by exactly one separator
// character (-, ., _, +, or :) between runs. This is the (previously
// unverified — see tryBareReference's doc comment) reason a name CAN
// legitimately contain an underscore ("real_target") without it being
// mistaken for the trailing reference-suffix underscore: it's a single
// separator between two alphanumeric runs, not a doubled, leading, or
// trailing one, and neither hyphen nor any other separator can start a
// name either — verified against real docutils for "real_target_"
// (matches whole), "some--name_" (only "name" matches, the double hyphen
// breaks it), "-foo_" (leading hyphen doesn't match, only "foo" does),
// and "foo-bar_baz_" (the whole thing matches; the final lone "_" is
// correctly left as the reference suffix, not consumed as a separator
// with nothing after it).
func scanSimpleName(runes []rune, i int) int {
	j := scanAlphaNumRun(runes, i)
	if j == i {
		return i
	}
	for j < len(runes) && isNameSepRune(runes[j]) {
		k := scanAlphaNumRun(runes, j+1)
		if k == j+1 {
			return j
		}
		j = k
	}
	return j
}

func scanAlphaNumRun(runes []rune, i int) int {
	j := i
	for j < len(runes) && isAlphaNumRune(runes[j]) {
		j++
	}
	return j
}

func isAlphaNumRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isNameSepRune(r rune) bool {
	return r == '-' || r == '.' || r == '_' || r == '+' || r == ':'
}

// tryStandaloneURI recognizes a bare "scheme://..." URI or "user@host"
// email address with no reST markup at all — docutils'
// Inliner.standalone_uri, tried only as a fallback once nothing else
// matches (implicit_dispatch). No refname/"name" attribute is set here,
// matching docutils: a standalone URI reference carries only refuri.
// SCOPE: only a "scheme://" (double-slash) URI is recognized — real
// docutils' own uri pattern also accepts a bare "scheme:path" with no
// "//" at all (mailto:, news:, urn: and friends), a real, separate,
// not-yet-ported gap (test_inline_markup.py's own standalone_hyperlink
// group, not chased this round).
func tryStandaloneURI(runes []rune, i int) (doctree.Node, int, bool) {
	if !validStartBoundary(runes, i, 0) {
		return nil, 0, false
	}
	if node, n, ok := tryURIScheme(runes, i); ok {
		return node, n, true
	}
	return tryEmail(runes, i)
}

// tryURIScheme's own matched span is unescaped before use — a real,
// previously-shipped bug (found chasing the phrase-reference corpus
// work, unrelated to it): a backslash-escaped character inside a
// standalone URI (rare, but corpus-tested — "http://x/\*content\*/y")
// leaked its raw escapeRune-shifted codepoint straight into the visible
// text/refuri, since escapeBackslashes' encoding is never meant to
// survive into rendered output unresolved.
func tryURIScheme(runes []rune, i int) (doctree.Node, int, bool) {
	if !unicode.IsLetter(runes[i]) {
		return nil, 0, false
	}
	j := i + 1
	for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) ||
		runes[j] == '+' || runes[j] == '-' || runes[j] == '.') {
		j++
	}
	if !hasPrefixAt(runes, j, "://") {
		return nil, 0, false
	}
	start := j + 3
	k := start
	for k < len(runes) && !unicode.IsSpace(runes[k]) {
		k++
	}
	end := trimTrailingURIPunct(runes, start, k)
	if end <= start {
		return nil, 0, false
	}
	if end < len(runes) {
		next := runes[end]
		if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
			return nil, 0, false
		}
	}
	text := unescapeRunes(runes[i:end])
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: text})
	el.SetAttr("refuri", text)
	return el, end - i, true
}

func tryEmail(runes []rune, i int) (doctree.Node, int, bool) {
	j := i
	for j < len(runes) && isEmailLocalChar(runes[j]) {
		j++
	}
	if j == i || j >= len(runes) || runes[j] != '@' {
		return nil, 0, false
	}
	j++
	domainStart := j
	for j < len(runes) && isEmailDomainChar(runes[j]) {
		j++
	}
	if !strings.Contains(string(runes[domainStart:j]), ".") {
		return nil, 0, false
	}
	end := trimTrailingURIPunct(runes, i, j)
	if end <= domainStart {
		return nil, 0, false
	}
	if end < len(runes) {
		next := runes[end]
		if !unicode.IsSpace(next) && !unicode.IsPunct(next) {
			return nil, 0, false
		}
	}
	text := unescapeRunes(runes[i:end])
	el := doctree.NewElement(doctree.TagReference, &doctree.Text{Data: text})
	el.SetAttr("refuri", "mailto:"+text)
	return el, end - i, true
}

func isEmailLocalChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(".+_%-", r)
}

func isEmailDomainChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-'
}

// trimTrailingURIPunct drops trailing punctuation unlikely to be part
// of the URI itself (matching docutils' uri_end character class in
// spirit, not exactly): a URL followed by ", " or "." at a sentence's
// end shouldn't swallow that punctuation into the link.
func trimTrailingURIPunct(runes []rune, start, end int) int {
	for end > start && strings.ContainsRune(`.,;:!?)]}'"`, runes[end-1]) {
		end--
	}
	return end
}

// tryFootnoteRef recognizes "[label]_" — a footnote reference ("[1]_",
// "[#]_", "[#name]_", "[*]_") or citation reference ("[name]_"),
// docutils' Inliner.footnote_reference. Unlike the phrase-reference
// form, only a single trailing "_" is meaningful here (no "__" form).
func (p *parser) tryFootnoteRef(runes []rune, i int) (doctree.Node, int, bool) {
	if runes[i] != '[' || !validStartBoundary(runes, i, 1) {
		return nil, 0, false
	}
	end := -1
	for j := i + 1; j < len(runes); j++ {
		if runes[j] == ']' {
			end = j
			break
		}
	}
	if end < 0 || end+1 >= len(runes) || runes[end+1] != '_' {
		return nil, 0, false
	}
	label := unescapeRunes(runes[i+1 : end])
	if label == "" {
		return nil, 0, false
	}
	// The label must BE a footnote or citation label, not just any text
	// between brackets — docutils' footnotelabel is `[0-9]+|#|#simplename|
	// \*` and its citationlabel a bare simplename, the same constructs
	// table the BLOCK-level ".. [label]" dispatch uses (explicit.go's
	// matchBracketLabel, gated on this same helper since v0.53.0; the
	// inline side was left over-accepting, so "[CIT 1]_" built a real
	// <citation_reference> named "cit 1" instead of staying plain text).
	if !isValidBracketLabel(label) {
		return nil, 0, false
	}
	total := end + 2 - i
	if end+2 < len(runes) && !isValidEndBoundaryChar(runes[end+2]) {
		// Real docutils' end_string_suffix, same as every other
		// construct's own close (findClose/
		// validEndBoundaryAfterOptionalUnderscores) — this used to be a
		// blanket unicode.IsPunct check, which wrongly accepted an
		// OPENER like '[' right after the closing "_" (unicode.IsPunct
		// doesn't distinguish openers from closers the way
		// punctuation_chars does), letting an adjacent "[label]_[other]_"
		// run resolve each bracket independently instead of correctly
		// rejecting every one of them (a corpus-verified real-world
		// shape: several footnote/citation references with no
		// separating whitespace at all).
		return nil, 0, false
	}

	var el *doctree.Element
	switch {
	case label == "*":
		el = doctree.NewElement(doctree.TagFootnoteReference)
		el.SetAttr("auto", "*")
	case label[0] == '#':
		el = doctree.NewElement(doctree.TagFootnoteReference)
		el.SetAttr("auto", "1")
		if name := label[1:]; name != "" {
			el.SetAttr("refname", normalizeName(name))
		}
	case isAllDigits(label):
		el = doctree.NewElement(doctree.TagFootnoteReference, &doctree.Text{Data: label})
		el.SetAttr("refname", label)
	default:
		el = doctree.NewElement(doctree.TagCitationReference, &doctree.Text{Data: label})
		el.SetAttr("refname", normalizeName(label))
	}
	// Every one of these gets an id at PARSE time, not in a later
	// transform: document.note_{footnote,citation,autofootnote,
	// symbol_footnote}_ref all call set_id, so even an auto-numbered or
	// symbol reference carrying no refname at all still has one.
	el.SetAttr("id", p.autoID(el.Tag))
	return el, total, true
}

func hasPrefixAt(runes []rune, i int, s string) bool {
	rs := []rune(s)
	if i+len(rs) > len(runes) {
		return false
	}
	for k, r := range rs {
		if runes[i+k] != r {
			return false
		}
	}
	return true
}

// validStartBoundary implements a simplified version of docutils'
// start-string-prefix: must be at the beginning of the text, or preceded
// by whitespace or opening punctuation, AND not immediately followed by
// whitespace. The full implementation keys off Unicode General Category
// tables (docutils.utils.punctuation_chars); this uses unicode.IsSpace /
// unicode.IsPunct as a documented approximation, close enough for ASCII
// prose but a known fidelity gap for exotic Unicode punctuation.
// validStartBoundary ports docutils' start_string_prefix (real docutils'
// Inliner.init_customizations, read directly) plus the separate
// quoted_start veto (Inliner.quoted_start): a markup start-string must be
// at the beginning of the text or preceded by whitespace, an OPENER, or a
// DELIMITER — never a CLOSER, a CLOSING-DELIMITER, or an ordinary
// alphanumeric/other character (this parser's own earlier, simpler
// unicode.IsPunct approximation treated an opening and closing
// bracket/quote identically, which is exactly why ")*emphasis*(" used to
// be accepted as genuine markup when real docutils rejects it). The
// character immediately after the marker must not be whitespace (an empty
// gap before real content) and must not be ABSENT entirely (a marker as
// the very last character of the text, with nothing after it at all, is
// real docutils' own quoted_start "no markup start-string either" case —
// verified against the foreign judge: it produces no <problematic> and no
// warning, plain text, not even a failed attempt). Finally, quotedStart
// vetoes a start-string immediately sandwiched between a real matching
// open/close pair with nothing else between (e.g. "(*)text").
func validStartBoundary(runes []rune, i, openLen int) bool {
	if i > 0 {
		// Unescaped for the character-class check only (never treated as
		// a real delimiter itself — isEscapedRune's own separation
		// already prevents that): real docutils' \x00-marker escaping
		// keeps the ORIGINAL character intact right after the marker
		// byte, so a lookbehind like this one sees a genuine space
		// there for free; this project's own escapeRune representation
		// shifts the character's rune value entirely instead, so the
		// SAME "backslash-escaped space is a valid boundary" behavior
		// (`m\ *a*`, a real, corpus-tested construct — every character-
		// level marker in a row, each preceded by an escaped space)
		// needs restoring it explicitly before the class checks below.
		prev := unescapeRune(runes[i-1])
		if !unicode.IsSpace(prev) && !isOpener(prev) && !isDelimiterChar(prev) {
			return false
		}
	}
	if i+openLen >= len(runes) {
		return false
	}
	next := runes[i+openLen]
	if unicode.IsSpace(next) {
		return false
	}
	if i > 0 && quotedStart(runes[i-1], next) {
		return false
	}
	return true
}

// isValidEndBoundaryChar reports whether r may immediately follow a
// closing delimiter under docutils' end_string_suffix — end-of-text
// (checked separately by every caller, since that needs an index bound,
// not a rune), whitespace, a CLOSING-DELIMITER, a DELIMITER, or a
// CLOSER, and an escaped rune (already excluded from ever matching as a
// real delimiter itself, so it's always safe here) — never an OPENER or
// an ordinary character. Shared by findClose,
// validEndBoundaryAfterOptionalUnderscores, and tryFootnoteRef, which
// each independently need the exact same check.
func isValidEndBoundaryChar(r rune) bool {
	return unicode.IsSpace(r) || isClosingDelimiter(r) || isDelimiterChar(r) || isCloser(r) || isEscapedRune(r)
}

// findClose scans forward from `from` for the next occurrence of `open`
// (the close-string is the same characters as the open-string in reST)
// satisfying docutils' end_string_suffix: not immediately preceded by
// whitespace, and followed by end-of-text, whitespace, a CLOSING-DELIMITER,
// a DELIMITER, or a CLOSER — never an OPENER or an ordinary character (the
// same asymmetry validStartBoundary enforces on the other side, replacing
// this parser's earlier blanket unicode.IsPunct approximation).
// findCloseLiteral is findClose for the inline-literal end-string, which
// docutils deliberately spells DIFFERENTLY from every other one. Compare
// the two end patterns as built in states.py:
//
//	emphasis: (?<![\s\x00])(\*)($|(?=...))
//	literal:  (?<!\s)(``)($|(?=...))
//
// The emphasis form refuses a delimiter preceded by the \x00 escape
// marker; the literal form has no \x00 in its lookbehind at all. That is
// the spec's "backslashes are not escapes inside inline literals" made
// mechanical: a backslash before a closing backquote does NOT protect it,
// so "“literal\“" closes normally and keeps the backslash as content,
// and "“a\\“" keeps BOTH backslashes. This project fuses the escape
// marker into the character's own rune value rather than keeping it as a
// separate preceding rune, so restoring docutils' behavior means matching
// an escaped backquote as a backquote here — which the shared findClose,
// used by every marker that DOES honor the escape, must keep refusing.
// All four shapes were checked against the reference.
func findCloseLiteral(runes []rune, from int) (int, int, bool) {
	isBackquote := func(r rune) bool { return unescapeRune(r) == '`' }
	for j := from; j <= len(runes)-2; j++ {
		if !isBackquote(runes[j]) || !isBackquote(runes[j+1]) {
			continue
		}
		if j == from {
			continue // empty content, e.g. "````"
		}
		if unicode.IsSpace(unescapeRune(runes[j-1])) {
			continue
		}
		if j+2 < len(runes) && !isValidEndBoundaryChar(runes[j+2]) {
			continue
		}
		return j, 2, true
	}
	return 0, 0, false
}

func findClose(runes []rune, from int, open string) (int, int, bool) {
	ol := len([]rune(open))
	for j := from; j <= len(runes)-ol; j++ {
		if !hasPrefixAt(runes, j, open) {
			continue
		}
		if j == from {
			continue // empty content, e.g. "****"
		}
		prev := runes[j-1]
		if unicode.IsSpace(prev) {
			continue
		}
		if j+ol < len(runes) && !isValidEndBoundaryChar(runes[j+ol]) {
			continue
		}
		return j, ol, true
	}
	return 0, 0, false
}

// findCloseBackquote is findClose specialized for the interpreted-text/
// phrase-reference backquote — real docutils' own patterns.
// interpreted_or_phrase_ref is a SEPARATE regex from the generic
// emphasis/strong/literal ones, with an optional trailing "__?" (one or
// two underscores, greedy) matched as part of the SAME regex BEFORE
// end_string_suffix is checked — so "`Section`_ for details" is valid
// (end_string_suffix checked after the "_", against the space before
// "for", not immediately after the backquote against the "_" itself,
// which is neither whitespace/delimiter/closer nor an end of text).
// Verified against the foreign judge — a first version of this fix that
// reused the generic findClose broke this construct entirely, turning
// every "`x`_"-style reference into a <problematic>.
//
// closeLen is deliberately ALWAYS 1 (the backquote alone), regardless of
// how many trailing underscores the boundary search looked past: this
// function only decides WHETHER a close is valid, not how much of the
// text it covers — referenceOrPhrase (the caller, one level up) already
// does its own independent check of runes[afterClose] to consume the
// trailing "_"/"__" and decide named vs. anonymous, and must keep seeing
// them un-consumed to do that.
func findCloseBackquote(runes []rune, from int) (int, int, bool) {
	for j := from; j < len(runes); j++ {
		if runes[j] != '`' {
			continue
		}
		if j == from {
			continue // empty content, e.g. "``"
		}
		prev := runes[j-1]
		if unicode.IsSpace(prev) {
			continue
		}
		if validEndBoundaryAfterOptionalUnderscores(runes, j+1) {
			return j, 1, true
		}
	}
	return 0, 0, false
}

// findCloseSubstitution is findClose specialized for substitution_reference
// the same way findCloseBackquote specializes it for the interpreted-text
// backquote: real docutils' own substitution_ref pattern is `(\|_{0,2})` —
// 0, 1, or 2 trailing underscores are part of the SAME close match
// end_string_suffix applies after, so "|sub|_ text" is valid (boundary
// checked after the "_", not immediately after "|" against the "_" itself).
// closeLen is always 1 (the "|" alone), same reasoning as
// findCloseBackquote: the caller (tryMarker's substitution-ref branch)
// already does its own independent runes[after]=='_' check to consume the
// trailing underscore(s) and decide named vs. anonymous.
func findCloseSubstitution(runes []rune, from int) (int, int, bool) {
	for j := from; j < len(runes); j++ {
		if runes[j] != '|' {
			continue
		}
		if j == from {
			continue // empty content, e.g. "||"
		}
		prev := runes[j-1]
		if unicode.IsSpace(prev) {
			continue
		}
		if validEndBoundaryAfterOptionalUnderscores(runes, j+1) {
			return j, 1, true
		}
	}
	return 0, 0, false
}

// validEndBoundaryAfterOptionalUnderscores checks docutils' end_string_suffix
// at `at`, or — greedily, matching "__?" — after skipping 2 underscores, then
// after skipping 1, before finally checking at `at` itself with none skipped.
func validEndBoundaryAfterOptionalUnderscores(runes []rune, at int) bool {
	for _, skip := range [3]int{2, 1, 0} {
		if at+skip > len(runes) {
			continue
		}
		ok := true
		for k := 0; k < skip; k++ {
			if runes[at+k] != '_' {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		pos := at + skip
		if pos >= len(runes) {
			return true
		}
		if isValidEndBoundaryChar(runes[pos]) {
			return true
		}
	}
	return false
}
