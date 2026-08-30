// Package doctree defines the document tree produced by the reST parser,
// modeled on docutils.nodes (Body, Structural, Inline element categories).
package doctree

// Node is any element of the parsed document tree.
type Node interface {
	nodeName() string
}

// Element is a Node with children, mirroring docutils.nodes.Element.
type Element struct {
	Tag      string
	Children []Node
	Attrs    map[string]string
}

func (e *Element) nodeName() string { return e.Tag }

func NewElement(tag string, children ...Node) *Element {
	return &Element{Tag: tag, Children: children}
}

func (e *Element) Append(n Node) { e.Children = append(e.Children, n) }

func (e *Element) Attr(key string) string {
	if e.Attrs == nil {
		return ""
	}
	return e.Attrs[key]
}

func (e *Element) SetAttr(key, value string) {
	if e.Attrs == nil {
		e.Attrs = map[string]string{}
	}
	e.Attrs[key] = value
}

// Text is a leaf text node, mirroring docutils.nodes.Text.
type Text struct {
	Data string
}

func (t *Text) nodeName() string { return "#text" }

// Element tag names, mirroring the docutils.nodes class names used by
// the pseudoxml writer (kept as plain strings rather than a Go type per
// tag: docutils itself treats them as an open, extensible set via
// directives/roles, so a closed enum would fight the model).
const (
	TagDocument           = "document"
	TagSection            = "section"
	TagTitle              = "title"
	TagParagraph          = "paragraph"
	TagBulletList         = "bullet_list"
	TagEnumeratedList     = "enumerated_list"
	TagListItem           = "list_item"
	TagBlockQuote         = "block_quote"
	TagTransition         = "transition"
	TagEmphasis           = "emphasis"
	TagStrong             = "strong"
	TagLiteral            = "literal"
	TagReference          = "reference"
	TagTarget             = "target"
	TagSubstitutionRef    = "substitution_reference"
	TagProblematic        = "problematic"
	TagSystemMessage      = "system_message"
	TagComment            = "comment"
	TagDirective          = "directive"
	TagLiteralBlock       = "literal_block"
	TagFieldList          = "field_list"
	TagField              = "field"
	TagFieldName          = "field_name"
	TagFieldBody          = "field_body"
	TagDefinitionList     = "definition_list"
	TagDefinitionListItem = "definition_list_item"
	TagTerm               = "term"
	TagDefinition         = "definition"
	TagLineBlock          = "line_block"
	TagLine               = "line"
	TagDoctestBlock       = "doctest_block"
)

// AsText concatenates all Text descendants, mirroring docutils
// Element.astext() (used for title-derived id/name generation).
func AsText(n Node) string {
	switch v := n.(type) {
	case *Text:
		return v.Data
	case *Element:
		s := ""
		for _, c := range v.Children {
			s += AsText(c)
		}
		return s
	default:
		return ""
	}
}
