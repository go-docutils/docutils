package rst

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func doctreeDump(src string) string { return doctree.Dump(Parse(src)) }

func TestSplitEmbeddedLink(t *testing.T) {
	cases := []struct {
		content     string
		wantDisplay string
		wantTarget  string
		wantKind    string
		wantOK      bool
	}{
		{"Python <https://python.org>", "Python", "https://python.org", "uri", true},
		{"alias <target_>", "alias", "target", "name", true},
		{"Jane <jane@example.com>", "Jane", "jane@example.com", "uri", true},
		{"no angle brackets here", "", "", "", false},
		{"missing space<https://example.com>", "", "", "", false},
		{"empty target <>", "", "", "", false},
	}
	for _, tc := range cases {
		display, kind, targetRunes, ok := splitEmbeddedLink(escapeBackslashes(tc.content))
		target := string(targetRunes)
		if ok != tc.wantOK || display != tc.wantDisplay || target != tc.wantTarget || kind != tc.wantKind {
			t.Errorf("splitEmbeddedLink(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				tc.content, display, target, kind, ok,
				tc.wantDisplay, tc.wantTarget, tc.wantKind, tc.wantOK)
		}
	}
}

func TestJoinEmbeddedURI(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"http://example.com/long/path", "http://example.com/long/path"},
		{"http://example.com/\nlong/path", "http://example.com/long/path"},
		{"http://example.com/\nlong/path /and  /whitespace", "http://example.com/long/path/and/whitespace"},
		{"http://example.com/a\\ long/path\\ and/some\\ escaped\\ whitespace", "http://example.com/a long/path and/some escaped whitespace"},
	}
	for _, tc := range cases {
		got := joinEmbeddedURI(escapeBackslashes(tc.content))
		if got != tc.want {
			t.Errorf("joinEmbeddedURI(%q) = %q, want %q", tc.content, got, tc.want)
		}
	}
}

