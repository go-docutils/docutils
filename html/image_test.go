package html_test

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/html"
	"github.com/go-docutils/docutils/rst"
)

func render(t *testing.T, src string) string {
	t.Helper()
	return html.Render(rst.Parse(src))
}

// TestAnImageReachesTheOutput pins the gap a content probe cannot see: an
// <image> has no text, so dropping it costs no character and the probe said
// 1564 of 1564 while 120 images rendered as nothing at all.
func TestAnImageReachesTheOutput(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{
			// alt DEFAULTS TO THE URI, and it is the writer that does it
			// ("alt = node.get('alt', uri)") -- the doctree carries no alt at
			// all here, matching the reference's own pseudoxml.
			name: "no :alt: falls back to the uri",
			src:  ".. image:: a.png\n",
			want: `<img alt="a.png" src="a.png" />`,
		},
		{
			name: "a unitless measure is an attribute",
			src:  ".. image:: a.png\n   :width: 200\n",
			want: `<img alt="a.png" src="a.png" width="200" />`,
		},
		{
			name: "a measure with a unit is a style declaration",
			src:  ".. image:: a.png\n   :width: 4em\n   :height: 20\n",
			want: `<img alt="a.png" height="20" src="a.png" style="width: 4em;" />`,
		},
		{
			name: ":scale: multiplies a declared measure",
			src:  ".. image:: b.png\n   :width: 50%\n   :scale: 50\n",
			want: `<img alt="b.png" src="b.png" style="width: 25%;" />`,
		},
		{
			name: ":alt: and :loading:",
			src:  ".. image:: a.png\n   :alt: Alt text\n   :loading: lazy\n",
			want: `<img alt="Alt text" loading="lazy" src="a.png" />`,
		},
	}
	for _, c := range cases {
		got := render(t, c.src)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// TestScaleAloneNeedsTheImageFile records a divergence rather than a behaviour:
// docutils reads the missing dimension out of the image FILE when :scale: is
// given and fewer than two measures are. This writer is handed a doctree and
// has no file to read, so it emits no size -- named here so the next reader
// finds a decision and not an oversight.
func TestScaleAloneNeedsTheImageFile(t *testing.T) {
	got := render(t, ".. image:: a.png\n   :scale: 50\n")
	if strings.Contains(got, "width") || strings.Contains(got, "style") {
		t.Errorf("a lone :scale: cannot produce a size without reading the file: %s", got)
	}
	if !strings.Contains(got, `src="a.png"`) {
		t.Errorf("the image itself must still be there: %s", got)
	}
}

// TestAFigureIsAFigure pins the structure read from html5_polyglot, which
// OVERRIDES _html_base here: the <figcaption> is opened by the caption or by a
// legend with no caption before it, and closed by the figure -- so ONE
// figcaption spans both. Reading _html_base alone gives <div class="figure">
// and no figcaption at all.
func TestAFigureIsAFigure(t *testing.T) {
	const src = ".. figure:: f.png\n\n   The caption.\n\n   The legend.\n"
	got := render(t, src)
	want := `<figure><img alt="f.png" src="f.png" /><figcaption><p>The caption.</p><div><p>The legend.</p></div></figcaption></figure>`
	if !strings.Contains(got, want) {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// TestAFigureWithNoCaptionHasNoFigcaption is the other branch: nothing opens a
// <figcaption>, so none appears.
func TestAFigureWithNoCaptionHasNoFigcaption(t *testing.T) {
	got := render(t, ".. figure:: f.png\n")
	if strings.Contains(got, "figcaption") {
		t.Errorf("an empty figure needs no <figcaption>: %s", got)
	}
	if !strings.Contains(got, "<figure>") {
		t.Errorf("the figure itself must be there: %s", got)
	}
}

// TestABlockDoesNotRunIntoTheNext pins the defect that made the words
// "a rubricThis is" appear in real output: an element with no case in the
// switch rendered its INLINE children bare, so the block boundary was gone
// while every character stayed in place, in order, in valid HTML.
func TestABlockDoesNotRunIntoTheNext(t *testing.T) {
	// Both neighbours have to be unwrapped for the words to touch, which is
	// what the first version of this test got wrong: a rubric followed by a
	// PARAGRAPH came out "a rubric<p>This is a paragraph.</p>" even with the
	// defect present, so the case passed either way and witnessed nothing.
	// docutils' own test-markup-rubric fixture is a run of CONSECUTIVE
	// rubrics, and that is the shape that glues.
	cases := []struct {
		name, src, glued string
	}{
		{
			"two rubrics in a row",
			".. rubric:: This is a rubric\n\n.. rubric:: This is another\n",
			"rubricThis is another",
		},
		{
			// Kept, but it does NOT discriminate: the legend's own paragraph
			// is wrapped either way, so the caption's words never touch it.
			// The positive assertions below are what witness the caption.
			"a caption and the legend after it",
			".. figure:: f.png\n\n   A caption\n\n   A legend paragraph.\n",
			"captionA legend",
		},
	}
	for _, c := range cases {
		got := render(t, c.src)
		if strings.Contains(got, c.glued) {
			t.Errorf("%s: two blocks' words ran together (%q): %s", c.name, c.glued, got)
		}
	}
	// And the positive form, which discriminates on its own: each of these
	// blocks must have an element of its own.
	for _, c := range []struct{ name, src, want string }{
		{"rubric", ".. rubric:: Rub\n", "<p>Rub</p>"},
		{"caption", ".. figure:: f.png\n\n   Cap\n", "<p>Cap</p>"},
		{"sidebar subtitle", ".. sidebar:: Side\n   :subtitle: Sub\n\n   Body.\n", "<p>Sub</p>"},
	} {
		if got := render(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: want %q in %s", c.name, c.want, got)
		}
	}
	// CONTROL: a paragraph already had a case, and passes either way. It is
	// here to show the check is not simply reporting on everything.
	if got := render(t, "One.\n\nTwo.\n"); strings.Contains(got, "One.Two.") {
		t.Errorf("control: paragraphs were never glued: %s", got)
	}
}
