package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// NodeContext is available to HTML templates.
type NodeContext struct {
	node *DocumentNode
	gen  *HTMLGenerator
}

// EntityContext is available to HTML templates.
type EntityContext struct {
	node *EntityNode
	gen  *HTMLGenerator
}

// StyleContext is available to HTML templates.
type StyleContext struct {
	node *StyleNode
	gen  *HTMLGenerator
}

// PageContext is available to HTML pages
type PageContext struct {
	// The `TagType` object for which this Page has been generated, or nil.
	TagType interface{}
	// The `TagValue` object for which this Page has been generated, or nil.
	TagValue interface{}

	page          *Page
	gen           *HTMLGenerator
	siteContext   interface{}
	folderContext interface{}
}

// Params returns a YAML map with attributes of the entire page.
func (ctx *PageContext) Params() map[string]interface{} {
	return ctx.page.Params
}

// Site returns an object that holds information about the site to which the page belongs.
func (ctx *PageContext) Site() interface{} {
	return ctx.siteContext
}

// Folder returns an object that holds information about the folder to which the page belongs.
func (ctx *PageContext) Folder() interface{} {
	return ctx.folderContext
}

// RelURL returns the relative URL of the generated page file.
func (ctx *PageContext) RelURL() string {
	return ctx.page.RelURL
}

// Type returns the name of the page type
func (ctx *PageContext) Type() string {
	return ctx.page.PageTypeName
}

// Title returns the title specified in the page config.
// A shortcut for .Params.Title
func (ctx *PageContext) Title() (string, error) {
	yamlNode, ok := ctx.page.Params["Title"]
	if !ok {
		return "", nil
	}
	if str, ok := yamlNode.(string); ok {
		return string(str), nil
	}
	return "", fmt.Errorf("Attribute title in page config must be of type string")
}

// NodeByID searches for an ID inside the DocumentNodes of the page.
func (ctx *PageContext) NodeByID(id string) *NodeContext {
	return wrapNode(ctx.gen, ctx.page.Document.NodeByID(id))
}

// NodesByTag returns all DocumentNodes of a specified tag, e.g. "#p" or "#code".
func (ctx *PageContext) NodesByTag(id string) []*NodeContext {
	nodes := ctx.page.Document.DocumentNodes(id)
	result := make([]*NodeContext, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, wrapNode(ctx.gen, n))
	}
	return result
}

// Document returns the root DocumentNode of the page
func (ctx *PageContext) Document() *NodeContext {
	return wrapNode(ctx.gen, ctx.page.Document)
}

// Content returns the HTML representation of the page content.
func (ctx *PageContext) Content() (string, error) {
	str, err := ctx.gen.outerHTML(ctx.page.Document)
	if err != nil {
		println(fmt.Sprintf("ERRC: %v", err))
	}
	return str, err
}

// Scripts returns a string that contains the HTML script tags required to load all scripts
// required by the page content.
func (ctx *PageContext) Scripts() string {
	str := ""
	for _, r := range ctx.gen.page.Resources {
		if r.Type == ResourceTypeScript {
			str += r.ToHTMLLink()
		}
	}
	return str
}

// Styles returns a string that contains the HTML style tags required to load all styles
// required by the page content.
func (ctx *PageContext) Styles() string {
	str := ""
	for _, r := range ctx.gen.page.Resources {
		if r.Type == ResourceTypeStyle {
			str += r.ToHTMLLink()
		}
	}
	return str
}

// Name returns the name of the entity.
func (ctx *EntityContext) Name() string {
	return ctx.node.Name
}

// Value returns the entity value.
func (ctx *EntityContext) Value() string {
	return ctx.node.Value
}

// Style returns the CSS style for the current DocumentNode as a string.
func (ctx *EntityContext) Style() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Style()
}

// Class returns the HTML classes for the current DocumentNode as a string.
func (ctx *EntityContext) Class() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Class()
}

// HasClass returns true of the CSS class string contains the requested class.
func (ctx *EntityContext) HasClass(class string) bool {
	if ctx.node == nil {
		return false
	}
	return ctx.node.HasClass(class)
}

