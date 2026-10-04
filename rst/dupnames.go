package rst

import (
	"fmt"

	"github.com/go-docutils/docutils/doctree"
)

// Duplicate reference names, ported from docutils' own
// document.note_names / document.set_duplicate_name (nodes.py, read
// directly). Two elements may legitimately carry the same reference name;
// what happens then depends on whether each is an EXPLICIT target (a
// ".. _name:" block target, an inline "_`name`" target, a footnote, a
// citation, a directive's own :name: option) or an IMPLICIT one (a section
// title, or the target a phrase reference with an embedded URI leaves
// behind). set_duplicate_name's own docstring gives the whole thing as a
// state-transition table, transcribed here:
//
//	Input     Old State       New State       Action
//	--------  --------------  --------------  ------------------------
//	type      name  type      name  type      invalidate       report
//	explicit  old   explicit  None  explicit  new,old          WARNING
//	implicit  old   explicit  old   explicit  new              INFO
//	explicit  old   implicit  new   explicit  old              INFO
//	implicit  old   implicit  None  implicit  new,old          INFO
//	explicit  None  explicit  None  explicit  new              WARNING
//	implicit  None  explicit  None  explicit  new              INFO
//	explicit  None  implicit  new   explicit
//	implicit  None  implicit  None  implicit  new              INFO
//
// "Invalidating" an element moves the name from its "name" attribute to
// "dupname" — the element keeps its identity and its id, but stops being
// something a reference can resolve to. A "None" name means the name is
// now ONLY a dupname: every element that claimed it has been invalidated.
//
// The one case ahead of the table: when the new and old element name the
// same destination (identical refuri or refname), only the new one is
// invalidated and the message is about an external target instead.

// registersName reports whether an element carrying a "name" attribute is
// claiming that name as a TARGET — the only thing this table is about.
// A reference's own "name" is not a claim: docutils keeps it as display
// text (whitespace_normalize_name) and registers the name it POINTS AT in
// document.refnames via note_refname, a different map entirely. Treating
// a reference as a claimant made every "_`term` ... `term`_" pair collide
// with itself. A substitution definition is excluded too, for the
// opposite reason: it has its own duplicate rule in Body.substitution_def
// (where the LATER definition wins and the earlier one is invalidated),
// not this one.
//
// <directive> is excluded for a third reason: docutils has no such node at
// all. It is this package's own placeholder for a directive it does not
// implement (ReportUnknownDirectives=false, see parseDirective), and its
// "name" attribute holds the DIRECTIVE's own name -- "deprecated",
// "versionadded", "toctree" -- not a target it claims. Letting it in made
// any document that both uses such a directive and has a target or section
// named the same word collide with itself: the directive lost its name to
// dupname (a consumer reconstructing the source then wrote ".. :: 9.1",
// which reads back as a COMMENT), a "Duplicate explicit target name" WARNING
// was fabricated, and the real target was invalidated so every reference to
// it dangled. Three wrong things from one overloaded attribute.
//
// Neither corpus can see it, because both run with the default options,
// where an unimplemented directive is a diagnostic and no <directive> node
// is built. The witness is go-richdoc/rst, which turns that option OFF --
// sphinx's ".. deprecated::" plus a "Deprecated" section is an ordinary
// shape in its corpus.
func registersName(tag string) bool {
	switch tag {
	case doctree.TagReference, doctree.TagFootnoteReference,
		doctree.TagCitationReference, doctree.TagSubstitutionRef,
		doctree.TagSubstitutionDef, doctree.TagDirective:
		return false
	}
	return true
}

// noteNameLine remembers the source line an inline-created target was
// built on. The duplicate-name pass runs AFTER parsing, where
// p.currentLine is long gone, but docutils reports these diagnostics
// against the line the duplicate name appeared on — so the line has to be
// captured here, while it is still known. Kept in a side map rather than
// an attribute so it never reaches the tree, the same reason
// implicitTargets is one.
func (p *parser) noteNameLine(el *doctree.Element) {
	// The STATE MACHINE's line, not the Inliner's: set_duplicate_name
	// passes a base_node that is not yet attached and carries no line of
	// its own, so Reporter.system_message falls back to
	// StateMachine.get_source_and_line() -- see parser.currentSMLine.
	line := p.currentSMLine
	if line == 0 {
		line = p.currentLine
	}
	if line == 0 {
		return
	}
	if p.nameLines == nil {
		p.nameLines = map[*doctree.Element]int{}
	}
	p.nameLines[el] = line
}

