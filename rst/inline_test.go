package rst

import "testing"

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
