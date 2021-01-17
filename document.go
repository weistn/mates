package main

import (
	"fmt"
)

/***
 * Classes and functions in node file represent a parsed document.
 * This parsed document is accessible to HTML templates.
 *
 * Some utility functions in node file are available to HTML templates as well.
 */

var uniqueID int

// Node is the basic type for all structs that build a document tree.
type Node interface {
	NodeName() string
}

// NodeWithText represents a node in the document tree that can be parent to text nodes.
type NodeWithText interface {
	Node
	AppendText(node Node)
	TextChildren() []Node
	documentNode() *DocumentNode
}

// DocumentNode represents the result of parsing a document,
// which is composed of a tree of document nodes.
// The leafs of DocumentNode are of type Text.
type DocumentNode struct {
	// Name of the tag
	Tag string
	// Definition of the tag
	TagDefinition *TagDefinition
	// State of all counters when the node has been created.
	// This includes all counter increments and resets triggered by the node.
	Counters map[string]int
	// forceID is an Id enforced onto a DocumentNode while processing a template
	forcedID string
	// The parent node or nil
	Parent      *DocumentNode
	NextSibling *DocumentNode
	PrevSibling *DocumentNode
	Children    []*DocumentNode
	// The text belonging to node node
	Text []Node
	// The root of the document node tree.
	Document *DocumentNode
	// Parsed attributes
	Attributes map[string]string
	Colspan    int
	Config     map[string]interface{}
	Indent     int
	ctx        *NodeContext
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *DocumentNode) NodeName() string {
	return node.Tag
}

// AppendText adds text to the node.
func (node *DocumentNode) AppendText(text Node) {
	node.Text = append(node.Text, text)
}

// TextChildren returns the direct text node children.
func (node *DocumentNode) TextChildren() []Node {
	return node.Text
}

func (node *DocumentNode) documentNode() *DocumentNode {
	return node
}

// TextNode represents some plaintext without markup.
// TextNode implments the Text interface.
type TextNode struct {
	Text   string
	Parent NodeWithText
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *TextNode) NodeName() string {
	return "node:Text"
}

// StyleNode represents a CSS markup.
// StyleNode implements the Text interface.
type StyleNode struct {
	Name            string
	StyleDefinition *StyleDefinition
	Attributes      map[string]string
	Parent          NodeWithText
	Text            []Node
	Value           string
	forcedID        string
	ctx             *StyleContext
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *StyleNode) NodeName() string {
	return "node:Style"
}

// AppendText adds text to the node.
func (node *StyleNode) AppendText(text Node) {
	node.Text = append(node.Text, text)
}

// TextChildren returns the direct text node children.
func (node *StyleNode) TextChildren() []Node {
	return node.Text
}

func (node *StyleNode) documentNode() *DocumentNode {
	return node.Parent.documentNode()
}

func (node *StyleNode) entities() []*EntityNode {
	var result []*EntityNode
	for _, t := range node.Text {
		if e, ok := t.(*EntityNode); ok {
			result = append(result, e)
		} else if s, ok := t.(*StyleNode); ok {
			result = append(result, s.entities()...)
		}
	}
	return result
}

func (node *StyleNode) styles() []*StyleNode {
	var result []*StyleNode
	result = append(result, node)
	for _, t := range node.Text {
		if s, ok := t.(*StyleNode); ok {
			result = append(result, s.styles()...)
		}
	}
	return result
}

// ID returns the HTML-id assigned to an entity.
func (node *StyleNode) ID() string {
	if label, ok := node.Attributes["label"]; ok {
		return label
	}
	if node.forcedID != "" {
		return node.forcedID
	}
	return ""
}

// Style returns the CSS style applied to an entity.
func (node *StyleNode) Style() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k == "class" || k == "label" {
			continue
		}
		result += k + ":" + v + ";"
	}
	return result
}

// Class returns the CSS class applied to an entity.
func (node *StyleNode) Class() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k != "class" {
			continue
		}
		if result != "" {
			result += " "
		}
		result += v
	}
	return result
}

// ForceID returns the id of the entity.
// If it has none, the function enforces an ID.
func (node *StyleNode) ForceID() string {
	if node.ID() == "" {
		if node.forcedID == "" {
			node.forcedID = fmt.Sprintf("_id_%v", uniqueID)
			uniqueID++
		}
		return node.forcedID
	}
	return node.ID()
}

// HasClass returns true of the CSS class string contains the requested class.
func (node *StyleNode) HasClass(class string) bool {
	for k, v := range node.Attributes {
		if v == "" && k == class {
			return true
		}
	}
	return false
}