// nameEntry is one row of docutils' document.names + document.nametypes.
// node == nil means the name has been invalidated for everyone (names[x]
// is None), which is NOT the same as the name being unseen.
type nameEntry struct {
	node     *doctree.Element
	explicit bool
}

// resolveDuplicateNames applies the transition table above in document
// order — the same order docutils' note_*_target calls happen in during
// parsing. It must run AFTER assignSectionTargets (so section names
// exist) and BEFORE resolveTargets (which resolves references against
// names this pass may invalidate).
func (p *parser) resolveDuplicateNames(doc *doctree.Element) {
	names := map[string]*nameEntry{}
	p.walkNames(doc, doc, nil, nil, names)
	// One rebuild per parent, after the walk, instead of one slice insertion per
	// message during it. Inserting immediately copies the whole tail of the
	// children slice every time, which is O(n) allocation per message and so
	// O(n^2) bytes for a document whose names collide -- the GC time dominated
	// the profile once the id counter was fixed.
	p.flushQueuedMessages()
	p.resolveDuplicateSubstitutions(doc, doc)
	p.flushQueuedMessages()
}

// resolveDuplicateSubstitutions is document.note_substitution_def's own
// rule, which runs BACKWARDS compared with every other duplicate here:
// "keep only the last definition", so it is the OLD node that gets
// dupname and the NEW one that keeps its name. The ERROR lands where the
// duplicate was found, between the two definitions.
func (p *parser) resolveDuplicateSubstitutions(el, body *doctree.Element) {
	seen := map[string]*doctree.Element{}
	var walk func(cur, body *doctree.Element)
	walk = func(cur, body *doctree.Element) {
		childBody := body
		if admitsBodyElements(cur.Tag) {
			childBody = cur
		}
		for _, c := range cur.Children {
			ce, ok := c.(*doctree.Element)
			if !ok {
				continue
			}
			if ce.Tag == doctree.TagSubstitutionDef {
				if name := ce.Attr("name"); name != "" {
					if old, dup := seen[name]; dup {
						dupname(old, name)
						msg := sectionMessage("3", "ERROR",
							`Duplicate substitution definition name: "`+name+`".`,
							p.nameLines[ce], "")
						p.queueMessage(childBody, topLevelAncestor(childBody, ce), msg)
					}
					seen[name] = ce
				}
			}
			walk(ce, childBody)
		}
	}
	walk(el, body)
}

// walkNames recurses in document order. body is the nearest ancestor that
// can hold body elements — where a diagnostic about one of its
// descendants belongs, and the reason a duplicate inside a <line> is
// reported nowhere at all (see emitDuplicateMessage).
// walkNames carries the two positions a duplicate message needs -- el's own
// PARENT, and the child of body on the path down to it -- rather than searching
// for them afterwards.
//
// They used to be found by inlineHost and topLevelAncestor, each of which walks
// the whole body per duplicate. That is O(n) per message and so O(n^2) for a
// document whose names collide: 20000 targets sharing one name took 14 SECONDS for
// 340 KB, while 20000 DISTINCT ones took 26ms. A complexity attack needs no
// special construct, just a repeated name -- found by a security audit's DoS probe,
// not by the corpus, where no file collides at that scale.
func (p *parser) walkNames(el, body, parent, topLevel *doctree.Element, names map[string]*nameEntry) {
	if name := el.Attr("name"); name != "" && registersName(el.Tag) {
		p.noteName(el, name, body, parent, topLevel, names)
	}
	childBody := body
	if admitsBodyElements(el.Tag) {
		childBody = el
	}
	for _, c := range el.Children {
		ce, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		// The top-level ancestor is the child of the BODY, so it resets every
		// time the body does and is otherwise carried down unchanged.
		childTop := topLevel
		if childBody == el {
			childTop = ce
		}
		p.walkNames(ce, childBody, el, childTop, names)
	}
}

func (p *parser) noteName(el *doctree.Element, name string, body, parent, topLevel *doctree.Element, names map[string]*nameEntry) {
	explicit := !p.isImplicitTarget(el)
	entry, seen := names[name]
	if !seen {
		names[name] = &nameEntry{node: el, explicit: explicit}
		return
	}
	if entry.node == el {
		return
	}
	p.setDuplicateName(el, name, body, parent, topLevel, explicit, entry)
}

