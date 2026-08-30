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
		{"Jane <jane@example.com>", "Jane", "mailto:jane@example.com", "uri", true},
		{"no angle brackets here", "", "", "", false},
		{"missing space<https://example.com>", "", "", "", false},
		{"empty target <>", "", "", "", false},
	}
	for _, tc := range cases {
		display, target, kind, ok := splitEmbeddedLink(tc.content)
		if ok != tc.wantOK || display != tc.wantDisplay || target != tc.wantTarget || kind != tc.wantKind {
			t.Errorf("splitEmbeddedLink(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				tc.content, display, target, kind, ok,
				tc.wantDisplay, tc.wantTarget, tc.wantKind, tc.wantOK)
		}
	}
}