/*
func (node *StyleNode) Source() string {
	if len(node.Styles) == 0 {
		return "}"
	}
	result := "{"
	last := ""
	for k, v := range node.Styles {
		last = k
		if result != "" {
			result += ";"
		}
		if v == "" {
			result += k
		} else {
			result += k + ":" + v
		}
	}
	if last != "!" && last != "?" && last != "@" && last != "_" && last != "*" && last != "/" {
		result += " "
	}
	return result
}
*/

// MathNode implements the Text interface.
type MathNode struct {
	Text   string
	Parent NodeWithText
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *MathNode) NodeName() string {
	return "node:Math"
}

/*
func (node *MathNode) Source() string {
	// TODO: proper escaping
	if t := node.Parent.parser.template.GetTag(node.Parent.Tag); t != nil && t.SectionMode == section_Math {
		return node.Text
	}
	return "$" + node.Text + "$"
}
*/

// CodeNode implements the Text interface.
type CodeNode struct {
	Text   string
	Parent NodeWithText
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *CodeNode) NodeName() string {
	return "node:Code"
}

/*
func (node *CodeNode) Source() string {
	// TODO: proper escaping
	if t := node.Parent.parser.template.GetTag(node.Parent.Tag); t != nil && t.SectionMode == section_Code {
		return node.Text
	}
	return "`" + node.Text + "`"
}
*/

// EntityNode implements the Text interface.
type EntityNode struct {
	Name             string
	EntityDefinition *EntityDefinition
	Attributes       map[string]string
	Parent           NodeWithText
	// State of all counters when node node has been created.
	// This includes all counter increments and resets triggered by node node
	Counters map[string]int
	// The text following the double point.
	Value    string
	forcedID string
	ctx      *EntityContext
}

// NodeName returns a type string that can be used to filter nodes by their type.
func (node *EntityNode) NodeName() string {
	return "node:Entity"
}

// ID returns the HTML-id assigned to an entity.
func (node *EntityNode) ID() string {
	if label, ok := node.Attributes["label"]; ok {
		return label
	}
	if node.forcedID != "" {
		return node.forcedID
	}
	return ""
}

// Style returns the CSS style applied to an entity.
func (node *EntityNode) Style() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k == "class" || k == "label" {
			continue
		}
		result += k + ":" + v + ";"
	}
	return result
}

// Class returns the CSS class applied to an entity.
func (node *EntityNode) Class() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k != "class" {
			continue
		}
		if result != "" {
			result += " "
		}
		result += v
	}
	return result
}

// ForceID returns the id of the entity.
// If it has none, the function enforces an ID.
func (node *EntityNode) ForceID() string {
	if node.ID() == "" {
		if node.forcedID == "" {
			node.forcedID = fmt.Sprintf("_id_%v", uniqueID)
			uniqueID++
		}
		return node.forcedID
	}
	return node.ID()
}

// HasClass returns true of the CSS class string contains the requested class.
func (node *EntityNode) HasClass(class string) bool {
	for k, v := range node.Attributes {
		if v == "" && k == class {
			return true
		}
	}
	return false
}

/*
func (node *EntityNode) Source() string {
	result := "~" + node.Name
	if tmp, ok := node.Attributes[node.Name]; ok {
		result += ":" + tmp
	}
	if node.Attributes != nil {
		for k, v := range node.Attributes {
			result += ";" + k
			if v != "" {
				result += ":" + v
			}
		}
	}
	result += "~"
	return result
}
*/

// ID returns the HTML-id assigned to a DocumentNode.
func (node *DocumentNode) ID() string {
	if label, ok := node.Attributes["label"]; ok {
		return label
	}
	if node.forcedID != "" {
		return node.forcedID
	}
	return ""
}

// Style returns the CSS style applied to a DocumentNode.
func (node *DocumentNode) Style() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k == "class" || k == "label" {
			continue
		}
		result += k + ":" + v + ";"
	}
	return result
}

// Class returns the CSS class applied to a DocumentNode.
func (node *DocumentNode) Class() string {
	var result string
	for k, v := range node.Attributes {
		if v == "" {
			continue
		}
		if k != "class" {
			continue
		}
		if result != "" {
			result += " "
		}
		result += v
	}
	return result
}