func (p *parser) setDuplicateName(el *doctree.Element, name string, body, parent, topLevel *doctree.Element, explicit bool, entry *nameEntry) {
	old, oldExplicit := entry.node, entry.explicit
	entry.explicit = oldExplicit || explicit

	// Every message below quotes the name with LITERAL quote characters,
	// not with %q. The two look identical until a name contains a quote
	// of its own: docutils writes f'Duplicate name "{name}" ...', a
	// plain interpolation, while %q is strconv.Quote and ESCAPES what it
	// interpolates. A PEP linking to `Python issue #16500 "Add an atfork
	// module"` twice came out with \" inside the message.
	level, msgType, text := 0, "", ""
	switch {
	case old != nil && sameDestination(el, old):
		// Two references to the SAME destination: keep the old target and
		// invalidate only the new one.
		ref := el.Attr("refuri")
		if ref == "" {
			ref = el.Attr("refname")
		}
		level, msgType = 1, "INFO"
		text = fmt.Sprintf("Duplicate name \"%s\" for external target \"%s\".", name, ref)
		dupname(el, name)
	case explicit:
		if oldExplicit {
			level, msgType = 2, "WARNING"
			text = fmt.Sprintf("Duplicate explicit target name: \"%s\".", name)
			dupname(el, name)
			if old != nil {
				dupname(old, name)
				entry.node = nil
			}
		} else {
			// An explicit target OVERRIDES an implicit one of the same
			// name rather than colliding with it.
			entry.node = el
			if old != nil {
				level, msgType = 1, "INFO"
				text = fmt.Sprintf("Target name overrides implicit target name \"%s\".", name)
				dupname(old, name)
			}
		}
	default:
		level, msgType = 1, "INFO"
		text = fmt.Sprintf("Duplicate implicit target name: \"%s\".", name)
		dupname(el, name)
		if old != nil && !oldExplicit {
			dupname(old, name)
			entry.node = nil
		}
	}
	if level != 0 {
		p.emitDuplicateMessage(el, body, parent, topLevel, level, msgType, text)
	}
}

// dupname moves a name from an element's "name" attribute to "dupname",
// docutils' nodes.dupname. The element keeps its id: it is still a place
// in the document, just no longer one a reference can resolve to.
func dupname(el *doctree.Element, name string) {
	if el.Attr("name") == name {
		delete(el.Attrs, "name")
	}
	el.SetAttr("dupname", name)
}

// sameDestination reports whether two named elements point at the same
// place — docutils checks refname against refname and refuri against
// refuri, never one against the other.
func sameDestination(a, b *doctree.Element) bool {
	if r := a.Attr("refname"); r != "" && r == b.Attr("refname") {
		return true
	}
	if r := a.Attr("refuri"); r != "" && r == b.Attr("refuri") {
		return true
	}
	return false
}

// isImplicitTarget reports whether el was registered with
// note_implicit_target rather than note_explicit_target. Sections (named
// after their own title) are implicit by construction; among <target>s
// only the one a phrase reference with an embedded URI leaves behind is,
// which referenceOrPhrase records as it builds it. Everything else that
// carries a name — a block target, an inline "_`name`" target, a
// footnote, a citation, a directive's :name: — is explicit.
func (p *parser) isImplicitTarget(el *doctree.Element) bool {
	if el.Tag == doctree.TagSection {
		return true
	}
	return p.implicitTargets[el]
}

