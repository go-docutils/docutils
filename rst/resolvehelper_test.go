package rst

import "github.com/go-docutils/docutils/doctree"

// parseResolvingReferences parses with Options.ResolveReferences on --
// the configuration a CONSUMER of the tree wants rather than the
// docutils-faithful default. docutils fills a reference's refuri in
// transforms.references.Hyperlinks, so a bare Parse leaves the refname
// alone and this package matches that; the tests using this helper are
// the ones whose SUBJECT is the resolution itself, so they ask for it
// directly rather than testing a shape the default never produces.
func parseResolvingReferences(src string) *doctree.Element {
	opts := DefaultOptions()
	opts.ResolveReferences = true
	return ParseWithOptions(src, opts)
}