/*
func (node *DocumentNode) Source() string {
	var result string
	if node.Parent == nil { // Root?
		// Do nothing by intention
	} else if node.Tag == "" && len(node.Attributes) == 0 {
		result += "\n\n"
	} else if node.Tag == "thead-row" {
		result += "\n"
	} else if node.Tag == "ul1" || node.Tag == "ul2" || node.Tag == "ul3" || node.Tag == "ol1" || node.Tag == "ol2" || node.Tag == "ol3" {
		// Do nothing by intention
	} else if node.Tag != "table" && len(node.Attributes) == 0 && len(node.Text) == 0 && len(node.Children) > 0 && *node.parser.template.GetTag(node.Children[0].Tag).DefaultParent == node.Tag {
		// Dn nothing by default
	} else if node.Tag == "thead-cell" || node.Tag == "tbody-cell" || node.Tag == "tbody-row" {
		// Dn nothing by default
	} else {
		if node.Tag == "li1" || node.Tag == "li2" || node.Tag == "li3" {
			var enum = ""
			t := node
			for t.Parent != nil && (t.Parent.Tag == "ul1" || t.Parent.Tag == "ul2" || t.Parent.Tag == "ul3" || t.Parent.Tag == "ol1" || t.Parent.Tag == "ol2" || t.Parent.Tag == "ol3") {
				if t.Parent.Tag[0] == 'u' {
					enum = "-" + enum
				} else {
					enum = "*" + enum
				}
				t = t.Parent
			}
			result += "\n" + enum
		} else {
			result = "\n#" + node.Tag
		}
		for k, v := range node.Attributes {
			result += ";"
			k = strings.Replace(k, " ", `\ `, -1)
			v = strings.Replace(v, " ", `\ `, -1)
			if v == "" {
				result += k
			} else {
				result += k + ":" + v
			}
		}
		result += " "
	}
	for i, t := range node.Text {
		text := t.Source()
		if i == 0 {
			text = strings.TrimLeft(text, "\n\t\f\r ")
		}
		if i + 1 == len(node.Text) {
			text = strings.TrimRight(text, "\n\t\f\r ")
		}
		result += text
	}
	for _, c := range node.Children {
		result += c.Source()
	}

	if node.Tag == "thead-cell" || node.Tag == "tbody-cell" {
		result += "|"
	}
	if node.Tag == "thread-row" || node.Tag == "tbody-row" {
		if node.Parent.Children[len(node.Parent.Children) - 1] != node {
			result += "\n"
		}
	}
	return result
}
*/

// PlainText returns the text embedded in TextNode objects which are direct leafes to this node.
func (node *DocumentNode) PlainText() string {
	var result string
	for _, t := range node.Text {
		switch t.(type) {
		case *TextNode:
			result += t.(*TextNode).Text
		}
	}
	return result
}

// ForceID returns the id of the document node.
// If it has none, node function enforces an ID.
func (node *DocumentNode) ForceID() string {
	if node.ID() == "" {
		if node.forcedID == "" {
			node.forcedID = fmt.Sprintf("_id_%v", uniqueID)
			uniqueID++
		}
		return node.forcedID
	}
	return node.ID()
}

// HasClass returns true if the CSS class string contains the requested class.
func (node *DocumentNode) HasClass(class string) bool {
	for k, v := range node.Attributes {
		if v == "" && k == class {
			return true
		}
	}
	return false
}

// ColumnCount returns the number of columns, if node node represents a table.
func (node *DocumentNode) ColumnCount() (int, error) {
	//	if node.Tag != "table" {
	//		return 0, errors.New("Not a table")
	//	}
	for _, c := range node.Children {
		if c.Tag == "#thead" {
			for _, row := range c.Children {
				if row.Tag == "#thead-row" {
					columns := 0
					for _, cell := range row.Children {
						if cell.Tag == "#thead-cell" {
							columns++
						}
					}
					return columns, nil
				}
			}
		}
	}
	return 0, nil
}

// Columns returns the columns, if node node represents a table.
func (node *DocumentNode) Columns() ([]*DocumentNode, error) {
	//	if node.Tag != "table" {
	//		return nil, errors.New("Not a table")
	//	}
	result := []*DocumentNode{}
	for _, c := range node.Children {
		if c.Tag == "#thead" {
			for _, row := range c.Children {
				if row.Tag == "#thead-row" {
					for _, cell := range row.Children {
						if cell.Tag == "#thead-cell" {
							result = append(result, cell)
						}
					}
				}
			}
		}
	}
	return result, nil
}

