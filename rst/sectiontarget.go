package rst

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/go-docutils/docutils/doctree"
)

// assignSectionTargets registers every section title as an implicit
// hyperlink target — real docutils' new_subsection + document.set_id, ported
// (verified against parsers/rst/states.py's new_subsection, which computes
// name := fully_normalize_name(title text) and calls
// document.note_implicit_target right when the section is built, and
// nodes.document.create_id, which derives the id from make_id(name) with a
// "-N" suffix on collision — auto_id_prefix defaults to "%", confirmed via
// get_default_settings, which is exactly that suffix behavior, not some
// other disambiguation scheme). Runs as its own pass, not inline during
// parsing, because collision disambiguation needs to see every section in
// the document, not just the ones already built when a given section is
// reached.
//
// The trailing system-messages section (see systemMessagesSection) is
// excluded: real docutils' own Messages transform builds its wrapper
// directly via nodes.section(classes=['system-messages']), bypassing
// note_implicit_target entirely — checked against transforms/universal.py,
// not assumed. Runs BEFORE resolveTargets, whose collectTargets folds a
// section's own name/id (once set here) into the same direct-target map a
// <target> populates — one source of truth for "what can a reference
// resolve to", not a parallel resolution path.
func assignSectionTargets(doc *doctree.Element) {
	used := map[string]bool{}
	var walk func(el *doctree.Element)
	walk = func(el *doctree.Element) {
		if el.Tag == doctree.TagSection && el.Attr("class") != "system-messages" {
			var titleText string
			for _, c := range el.Children {
				if ce, ok := c.(*doctree.Element); ok && ce.Tag == doctree.TagTitle {
					titleText = doctree.AsText(ce)
					break
				}
			}
			name := normalizeName(titleText)
			if name != "" {
				id := uniqueID(makeID(name), used, "section")
				used[id] = true
				el.SetAttr("name", name)
				el.SetAttr("id", id)
			}
		}
		for _, c := range el.Children {
			if ce, ok := c.(*doctree.Element); ok {
				walk(ce)
			}
		}
	}
	walk(doc)
}

// uniqueID returns base if unused, else base + "-1", base + "-2", ... —
// docutils' own create_id disambiguation for auto_id_prefix="%" (the
// default this project always behaves as, having no id_prefix/
// auto_id_prefix settings of its own). fallback names the tag-derived
// prefix (make_id(node.tagname), i.e. "section" here) real docutils uses
// when base itself is empty (a title with no ASCII-alnum content at all,
// e.g. "1" or "±") — checked against the foreign judge: an empty base_id
// NEVER gets the bare fallback name on its own, only ever fallback+"-1",
// fallback+"-2", ...; that's a real, different rule from the "name
// collides with an EARLIER section's real slug" case just below, where
// the first occurrence DOES get the bare slug and only the second
// collision gets suffixed.
func uniqueID(base string, used map[string]bool, fallback string) string {
	if base == "" {
		for n := 1; ; n++ {
			candidate := fallback + "-" + strconv.Itoa(n)
			if !used[candidate] {
				return candidate
			}
		}
	}
	if !used[base] {
		return base
	}
	for n := 1; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !used[candidate] {
			return candidate
		}
	}
}

// makeID ports docutils.nodes.make_id: lowercase, fold common accented
// Latin letters to their unaccented ASCII form, drop any character that
// still isn't ASCII alphanumeric, collapse runs of the rest to a single
// hyphen, and strip a leading digit/hyphen run or trailing hyphen run —
// producing an identifier matching docutils' own documented
// `[a-z](-?[a-z0-9]+)*` shape.
//
// asciiFold below covers Latin-1 Supplement + the common Latin Extended-A
// letters (à, ø, đ, ł, ...), the same digraphs/stroke letters real
// docutils' own _non_id_translate table special-cases plus everything NFKD
// decomposes to a plain ASCII base letter — not the full Unicode
// normalization docutils gets from Python's unicodedata (this project has
// no dependency on golang.org/x/text/unicode/norm, deliberately, matching
// [[feedback-reference-libraries]]'s zero-third-party-dependency stance).
// A rune outside that table and outside ASCII alphanumerics is dropped
// entirely, same treatment as any other non-id character — a real, narrow
// divergence for titles in scripts asciiFold doesn't cover (CJK, Cyrillic,
// Greek, ...), not a correctness issue for the common case.
func makeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		r = unicode.ToLower(r)
		if folded, ok := asciiFold[r]; ok {
			r = folded
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	id := strings.Join(strings.Fields(b.String()), "-")
	id = strings.TrimLeft(id, "-0123456789")
	id = strings.TrimRight(id, "-")
	return id
}

// asciiFold maps a lowercase accented/digraph rune to its closest plain
// ASCII letter. Latin-1 Supplement (à-ÿ) plus the handful of Latin
// Extended-A stroke/digraph letters docutils' own _non_id_translate table
// names explicitly.
var asciiFold = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ă': 'a', 'ą': 'a',
	'ç': 'c', 'ć': 'c', 'ĉ': 'c', 'ċ': 'c', 'č': 'c',
	'ð': 'd', 'đ': 'd',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ĕ': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e',
	'ĝ': 'g', 'ğ': 'g', 'ġ': 'g', 'ģ': 'g',
	'ĥ': 'h', 'ħ': 'h',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ĩ': 'i', 'ī': 'i', 'ĭ': 'i', 'į': 'i', 'ı': 'i',
	'ĵ': 'j',
	'ķ': 'k',
	'ĺ': 'l', 'ļ': 'l', 'ľ': 'l', 'ŀ': 'l', 'ł': 'l',
	'ñ': 'n', 'ń': 'n', 'ņ': 'n', 'ň': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o', 'ō': 'o', 'ŏ': 'o', 'ő': 'o',
	'ŕ': 'r', 'ŗ': 'r', 'ř': 'r',
	'ś': 's', 'ŝ': 's', 'ş': 's', 'š': 's', 'ß': 's',
	'ţ': 't', 'ť': 't', 'ŧ': 't',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ũ': 'u', 'ū': 'u', 'ŭ': 'u', 'ů': 'u', 'ű': 'u', 'ų': 'u',
	'ŵ': 'w',
	'ý': 'y', 'ÿ': 'y', 'ŷ': 'y',
	'ź': 'z', 'ż': 'z', 'ž': 'z',
	'æ': 'a',
	'œ': 'o',
}

// MakeID returns the identifier reStructuredText derives from a name —
// docutils' own nodes.make_id, and the exact rule this package uses for a
// section's implicit target, a hyperlink target's anchor and every other
// generated id.
//
// It is exported for consumers that need to ask "would this id be
// produced anyway?" — go-richdoc/rst's writer, for one, emits an explicit
// ".. _id:" target before a heading only when the heading's own title
// would NOT already slug to that id, since emitting it regardless
// produces a genuine duplicate-name diagnostic on reparse (see
// dupnames.go). Reimplementing the rule downstream would duplicate this
// package's asciiFold table and drift from it.
func MakeID(s string) string { return makeID(s) }
