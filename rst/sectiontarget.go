package rst

import (
	"strings"
	"unicode"
)

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