// ID returns the HTML-ID for the current DocumentNode as a string.
func (ctx *EntityContext) ID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ID()
}

// ForceID forces the node to have a HTML-ID and returns it.
func (ctx *EntityContext) ForceID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ForceID()
}

// Name returns the name of the entity.
func (ctx *StyleContext) Name() string {
	return ctx.node.Name
}

// Value returns the entity value.
func (ctx *StyleContext) Value() string {
	return ctx.node.Value
}

// Style returns the CSS style for the current DocumentNode as a string.
func (ctx *StyleContext) Style() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Style()
}

// Class returns the HTML classes for the current DocumentNode as a string.
func (ctx *StyleContext) Class() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Class()
}

// HasClass returns true of the CSS class string contains the requested class.
func (ctx *StyleContext) HasClass(class string) bool {
	if ctx.node == nil {
		return false
	}
	return ctx.node.HasClass(class)
}

// ID returns the HTML-ID for the current DocumentNode as a string.
func (ctx *StyleContext) ID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ID()
}

// ForceID forces the node to have a HTML-ID and returns it.
func (ctx *StyleContext) ForceID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ForceID()
}

// InnerText returns the HTML for the text nodes inside the style.
func (ctx *StyleContext) InnerText() (string, error) {
	if ctx.node == nil {
		return "", nil
	}
	return ctx.gen.innerText(ctx.node)
}

// Counters returns the state of all counters when the node has been created.
// This includes all counter increments and resets triggered by the node.
func (ctx *NodeContext) Counters() map[string]int {
	if ctx.node == nil {
		return nil
	}
	return ctx.node.Counters
}

// PrevSibling returns the prev sibling DocumentNode instance.
func (ctx *NodeContext) PrevSibling() (*NodeContext, error) {
	if ctx.node == nil {
		return nil, nil
	}
	return wrapNode(ctx.gen, ctx.node.PrevSibling), nil
}

// NextSibling returns the next sibling DocumentNode instance.
func (ctx *NodeContext) NextSibling() (*NodeContext, error) {
	if ctx.node == nil {
		return nil, nil
	}
	return wrapNode(ctx.gen, ctx.node.NextSibling), nil
}

// Parent returns the parent DocumentNode instance.
func (ctx *NodeContext) Parent() (*NodeContext, error) {
	if ctx.node == nil {
		return nil, nil
	}
	return wrapNode(ctx.gen, ctx.node.Parent), nil
}

// Children returns the direct child DocumentNode instances.
func (ctx *NodeContext) Children() ([]*NodeContext, error) {
	if ctx.node == nil {
		return nil, nil
	}
	return wrapNodes(ctx.gen, ctx.node.Children), nil
}

// ColumnCount returns the number of columns, if node node represents a table.
// For all other DocumentNodes, ColumnCount returns 0.
func (ctx *NodeContext) ColumnCount() (int, error) {
	if ctx.node == nil {
		return 0, nil
	}
	return ctx.node.ColumnCount()
}

// Colspan returns the number of columns spanned by a table cell DocumentNode.
// For all other DocumentNodes, Colspan returns 0.
func (ctx *NodeContext) Colspan() int {
	if ctx.node == nil {
		return 0
	}
	return ctx.node.Colspan
}

// Columns returns the columns, if node node represents a table.
func (ctx *NodeContext) Columns() ([]*NodeContext, error) {
	if ctx.node == nil {
		return nil, nil
	}
	columns, err := ctx.node.Columns()
	if err != nil {
		return nil, err
	}
	return wrapNodes(ctx.gen, columns), nil
}

// Rows returns the child rows of a DocumentNode.
func (ctx *NodeContext) Rows() []*NodeContext {
	if ctx.node == nil {
		return nil
	}
	return wrapNodes(ctx.gen, ctx.node.Rows())
}

// Page returns the page context.
func (ctx *NodeContext) Page() *PageContext {
	return ctx.gen.pageContext
}

// Site returns the site context.
func (ctx *NodeContext) Site() interface{} {
	return ctx.gen.pageContext.Site()
}

// TagName returns the name of the tag.
// Normal paragraphs are reported to have the tagnane "p".
func (ctx *NodeContext) TagName() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Tag
}