func TestAdjustEmbeddedURI(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"jane@example.com", "mailto:jane@example.com"},
	}
	for _, tc := range cases {
		if got := adjustEmbeddedURI(tc.uri); got != tc.want {
			t.Errorf("adjustEmbeddedURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

// TestInlineLiteralBackslashes pins the one place docutils' end-string
// lookbehind deliberately DIFFERS between markers. As built in states.py:
//
//	emphasis: (?<![\s\x00])(\*)($|(?=...))
//	literal:  (?<!\s)(``)($|(?=...))
//
// Only emphasis (and every other marker) refuses a delimiter carrying the
// \x00 escape marker; the literal form has no \x00 in its lookbehind at
// all. That is the spec's "backslashes are not escapes inside inline
// literals" made mechanical -- a backslash neither protects the closing
// backquotes nor disappears from the content. Every case here was run
// against the reference implementation.
func TestInlineLiteralBackslashes(t *testing.T) {
	cases := []struct{ source, want string }{
		// The backslash does NOT protect the close; it stays as content.
		{"``literal\\``\n", "<document>\n    <paragraph>\n        <literal>\n            literal\\\n"},
		// An escaped backquote mid-content is kept verbatim, backslash and
		// all, and the real close is still found afterwards.
		{"``a\\`b``\n", "<document>\n    <paragraph>\n        <literal>\n            a\\`b\n"},
		// Two backslashes stay two backslashes -- no collapsing either.
		{"``a\\\\``\n", "<document>\n    <paragraph>\n        <literal>\n            a\\\\\n"},
		// The end-boundary rule still applies after the close.
		{"``a\\`` b\n", "<document>\n    <paragraph>\n        <literal>\n            a\\\n         b\n"},
		// A LONE backslash is the whole content (v0.102.0+). The escaped
		// backquote lends its backquote to the end string while the
		// marker stays content, so the close lands at the very first
		// content position -- which the "empty content" guard used to
		// reject on position alone, making this a <problematic>. It is
		// what PEP 12 writes when documenting line continuations.
		{"``\\``\n", "<document>\n    <paragraph>\n        <literal>\n            \\\n"},
		// And the guard that must SURVIVE that: four backquotes really
		// are an empty literal, and docutils rejects them. Surrounded by
		// text, because four identical characters ALONE on a line are a
		// <transition> and never reach inline parsing at all -- which is
		// how the first draft of this case tested the wrong layer.
		{"x ```` y\n", "<document>\n    <paragraph>\n        x \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            ``\n        `` y\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline literal start-string without end-string.\n"},
		// Six give a literal whose content is two backquotes.
		{"x `````` y\n", "<document>\n    <paragraph>\n        x \n        <literal>\n            ``\n         y\n"},
		// The CONTRAST case: emphasis does honor the escape, so this is
		// one <emphasis> spanning the escaped asterisk, not two.
		{"*a\\*b*\n", "<document>\n    <paragraph>\n        <emphasis>\n            a*b\n"},
	}
	for _, tc := range cases {
		if got := doctreeDump(tc.source); got != tc.want {
			t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
		}
	}
}

// TestPipesAreNotSubstitutionStarts covers docutils' substitution
// start-string `\|(?!\|)`: a "|" followed by another "|" is not a
// start-string at all. Without the negative lookahead this opened a
// substitution reference at the first "|" of the "||" pair and swallowed
// the rest of the line.
func TestPipesAreNotSubstitutionStarts(t *testing.T) {
	src := "first | then || and finally |||\n"
	want := "<document>\n    <paragraph>\n        first | then || and finally |||\n"
	if got := doctreeDump(src); got != want {
		t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", src, got, want)
	}
}

// TestDumpDoesNotEscapeQuotesInAttributes pins Dump to docutils'
// nodes.pseudo_quoteattr, which is literally `'"%s"' % value` -- it
// escapes nothing, producing technically-invalid XML for a value holding
// a quote and not caring, because pseudoxml is a debug format.
func TestDumpDoesNotEscapeQuotesInAttributes(t *testing.T) {
	got := doctreeDump("_`\"target2\"` with quotes\n")
	if !strings.Contains(got, `name=""target2""`) {
		t.Errorf("attribute quote was escaped rather than passed through:\n%s", got)
	}
}

// refuris returns every refuri in the parsed tree, in document order.
func refuris(t *testing.T, source string) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(doctree.Dump(Parse(source)), "\n") {
		if i := strings.Index(line, `refuri="`); i >= 0 {
			rest := line[i+len(`refuri="`):]
			out = append(out, rest[:strings.IndexByte(rest, '"')])
		}
	}
	return out
}

// TestEmailStartBoundary covers docutils' emailc class, transcribed.
// v0.87.0 did the END boundary of a standalone email address and left
// the start; these are the cases that separate the two.
//
// Every expectation was produced by running real docutils, not by
// reading its regular expression: the "café" case in particular is one
// I would have got wrong by reasoning (no address at all, not a
// shortened one, because no position inside a word is a valid start).
func TestEmailStartBoundary(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			// emailc contains "/", so the path-looking prefix is part
			// of the LOCAL PART. PEP 20 writes exactly this.
			"a slash is an email character",
			"posted to comp.lang.python/python-list@python.org under a\n",
			[]string{"mailto:comp.lang.python/python-list@python.org"},
		},
		{"a short slash case", "mail a/b@example.com here\n", []string{"mailto:a/b@example.com"}},
		{
			// emailc is a-zA-Z0-9, not unicode.IsLetter. "caf" then
			// stops at "é", which is not "@", and no later position is
			// a valid start boundary -- so there is no address here at
			// all, not a shorter one.
			"a non-ASCII letter is not an email character",
			"mail café@example.com here\n", nil,
		},
		// "." is a SEPARATOR between runs of emailc, not a member:
		// emailc+(\.emailc+)* has no room for a leading, trailing or
		// doubled dot.
		{"a dot separates runs", "mail x.y@example.com here\n", []string{"mailto:x.y@example.com"}},
		{"a leading dot is not an address", "mail .lead@example.com here\n", nil},
		{"a doubled dot is not an address", "mail a..b@example.com here\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refuris(t, tc.source); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("refuris = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUnknownSchemeRefusesTheWholeRun covers what docutils does AFTER
// its pattern matches and standalone_uri refuses the scheme: it raises
// MarkupMismatch, and implicit_inline abandons the entire text it was
// given ("except MarkupMismatch: pass", then "return [nodes.Text(text)]").
//
// Two things follow, and this package had neither. No email may be
// found INSIDE the refused URI -- a per-position scanner happily reads
// "svn+ssh://pythondev@svn.python.org/" as a mailto: address, which is
// a link real docutils does not make. And a perfectly good URI AFTER it
// is not recognised either, until an explicit construct starts a fresh
// run.
func TestUnknownSchemeRefusesTheWholeRun(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{"no email is invented inside it", "see svn+ssh://x@y.org/ here\n", nil},
		{"nor a later email", "see svn+ssh://x@y.org/ and plain a@b.org here\n", nil},
		{"nor a later URI", "bad svn+ssh://x@y.org/ then http://real.com end\n", nil},
		{"a newline does not end the run", "bad svn+ssh://x@y.org/ then\nhttp://real.com next\n", nil},
		// The resets, each verified against real docutils.
		{"emphasis starts a fresh run", "bad svn+ssh://x@y.org/ then *em* then http://real.com end\n", []string{"http://real.com"}},
		{"an inline literal does too", "bad svn+ssh://x@y.org/ then ``lit`` then http://real.com end\n", []string{"http://real.com"}},
		{"and so does a new paragraph", "bad svn+ssh://x@y.org/ end\n\nnew http://real.com end\n", []string{"http://real.com"}},
		// A URI BEFORE the refused one already matched, so it survives.
		{"an earlier URI is kept", "see http://real.com and svn+ssh://x@y.org/ here\n", []string{"http://real.com"}},
		// The controls. The scheme test must happen only once the whole
		// absolute-URI shape has matched: "punctuation:" is letters and
		// a colon, and is not a URI, because what follows it cannot end
		// on a URI-final character. Testing the scheme at the colon
		// refused this whole paragraph.
		{
			"a word before a colon is not a refused scheme",
			"Trailing punctuation: https://x.org, and https://y.org.\n",
			[]string{"https://x.org", "https://y.org"},
		},
		{"a known scheme is untouched", "see http://real.com and b@c.org here\n", []string{"http://real.com", "mailto:b@c.org"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refuris(t, tc.source); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("refuris = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestImplicitMatchStopsAtExplicitMarkup pins the bound implicitLimit
// applies. docutils gets it structurally: Inliner.parse finds explicit
// start-strings over the whole string first and hands implicit_inline
// only the text between them, so an implicit match cannot cross one.
//
// This case is the one that proved it matters. Transcribing emailc
// faithfully brought "`" in with it, and "non-“@overload“-decorated"
// -- PEP 484, which the corpus caught within the same round -- became a
// single mailto: address swallowing the inline literal whole.
func TestImplicitMatchStopsAtExplicitMarkup(t *testing.T) {
	const source = "text non-``@overload``-decorated more\n"
	if got := refuris(t, source); got != nil {
		t.Errorf("refuris = %v, want none: an implicit match crossed an inline literal", got)
	}
	got := doctree.Dump(Parse(source))
	if !strings.Contains(got, "<literal>") {
		t.Errorf("the inline literal did not survive:\n%s", got)
	}
}

// TestReferenceEndBoundary covers the character that may follow a bare
// reference's trailing "_". docutils tests it against end_string_suffix,
// a NAMED class (whitespace, NUL, backslash, the closers and the
// delimiters); this package tested unicode.IsPunct, a Unicode CATEGORY.
//
// They are not the same set, and not even nested: of 39 characters run
// through real docutils, twelve disagreed, in both directions. The same
// mistake was made and corrected twice before in this file, for the URI
// scan (v0.63.0) and the email scan (v0.87.0); this was the third
// sibling and the last one still ad hoc.
//
// Every expectation below is what real docutils produced for
// "see name_<c> rest", not what the class membership suggests.
func TestReferenceEndBoundary(t *testing.T) {
	check := func(t *testing.T, c string, want bool) {
		t.Helper()
		got := strings.Contains(doctree.Dump(Parse("see name_"+c+" rest\n")), `refname="name"`)
		if got != want {
			t.Errorf("after %q: reference recognised = %v, want %v", c, got, want)
		}
	}
	t.Run("accepted", func(t *testing.T) {
		// Whitespace and the delimiters/closers of end_string_suffix.
		for _, c := range []string{" ", ".", ",", ";", "!", "?", "-", "/", ":", ")", "]", "}", "\"", "'", "’", "”", "»"} {
			check(t, c, true)
		}
		// ">" is a math SYMBOL and "\\" is not punctuation at all, so an
		// IsPunct test rejected both. docutils accepts them.
		check(t, ">", true)
		check(t, "\\", true)
	})
	t.Run("rejected", func(t *testing.T) {
		// All Unicode punctuation, none of them in end_string_suffix.
		// "*" is the one the corpus found: "bdist_* to stdlib" is a
		// plain sentence, not a reference to "bdist".
		for _, c := range []string{"*", "(", "[", "{", "&", "%", "#", "@", "§", "¶", "_"} {
			check(t, c, false)
		}
	})
	t.Run("the corpus cases", func(t *testing.T) {
		for _, src := range []string{
			"Joe Smith, bdist_* to stdlib?\n",
			"interned: interned-state (SSTATE_*) as in 3.2\n",
		} {
			got := doctree.Dump(Parse(src))
			if strings.Contains(got, "<reference") {
				t.Errorf("%q produced a reference:\n%s", src, got)
			}
			// The underscore and what follows must still be in the TEXT,
			// not consumed by a reference that was never there.
			if !strings.Contains(got, "_*") {
				t.Errorf("%q lost its \"_*\" from the text:\n%s", src, got)
			}
		}
	})
}

// TestProblematicQuotesTheSourceAsWritten covers the text a
// <problematic> node shows. docutils builds it from the RAWSOURCE with
// unescape(..., restore_backslashes=True), so it reads exactly as the
// author typed it -- ":file:`PC\\\\python_uwp.cpp`" keeps BOTH
// backslashes.
//
// Three sites passed the runes still carrying escapeBackslashes'
// private-use encoding, so U+F005C -- a shifted backslash, not a
// character any document contains -- went straight into the tree. PEP
// 773 came out with one. tryURIScheme carries the same note from
// v0.31.0 for the standalone-URI path; these were the sites it did not
// cover.
func TestProblematicQuotesTheSourceAsWritten(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"an escaped backslash stays escaped", "see :file:`PC\\\\python_uwp.cpp` here\n", ":file:`PC\\\\python_uwp.cpp`"},
		{"a lone backslash stays", "see :file:`PC\\python_uwp.cpp` here\n", ":file:`PC\\python_uwp.cpp`"},
		{"plain text, the control", "see :file:`PCpython` here\n", ":file:`PCpython`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, got)
			}
			// No private-use rune may ever reach the tree: that encoding
			// is internal and a document cannot contain one.
			for _, r := range got {
				if r >= 0xF0000 {
					t.Fatalf("private-use rune U+%04X leaked into the tree:\n%q", r, got)
				}
			}
		})
	}
}

// TestSchemeWithEmptyPathIsAURI covers a URI whose path is empty. The
// hierarchical slashes are PART of it -- "urilast" includes "/" -- so
// "file://" ends on a valid final character and is a reference, while
// "mailto:" and "news:", a scheme with no slash and nothing after, are
// not. This skipped PAST the slashes before measuring and made all four
// plain text.
func TestSchemeWithEmptyPathIsAURI(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"see file:// here\n", "file://"},
		{"see http:// here\n", "http://"},
		{"see file:/ here\n", "file:/"},
		{"see file:///tmp here\n", "file:///tmp"},
		{"see http://x.com/a here\n", "http://x.com/a"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); !strings.Contains(got, `refuri="`+tc.want+`"`) {
				t.Errorf("want refuri %q in:\n%s", tc.want, got)
			}
		})
	}
	// The controls: no slash, nothing after -- not a URI.
	for _, src := range []string{"see mailto: here\n", "see news: here\n"} {
		if got := doctree.Dump(Parse(src)); strings.Contains(got, "refuri=") {
			t.Errorf("%q became a reference:\n%s", src, got)
		}
	}
}