// emitDuplicateMessage places the diagnostic the way docutils does, which
// is the subtle part. set_duplicate_name ends with:
//
//	if msgnode is not None and 'Body' in repr(msgnode.content_model):
//	    msgnode += msg
//
// so a message whose msgnode cannot hold body elements is BUILT AND THEN
// DROPPED, never reaching the tree — which is exactly why two duplicate
// embedded-URI targets inside a line block produce no diagnostic at all
// ("System messages are no longer inserted between <line>s", the
// corpus fixture's own title). Where it IS kept, msgnode is the body
// element the state machine is currently filling, and the paragraph
// holding the duplicate has not been appended to it yet — so the message
// lands as a SIBLING immediately BEFORE that paragraph, not inside it.
func (p *parser) emitDuplicateMessage(el, body, parent, topLevel *doctree.Element, level int, msgType, text string) {
	if body == nil {
		return
	}
	// An empty <target> renders nothing, so docutils gives it no backref
	// id to point at; anything else gets one (set_id).
	backref := ""
	if !(el.Tag == doctree.TagTarget && len(el.Children) == 0) {
		backref = el.Attr("id")
		if backref == "" {
			backref = p.autoID(el.Tag)
			el.SetAttr("id", backref)
		}
	}
	msg := doctree.NewElement(doctree.TagSystemMessage,
		doctree.NewElement(doctree.TagParagraph, &doctree.Text{Data: text}))
	msg.SetAttr("level", fmt.Sprintf("%d", level))
	msg.SetAttr("type", msgType)
	if line := p.nameLines[el]; line != 0 {
		msg.SetAttr("line", fmt.Sprintf("%d", line))
	}
	if backref != "" {
		msg.SetAttr("backref", backref)
	}
	// Where the message lands follows from WHEN docutils registers the
	// name, since it appends to whatever body element is being filled at
	// that moment:
	//
	//   - a SECTION's implicit name is registered as the section is
	//     created, when it holds nothing but its own <title> — so the
	//     message goes INSIDE the section, right after that title;
	//   - every other name here is registered during INLINE parsing, when
	//     the paragraph being built has not yet been appended to its
	//     parent — so the message goes BEFORE that paragraph, as a
	//     sibling, not inside it.
	//
	// Both were read off the reference's own output rather than reasoned
	// about in the abstract.
	// An element that can itself hold body elements IS the msgnode, so
	// the message goes INSIDE it. Where inside follows from what was
	// already attached when docutils registered the name: a section's
	// <title> is built with the section, so the message lands after it; a
	// footnote's or citation's own body is not, so the message lands
	// first, after only the <label> that was built alongside it.
	if el.Tag == doctree.TagSection {
		insertAfterTitle(el, msg)
		return
	}
	if el.Tag == doctree.TagFootnote || el.Tag == doctree.TagCitation {
		insertAfterLabel(el, msg)
		return
	}
	host := parent
	if host == nil || !admitsBodyElements(hostMsgnodeTag(host, body)) {
		// msgnode cannot hold body elements, so docutils BUILDS the
		// message and then drops it. A duplicate inside a line block is
		// the corpus's own example: the <line> is msgnode there, and
		// "System messages are no longer inserted between <line>s".
		return
	}
	p.queueMessage(body, topLevel, msg)
}

// queueMessage records a message to be inserted before mark in parent, for
// flushQueuedMessages to apply. A nil mark appends.
func (p *parser) queueMessage(parent, mark, msg *doctree.Element) {
	p.queuedMessages = append(p.queuedMessages, queuedMessage{parent: parent, mark: mark, msg: msg})
}

// flushQueuedMessages applies every queued insertion, rebuilding each parent's
// children slice ONCE however many messages land in it. Messages queued for the
// same mark keep the order they were queued in, which is the order
// insertBefore produced when it ran one at a time.
func (p *parser) flushQueuedMessages() {
	if len(p.queuedMessages) == 0 {
		return
	}
	type plan struct {
		before map[*doctree.Element][]*doctree.Element
		atEnd  []*doctree.Element
	}
	plans := map[*doctree.Element]*plan{}
	var order []*doctree.Element
	for _, q := range p.queuedMessages {
		pl, ok := plans[q.parent]
		if !ok {
			pl = &plan{before: map[*doctree.Element][]*doctree.Element{}}
			plans[q.parent] = pl
			order = append(order, q.parent)
		}
		if q.mark == nil {
			pl.atEnd = append(pl.atEnd, q.msg)
			continue
		}
		pl.before[q.mark] = append(pl.before[q.mark], q.msg)
	}
	p.queuedMessages = nil
	for _, parent := range order {
		pl := plans[parent]
		out := make([]doctree.Node, 0, len(parent.Children)+len(pl.atEnd))
		for _, c := range parent.Children {
			if ce, ok := c.(*doctree.Element); ok {
				for _, msg := range pl.before[ce] {
					out = append(out, msg)
				}
				delete(pl.before, ce)
			}
			out = append(out, c)
		}
		// A mark that is no longer a child of parent -- nothing in the corpus
		// produces one, but insertBefore appended in that case and so does this.
		for _, msgs := range pl.before {
			for _, msg := range msgs {
				out = append(out, msg)
			}
		}
		out = append(out, nodesOf(pl.atEnd)...)
		parent.Children = out
	}
}

// nodesOf widens a slice of elements to a slice of nodes.
func nodesOf(els []*doctree.Element) []doctree.Node {
	out := make([]doctree.Node, len(els))
	for i, el := range els {
		out[i] = el
	}
	return out
}