// TagParams returns the parameters passed to '#tag' while defining the tag.
func (ctx *NodeContext) TagParams() map[string]interface{} {
	if ctx.node == nil {
		return nil
	}
	return ctx.node.TagDefinition.Config
}

// Style returns the CSS style for the current DocumentNode as a string.
func (ctx *NodeContext) Style() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Style()
}

// Class returns the HTML classes for the current DocumentNode as a string.
func (ctx *NodeContext) Class() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.Class()
}

// HasClass returns true of the CSS class string contains the requested class.
func (ctx *NodeContext) HasClass(class string) bool {
	if ctx.node == nil {
		return false
	}
	return ctx.node.HasClass(class)
}

// ID returns the HTML-ID for the current DocumentNode as a string.
func (ctx *NodeContext) ID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ID()
}

// Params returns the YAML map with attributes specific to the current DocumentNode.
func (ctx *NodeContext) Params() map[string]interface{} {
	if ctx.node == nil {
		return nil
	}
	return ctx.node.Config

}

// Content returns HTML for the child content of the current DocumentNode.
func (ctx *NodeContext) Content() (string, error) {
	if ctx.node == nil {
		return "", nil
	}
	if ctx.node.TagDefinition.ParentHood == TextParentHood {
		return ctx.gen.innerText(ctx.node)
	}
	return ctx.gen.innerHTML(ctx.node)
}

// Text returns HTML for tags that can only contain text.
// For others, it returns the text of the first child tag (if that child tag is of type #p).
// Otherwise, an empty string is returned
func (ctx *NodeContext) Text() (string, error) {
	if ctx.node == nil {
		return "", nil
	}
	if ctx.node.TagDefinition.ParentHood == TextParentHood {
		return ctx.gen.innerText(ctx.node)
	}
	if len(ctx.node.Children) > 0 && ctx.node.Children[0].TagDefinition.Name == "#p" {
		return ctx.gen.innerText(ctx.node.Children[0])
	}
	return "", nil
}

// RemainingContent returns everything that Content returns minus what Text returns.
func (ctx *NodeContext) RemainingContent() (string, error) {
	if ctx.node == nil {
		return "", nil
	}
	var result string
	for i := 1; i < len(ctx.node.Children); i++ {
		str, err := ctx.gen.outerHTML(ctx.node.Children[i])
		if err != nil {
			return "", err
		}
		result += str
	}
	return result, nil
}

// OuterHTML returns HTML for the current DocumentNode.
func (ctx *NodeContext) OuterHTML() (string, error) {
	if ctx.node == nil {
		return "", nil
	}
	return ctx.gen.outerHTML(ctx.node)
}

// ForceID forces the node to have a HTML-ID and returns it.
func (ctx *NodeContext) ForceID() string {
	if ctx.node == nil {
		return ""
	}
	return ctx.node.ForceID()
}

func funcHex(a []byte) string {
	return hex.EncodeToString([]byte(a))
}

func funcHash(a string) string {
	h := sha256.New()
	h.Write([]byte(a))
	b := hex.EncodeToString(h.Sum(nil))
	return b
}

func funcHasPrefix(prefix string, str string) bool {
	return strings.HasPrefix(str, prefix)
}

func funcPrefix(length int, str string) string {
	if len(str) < length {
		return str
	}
	var result string
	for _, r := range str {
		if length == 0 {
			break
		}
		result += string(r)
		length--
	}
	return result
}

func funcInitial(str string) string {
	for _, r := range str {
		return string(r)
	}
	return ""
}

func funcSort(list reflect.Value) ([]string, error) {
	a, err := strslice(list)
	if err != nil {
		return nil, err
	}
	s := sort.StringSlice(a)
	s.Sort()
	return s, nil
}

type sortHelper struct {
	list []interface{}
	keys []string
}

func (h *sortHelper) Len() int {
	return len(h.list)
}

func (h *sortHelper) Less(i, j int) bool {
	return h.keys[i] < h.keys[j]
}

