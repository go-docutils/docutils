// Copyright (c) the go-docutils authors.
// SPDX-License-Identifier: BSD-3-Clause

package rst

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-docutils/docutils/doctree"
)

// TestLineLengthLimitRefusesTheDocument pins the reference's own
// denial-of-service guard: rst.Parser.parse checks every line BEFORE parsing and,
// on the first one over the limit, appends `Line N exceeds the line-length-limit.`
// as an ERROR and parses nothing at all. The wording and the refusal are both the
// reference's, checked against it for the same input.
func TestLineLengthLimitRefusesTheDocument(t *testing.T) {
	src := "ok line\n" + strings.Repeat("a", 10001) + "\n\nA paragraph that will never be parsed.\n"
	got := doctree.Dump(Parse(src))
	if !strings.Contains(got, "Line 2 exceeds the line-length-limit.") {
		t.Errorf("want the reference's own message in:\n%s", got)
	}
	if !strings.Contains(got, `level="3"`) {
		t.Errorf("the refusal should be an ERROR:\n%s", got)
	}
	if strings.Contains(got, "never be parsed") {
		t.Errorf("the document should not have been parsed at all:\n%s", got)
	}
}

// TestLineLengthLimitZeroMeansNoLimit pins the zero value, which matters because a
// caller using Options{} directly must not find every line refused.
func TestLineLengthLimitZeroMeansNoLimit(t *testing.T) {
	src := strings.Repeat("a", 50_000) + "\n"
	got := doctree.Dump(ParseWithOptions(src, Options{}))
	if strings.Contains(got, "line-length-limit") {
		t.Errorf("Options{} should impose no limit:\n%s", got[:200])
	}
	if n := DefaultOptions().LineLengthLimit; n != 10000 {
		t.Errorf("DefaultOptions().LineLengthLimit = %d, want the reference's 10000", n)
	}
}

// TestIncludeRespectsTheLineLengthLimit pins the other half, which the reference
// applies to the INCLUDED lines and answers with a warning naming the file --
// refusing the inclusion rather than the document.
func TestIncludeRespectsTheLineLengthLimit(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"long.txt": "short\n" + strings.Repeat("b", 10001) + "\n",
	})
	got := parseWithSource(t, dir, "main.rst", ".. include:: long.txt\n")
	if !strings.Contains(got, "exceeds the line-length-limit") {
		t.Errorf("want the limit to refuse the inclusion:\n%s", got)
	}
	if !strings.Contains(got, `level="2"`) {
		t.Errorf("the reference makes this a WARNING, not an error:\n%s", got)
	}
	if strings.Contains(got, "short") {
		t.Errorf("nothing of the file should have been spliced:\n%s", got)
	}
}

// TestIncludeRootPrefixConfinesAnAbsolutePath pins the confinement knob a caller
// needs to parse untrusted reST with a SourcePath set: docutils' own root_prefix,
// "Base directory for absolute paths when reading from the local filesystem".
func TestIncludeRootPrefixConfinesAnAbsolutePath(t *testing.T) {
	// An absolute path OUTSIDE the document's tree, created for this test and
	// removed after it, so the case is precise without reading a system file.
	outsideDir, err := os.MkdirTemp("", "rootprefix-outside-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(outsideDir) })
	outside := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outside, []byte("THE REAL FILE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The same absolute path, reproduced UNDER the jail: that is what root_prefix
	// means -- "/a/b" read as "<prefix>/a/b".
	dir := writeTree(t, map[string]string{
		filepath.Join("jail", strings.TrimPrefix(outside, string(filepath.Separator))): "THE CONFINED FILE\n",
	})

	opts := DefaultOptions()
	opts.SourcePath = filepath.Join(dir, "main.rst")
	opts.IncludeRootPrefix = filepath.Join(dir, "jail")
	got := doctree.Dump(ParseWithOptions(".. include:: "+outside+"\n", opts))
	if !strings.Contains(got, "THE CONFINED FILE") {
		t.Errorf("the absolute path was not resolved under the prefix:\n%s", got)
	}
	if strings.Contains(got, "THE REAL FILE") {
		t.Errorf("the absolute path escaped the prefix:\n%s", got)
	}

	// CONTROL: without the prefix the same document reads the real path, which is
	// what the reference does too -- and is why the knob exists. Without this the
	// test would pass just as well if absolute includes never worked at all.
	opts.IncludeRootPrefix = ""
	got = doctree.Dump(ParseWithOptions(".. include:: "+outside+"\n", opts))
	if !strings.Contains(got, "THE REAL FILE") {
		t.Errorf("without a prefix an absolute include should read the real path:\n%s", got)
	}
}

// TestADuplicateNameIsNotQuadratic is the regression test for a complexity attack
// found by this audit, and it is written as a RATIO rather than a wall-clock budget
// so it does not fail on a loaded machine.
//
// 20000 targets sharing one name took 14 SECONDS for 340 KB, while 20000 distinct
// ones took 26ms: three O(n) searches per duplicate message (the inline host, the
// top-level ancestor, and a slice insertion that copied the whole tail), plus an id
// allocator that rescanned "name-1", "name-2", … from 1 for every collision. All
// four are gone; the same document is now ~70ms.
func TestADuplicateNameIsNotQuadratic(t *testing.T) {
	dup := func(n int) time.Duration {
		src := strings.Repeat(".. _t: http://x/\n", n)
		start := time.Now()
		Parse(src)
		return time.Since(start)
	}
	// Warm up, so the first allocation of the parser's maps is not counted.
	dup(200)
	small, large := dup(2000), dup(8000)
	// Four times the input. Linear would be ~4x; the quadratic version was ~16x.
	// 8 leaves room for noise and for the genuinely superlinear parts of parsing
	// while still failing loudly if the O(n^2) behaviour comes back.
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("4x the duplicates cost %.1fx the time (%s -> %s): the quadratic path is back",
			ratio, small, large)
	}
}