// inlineHost returns the block whose inline content el belongs to — the
// nearest ancestor below body that directly hosts inline nodes.
func inlineHost(body, el *doctree.Element) *doctree.Element {
	var found *doctree.Element
	var walk func(cur *doctree.Element)
	walk = func(cur *doctree.Element) {
		for _, c := range cur.Children {
			ce, ok := c.(*doctree.Element)
			if !ok {
				continue
			}
			if ce == el {
				found = cur
				return
			}
			walk(ce)
			if found != nil {
				return
			}
		}
	}
	walk(body)
	return found
}

// hostMsgnodeTag names the node docutils would have as self.parent while
// parsing host's inline content — which is what its "can this hold body
// elements?" test actually runs against. For a PARAGRAPH that is the
// paragraph's own parent, because the paragraph has not been appended
// yet; for a <line>, the line itself, which is why a duplicate there is
// reported nowhere.
func hostMsgnodeTag(host, body *doctree.Element) string {
	if host.Tag == doctree.TagParagraph {
		return body.Tag
	}
	return host.Tag
}

// insertAfterLabel puts msg at the front of el's children, after a
// <label> if it has one.
func insertAfterLabel(el, msg *doctree.Element) {
	at := 0
	if len(el.Children) > 0 {
		if ce, ok := el.Children[0].(*doctree.Element); ok && ce.Tag == doctree.TagLabel {
			at = 1
		}
	}
	rest := append([]doctree.Node{msg}, el.Children[at:]...)
	el.Children = append(el.Children[:at:at], rest...)
}

// insertAfterTitle puts msg immediately after section's <title> child.
func insertAfterTitle(section, msg *doctree.Element) {
	for i, c := range section.Children {
		if ce, ok := c.(*doctree.Element); ok && ce.Tag == doctree.TagTitle {
			rest := append([]doctree.Node{msg}, section.Children[i+1:]...)
			section.Children = append(section.Children[:i+1:i+1], rest...)
			return
		}
	}
	section.Append(msg)
}

// topLevelAncestor returns the direct child of body that contains el (or
// el itself when it already is one), so the message can be inserted right
// before the block the duplicate appeared in.
func topLevelAncestor(body, el *doctree.Element) *doctree.Element {
	for _, c := range body.Children {
		if ce, ok := c.(*doctree.Element); ok && containsElement(ce, el) {
			return ce
		}
	}
	return nil
}

func containsElement(root, want *doctree.Element) bool {
	if root == want {
		return true
	}
	for _, c := range root.Children {
		if ce, ok := c.(*doctree.Element); ok && containsElement(ce, want) {
			return true
		}
	}
	return false
}

// insertBefore puts msg immediately before mark among parent's children,
// appending when mark isn't one of them.
func insertBefore(parent, mark, msg *doctree.Element) {
	for i, c := range parent.Children {
		if c == doctree.Node(mark) {
			rest := append([]doctree.Node{msg}, parent.Children[i:]...)
			parent.Children = append(parent.Children[:i:i], rest...)
			return
		}
	}
	parent.Append(msg)
}

// admitsBodyElements reports whether a tag's docutils content model
// includes Body elements — the test set_duplicate_name itself makes
// before attaching a message. A <line>, a <title> or a <paragraph> does
// not; a document, a section, a list item or a block quote does.
func admitsBodyElements(tag string) bool {
	switch tag {
	case doctree.TagDocument, doctree.TagSection, doctree.TagListItem,
		doctree.TagBlockQuote, doctree.TagFootnote, doctree.TagCitation,
		doctree.TagDefinition, doctree.TagFieldBody, doctree.TagTopic,
		doctree.TagSidebar, doctree.TagContainer, doctree.TagCompound,
		doctree.TagAdmonition, doctree.TagEntry,
		// The NINE specific admonitions, and header/footer. This list
		// named ".. admonition::" and not one of the nine it shares its
		// content model with, so a duplicate name inside a ".. note::"
		// put its message BEFORE the note where docutils puts it inside,
		// as the note's first child. PEP 813 is the corpus file; every
		// one of the eleven was then checked against the reference
		// individually, since a family is exactly what this kind of list
		// gets wrong.
		doctree.TagAttention, doctree.TagCaution, doctree.TagDanger,
		doctree.TagErrorAdmonition, doctree.TagHint, doctree.TagImportant,
		doctree.TagNote, doctree.TagTip, doctree.TagWarningAdmonition,
		doctree.TagHeader, doctree.TagFooter:
		return true
	}
	return false
}

// queuedMessage is one pending insertion: msg goes before mark in parent, or at
// the end of parent when mark is nil.
type queuedMessage struct {
	parent *doctree.Element
	mark   *doctree.Element
	msg    *doctree.Element
}