// TestEmbeddedURIGetsMailtoOnlyWhenItIsAnAddress covers adjust_uri,
// which prefixes "mailto:" only when the WHOLE target matches docutils'
// email pattern, anchored with "$".
//
// The heuristic this replaced -- "contains @, does not contain ://" --
// was wrong in both directions. It prefixed "mailto:core@pytest.org"
// AGAIN, so pytest's own contact page pointed at
// "mailto:mailto:core@pytest.org"; and it prefixed "a@b", which
// docutils leaves alone because the host half needs two characters.
//
// The grammar answers both without a special case: emailc excludes
// ":", so "mailto:core" cannot be a local part. Every expectation here
// came from running Inliner.adjust_uri.
func TestEmbeddedURIGetsMailtoOnlyWhenItIsAnAddress(t *testing.T) {
	for _, tc := range []struct{ uri, want string }{
		{"core@pytest.org", "mailto:core@pytest.org"},
		{"a@b.org", "mailto:a@b.org"},
		{"user@host", "mailto:user@host"},
		{"a/b@c.org", "mailto:a/b@c.org"},
		{"x@y.org?subject=hi", "mailto:x@y.org?subject=hi"},
		// Already a mailto: the one that was doubled.
		{"mailto:core@pytest.org", "mailto:core@pytest.org"},
		// A host of one character is not an address.
		{"a@b", "a@b"},
		// Schemes, with and without an "@" in them.
		{"http://e.com", "http://e.com"},
		{"news:comp.lang", "news:comp.lang"},
		{"http://x.com/a@b", "http://x.com/a@b"},
		{"ftp://u@h/p", "ftp://u@h/p"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			got := doctree.Dump(Parse("see `x <" + tc.uri + ">`_ here\n"))
			if !strings.Contains(got, `refuri="`+tc.want+`"`) {
				t.Errorf("want refuri %q in:\n%s", tc.want, got)
			}
		})
	}
}
