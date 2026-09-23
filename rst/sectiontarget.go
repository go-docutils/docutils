package rst

import (
	"strings"
)

// makeID ports docutils.nodes.make_id, in its own five steps and in its
// own order (nodes.py, read directly):
//
//	id = string.lower()
//	id = id.translate(_non_id_translate_digraphs)   # ß -> sz, æ -> ae, ...
//	id = id.translate(_non_id_translate)            # ø -> o, đ -> d, ...
//	id = unicodedata.normalize('NFKD', id).encode('ascii', 'ignore')...
//	id = _non_id_chars.sub('-', ' '.join(id.split()))
//	id = _non_id_at_ends.sub('', id)
//
// The order matters twice. The two translate tables run BEFORE the
// normalization because they name letters NFKD does not decompose at all
// — a stroke or a hook is part of the letter, not a combining mark — and
// the normalization runs before the hyphen substitution because
// ascii-ignore DELETES what it cannot represent, while the substitution
// REPLACES it with a hyphen. Getting those two the wrong way round is
// the whole difference between "whats-new" and "what-s-new".
//
// What this reimplemented by hand before was a single rune-to-rune fold
// covering Latin-1 and some of Latin Extended-A, with three of docutils'
// five digraphs mapped to ONE letter instead of two (ß to "s", not "sz";
// æ to "a", not "ae"; œ to "o", not "oe") and every remaining non-ASCII
// rune turned into a hyphen. See makeidfold.go for the generated answer
// that replaces it.
//
// One documented divergence remains, and it is in the lowercasing rather
// than in anything above: Go's strings.ToLower is Unicode's SIMPLE case
// mapping, Python's str.lower the FULL one, so a rune whose lowercase is
// several runes differs. Every such rune this could find either folds to
// the same ASCII either way (U+0130 gives "i" both ways, since the
// combining dot Python adds is dropped by ascii-ignore) or contributes
// no ASCII at all.
func makeID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if d, ok := nonIDTranslateDigraphs[r]; ok {
			b.WriteString(d)
			continue
		}
		if d, ok := nonIDTranslate[r]; ok {
			b.WriteString(d)
			continue
		}
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		b.WriteString(asciiFold(r))
	}
	// " ".join(id.split()) then [^a-z0-9]+ -> "-": a run of any
	// non-identifier characters, whitespace included, collapses to ONE
	// hyphen.
	var out strings.Builder
	pendingHyphen := false
	for _, r := range b.String() {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && out.Len() > 0 {
				out.WriteByte('-')
			}
			pendingHyphen = false
			out.WriteRune(r)
			continue
		}
		pendingHyphen = true
	}
	// _non_id_at_ends = '^[-0-9]+|-+$'
	id := strings.TrimLeft(out.String(), "-0123456789")
	return strings.TrimRight(id, "-")
}

// nonIDTranslate and nonIDTranslateDigraphs are docutils' own two tables,
// transcribed entry for entry. They exist because NFKD leaves these
// letters alone: a stroke, a hook or a curl belongs to the letter itself,
// so ø does not decompose to o and would otherwise be DELETED by
// ascii-ignore rather than folded.
var nonIDTranslate = map[rune]string{
	0x00f8: "o", // o with stroke
	0x0111: "d", // d with stroke
	0x0127: "h", // h with stroke
	0x0131: "i", // dotless i
	0x0142: "l", // l with stroke
	0x0167: "t", // t with stroke
	0x0180: "b", // b with stroke
	0x0183: "b", // b with topbar
	0x0188: "c", // c with hook
	0x018c: "d", // d with topbar
	0x0192: "f", // f with hook
	0x0199: "k", // k with hook
	0x019a: "l", // l with bar
	0x019e: "n", // n with long right leg
	0x01a5: "p", // p with hook
	0x01ab: "t", // t with palatal hook
	0x01ad: "t", // t with hook
	0x01b4: "y", // y with hook
	0x01b6: "z", // z with stroke
	0x01e5: "g", // g with stroke
	0x0225: "z", // z with hook
	0x0234: "l", // l with curl
	0x0235: "n", // n with curl
	0x0236: "t", // t with curl
	0x0237: "j", // dotless j
	0x023c: "c", // c with stroke
	0x023f: "s", // s with swash tail
	0x0240: "z", // z with swash tail
	0x0247: "e", // e with stroke
	0x0249: "j", // j with stroke
	0x024b: "q", // q with hook tail
	0x024d: "r", // r with stroke
	0x024f: "y", // y with stroke
}

var nonIDTranslateDigraphs = map[rune]string{
	0x00df: "sz", // ligature sz
	0x00e6: "ae", // ae
	0x0153: "oe", // ligature oe
	0x0238: "db", // db digraph
	0x0239: "qp", // qp digraph
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