func (h *sortHelper) Swap(i, j int) {
	tmp := h.list[i]
	tmpKey := h.keys[i]
	h.list[i] = h.list[j]
	h.keys[i] = h.keys[j]
	h.list[j] = tmp
	h.keys[j] = tmpKey
}

func funcSortBy(field string, list reflect.Value) ([]interface{}, error) {
	var h sortHelper
	switch list.Kind() {
	case reflect.Array, reflect.Slice:
		l := list.Len()
		h.list = make([]interface{}, 0, l)
		for i := 0; i < l; i++ {
			value := list.Index(i).Interface()
			h.list = append(h.list, value)
		}
	case reflect.Map:
		l := list.Len()
		h.list = make([]interface{}, 0, l)
		for _, key := range list.MapKeys() {
			value := list.MapIndex(key).Interface()
			h.list = append(h.list, value)
		}

	default:
		return nil, fmt.Errorf("sortBy: Unsupported data type")
	}

	h.keys = make([]string, len(h.list))
	for i, value := range h.list {
		v := reflect.ValueOf(value)
		// Try to extract a field of the given name
		if v.Kind() == reflect.Ptr {
			vvv := v.Elem()
			result := vvv.FieldByName(field)
			if result.IsValid() && result.Kind() == reflect.String {
				h.keys[i] = result.String()
				continue
			}
		} else {
			result := v.FieldByName(field)
			if result.IsValid() && result.Kind() == reflect.String {
				h.keys[i] = result.String()
				continue
			}
		}
		// Try to call a method of the given name
		method := v.MethodByName(field)
		if !method.IsValid() {
			return nil, fmt.Errorf("Field %v is neither a string, nor a method", field)
		}
		if method.Type().NumIn() != 0 {
			return nil, fmt.Errorf("Method %v expects additional arguments", field)
		}
		result := method.Call(nil)[0]
		if result.IsValid() && result.Kind() == reflect.String {
			h.keys[i] = result.String()
			continue
		}
	}

	sort.Sort(&h)
	return h.list, nil
}

func funcUniq(list reflect.Value) ([]interface{}, error) {
	switch list.Kind() {
	case reflect.Slice, reflect.Array:
		l := list.Len()
		dest := []interface{}{}
		for i := 0; i < l; i++ {
			item := list.Index(i).Interface()
			if !inList(dest, item) {
				dest = append(dest, item)
			}
		}
		return dest, nil
	default:
		return nil, fmt.Errorf("Expected a list instead of type %s", list.Type())
	}
}

func strslice(list reflect.Value) ([]string, error) {
	switch list.Kind() {
	case reflect.Array, reflect.Slice:
		l := list.Len()
		b := make([]string, l)
		for i := 0; i < l; i++ {
			b[i] = strval(list.Index(i))
		}
		return b, nil
	}
	return nil, fmt.Errorf("Expected an array or slice")
}

func inList(list []interface{}, element interface{}) bool {
	for _, h := range list {
		if reflect.DeepEqual(element, h) {
			return true
		}
	}
	return false
}

func strval(value reflect.Value) string {
	var v interface{}
	v = value.Interface()
	if v == nil {
		return "nil"
	}
	switch v := v.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func wrapNodes(gen *HTMLGenerator, nodes []*DocumentNode) []*NodeContext {
	var result = make([]*NodeContext, 0, len(nodes))
	for _, n := range nodes {
		if n.ctx == nil {
			n.ctx = &NodeContext{node: n, gen: gen}
		}
		result = append(result, n.ctx)
	}
	return result
}

func wrapNode(gen *HTMLGenerator, node *DocumentNode) *NodeContext {
	if node.ctx == nil {
		node.ctx = &NodeContext{node: node, gen: gen}
	}
	return node.ctx
}

func wrapEntity(gen *HTMLGenerator, node *EntityNode) *EntityContext {
	if node.ctx == nil {
		node.ctx = &EntityContext{node: node, gen: gen}
	}
	return node.ctx
}

func wrapStyle(gen *HTMLGenerator, node *StyleNode) *StyleContext {
	if node.ctx == nil {
		node.ctx = &StyleContext{node: node, gen: gen}
	}
	return node.ctx
}