/*
func (node *DocumentNode) InsertRow(cells ...string) (*DocumentNode, error) {
//	if node.Tag != "table" {
//		return nil, errors.New("Not a table")
//	}
	count, err := node.ColumnCount()
	if err != nil {
		return nil, err
	}
	if len(cells) != count {
		return nil, errors.New("Mismatch in column count")
	}
	// Find or create the 'tbody' node
	var body *DocumentNode
	tmp := node.Nodes("tbody")
	if len(tmp) == 0 {
		var prev *DocumentNode
		if len(node.Children) != 0 {
			prev = node.Children[len(node.Children)-1]
		}
		body = &DocumentNode{parser: node.parser, Tag: "tbody", Document: node.Document, PrevSibling: prev}
		if prev != nil {
			prev.NextSibling = body
		}
		node.Children = append(node.Children, body)
	} else {
		body = tmp[0]
	}
	// Create the 'tbody-row' node
	row := &DocumentNode{parser: node.parser, Tag: "tbody-row", Document: node.Document}
	body.Children = append(body.Children, row)
	// Create all 'tbody-cell' nodes
	var prev *DocumentNode
	for _, c := range cells {
		cell := &DocumentNode{parser: node.parser, Tag: "tbody-cell", Document: node.Document, Parent: row, PrevSibling: prev}
		if prev != nil {
			prev.NextSibling = cell
		}
		cell.Text = []Text{&TextNode{Parent: cell, Text: c}}
		row.Children = append(row.Children, cell)
		prev = cell
	}
	return row, nil
}
*/

/*
func (node *DocumentNode) DeleteRow(row *DocumentNode) error {
	if node.Tag != "tbody-row" {
		return errors.New("Not a table row")
	}
	index := -1
	for i, c := range row.Parent.Children {
		if c == row {
			index = i
			break
		}
	}
	row.Parent.Children = append(row.Parent.Children[:index], row.Parent.Children[index+1:]...)
	return nil
}
*/

// Rows returns the child rows of a DocumentNode.
func (node *DocumentNode) Rows() []*DocumentNode {
	var result []*DocumentNode
	for _, c := range node.Children {
		if c.Tag == "#tbody" {
			for _, row := range c.Children {
				if row.Tag == "#tbody-row" {
					result = append(result, row)
				}
			}
		}
	}
	return result
}

/*
func (node *DocumentNode) RowByHash(hash string) (*DocumentNode, error) {
//	if node.Tag != "table" {
//		return nil, errors.New("Not a table")
//	}
	for _, c := range node.Children {
		if c.Tag == "tbody" {
			for _, row := range c.Children {
				if row.Tag == "tbody-row" {
					h := sha256.New()
					h.Write([]byte(row.Source()))
					b := hex.EncodeToString(h.Sum(nil))
					if b == hash {
						return row, nil
					}
				}
			}
		}
	}
	return nil, nil
}
*/

// DocumentNodes returns all DocumentNode chikdren that have one of the specified tag names.
func (node *DocumentNode) DocumentNodes(names ...string) []*DocumentNode {
	result := []*DocumentNode{}
	return node.documentNodes(result, names)
}

func (node *DocumentNode) documentNodes(val []*DocumentNode, names []string) []*DocumentNode {
	if len(names) == 0 {
		val = append(val, node.Children...)
	} else {
		for _, n := range names {
			if node.Tag == n {
				val = append(val, node)
			}
		}
	}
	for _, c := range node.Children {
		val = c.documentNodes(val, names)
	}
	return val
}

// Entities returns all direct or indirect entities owned by the DocumentNode.
func (node *DocumentNode) Entities() []*EntityNode {
	var result []*EntityNode
	for _, t := range node.Text {
		if e, ok := t.(*EntityNode); ok {
			result = append(result, e)
		} else if s, ok := t.(*StyleNode); ok {
			result = append(result, s.entities()...)
		}
	}
	for _, c := range node.Children {
		result = append(result, c.Entities()...)
	}
	return result
}

func (node *DocumentNode) styles() []*StyleNode {
	var result []*StyleNode
	for _, t := range node.Text {
		if s, ok := t.(*StyleNode); ok {
			result = append(result, s.styles()...)
		}
	}
	for _, c := range node.Children {
		result = append(result, c.styles()...)
	}
	return result
}

func (node *DocumentNode) hasMathNodes() bool {
	for _, t := range node.Text {
		if _, ok := t.(*MathNode); ok {
			return true
		}
	}
	for _, c := range node.Children {
		if c.hasMathNodes() {
			return true
		}
	}
	return false
}

// NodeByID searches for an ID inside the DocumentNode and its child nodes.
func (node *DocumentNode) NodeByID(id string) *DocumentNode {
	if node.ID() == id {
		return node
	}
	for _, c := range node.Children {
		if result := c.NodeByID(id); result != nil {
			return result
		}
	}
	return nil
}

// RemoveChild removes a child node from a DocumentNode.
func (node *DocumentNode) RemoveChild(child *DocumentNode) {
	for i, c := range node.Children {
		if c == child {
			node.Children = append(node.Children[:i], node.Children[i+1:]...)
			child.Parent = nil
			return
		}
	}
}
