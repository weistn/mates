package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/weistn/template"
)

// Page is the result of parsing a markdown file.
type Page struct {
	Grammar *Grammar
	//	Meta     map[string]string
	Document *DocumentNode
	// Resources required by the generated output.
	// This field is populated while generating the output
	Resources []*Resource
	// The file name (and path) of the source file
	Fname string
	// Slash-separated path to the content-file generated on the file system, like "/recipes/beef/index.html".
	// This may be the empty string. In this case no content-file has been generated, for example because
	// the page type has been set to "none".
	RelURL       string
	Params       map[string]interface{}
	PageTypeName string
}

// GeneratorError reports an error that occured while generating output.
// The typical explanation is that a template could not be expanded.
type GeneratorError struct {
	Node Node
	Text string
}

// HTMLGenerator generates HTML from parsed markdown.
type HTMLGenerator struct {
	page         *Page
	options      *Options
	textTemplate *template.Template
	pageContext  *PageContext
	baseHTML     string
	layouts      []string
	resolver     ResourceResolver
}

func (err GeneratorError) Error() string {
	return err.Text
}

// NewHTMLGenerator returns a new HTML generator for a page.
func NewHTMLGenerator(page *Page, baseHTML string, layouts []string, resolver ResourceResolver, folderContext, siteContext interface{}, options *Options) *HTMLGenerator {
	gen := &HTMLGenerator{options: options, page: page, baseHTML: baseHTML, layouts: layouts, resolver: resolver}
	gen.pageContext = &PageContext{page: page, gen: gen, folderContext: folderContext, siteContext: siteContext}
	return gen
}

// PageContext returns the PageContext object that represents the generator's page during
// template execution.
func (gen *HTMLGenerator) PageContext() *PageContext {
	return gen.pageContext
}

// Page returns the page on which the generator is working.
func (gen *HTMLGenerator) Page() *Page {
	return gen.page
}

// Prepare parses and prepares all required templates and determines
// all required resources.
func (gen *HTMLGenerator) Prepare() error {
	// Find all required resources and add them to the file, but do not add duplicates
	resources := make(map[string]*Resource)
	addResource := func(r *Resource, add bool) error {
		err := gen.resolver(r)
		if err != nil {
			return err
		}
		id := r.UniqueID()
		if _, ok := resources[id]; !ok {
			resources[id] = r
			if add {
				gen.page.Resources = append(gen.page.Resources, r)
			}
		}
		return nil
	}

	gen.textTemplate = template.New("__main__").Delims("{{", "}}")
	fmap := make(template.FuncMap)
	fmap["prefix"] = funcPrefix
	fmap["hasPrefix"] = funcHasPrefix
	fmap["initial"] = funcInitial
	fmap["map"] = funcMap
	fmap["field"] = funcField
	fmap["uniq"] = funcUniq
	fmap["sort"] = funcSort
	fmap["sortBy"] = funcSortBy
	fmap["filter"] = funcFilter
	fmap["hex"] = funcHex
	fmap["hash"] = funcHash
	fmap["resource"] = func(urlString string) (string, error) {
		u, err := url.Parse(urlString)
		if err != nil {
			return "", err
		}
		r := &Resource{Type: ResourceTypeUnknown, URL: u}
		err = gen.resolver(r)
		if err != nil {
			return "", err
		}
		addResource(r, true)
		return r.URL.String(), nil
	}
	_, err := gen.textTemplate.Funcs(fmap).Parse(gen.baseHTML)
	if err != nil {
		return err
	}
	for _, l := range gen.layouts {
		_, err = gen.textTemplate.Parse(l)
		if err != nil {
			return err
		}
	}

	// Parse all tag templates
	for _, tagdef := range gen.page.Grammar.Tags {
		_, err := gen.textTemplate.New(tagdef.Name).Parse(tagdef.HTMLTemplate)
		if err != nil {
			return &GeneratorError{nil, fmt.Sprintf("Error parsing template for section '%v': %v", tagdef.Name, err)}
		}
	}
	// Parse all entity templates
	for _, edef := range gen.page.Grammar.Entities {
		_, err := gen.textTemplate.New(edef.Name).Parse(edef.HTMLTemplate)
		if err != nil {
			return &GeneratorError{nil, fmt.Sprintf("Error parsing template for entity '%v': %v", edef.Name, err)}
		}
	}
	// Parse all style templates
	for _, sdef := range gen.page.Grammar.Styles {
		_, err := gen.textTemplate.New(sdef.Name).Parse(sdef.HTMLTemplate)
		if err != nil {
			return &GeneratorError{nil, fmt.Sprintf("Error parsing template for style '%v': %v", sdef.Name, err)}
		}
	}

	// Resources defined in the page config
	for _, r := range gen.page.Resources {
		addResource(r, false)
	}
	// Resources required by the tags used
	for _, node := range gen.page.Document.DocumentNodes() {
		for _, r := range node.TagDefinition.Resources {
			addResource(r, true)
		}
	}
	// Resources required by the entities used
	for _, e := range gen.page.Document.Entities() {
		for _, r := range e.EntityDefinition.Resources {
			addResource(r, true)
		}
	}
	// Resources required by the styles used
	for _, s := range gen.page.Document.styles() {
		if s.StyleDefinition != nil {
			for _, r := range s.StyleDefinition.Resources {
				addResource(r, true)
			}
		}
	}

	return nil
}

// Generate HTML for the given file.
func (gen *HTMLGenerator) Generate() (string, error) {
	w := bytes.NewBuffer(nil)
	err := gen.textTemplate.ExecuteTemplate(w, "__main__", gen.pageContext)
	if err != nil {
		return "", &GeneratorError{gen.page.Document, fmt.Sprintf("Error executing template for page '%v': %v", gen.page.Fname, err)}
	}
	return w.String(), nil
}

func (gen *HTMLGenerator) imgSrc(href string) string {
	return fmt.Sprintf(`src="%v"`, href)
}

func (gen *HTMLGenerator) executeTemplate(name string, node *DocumentNode) (string, error) {
	w := bytes.NewBuffer(nil)
	ctx := wrapNode(gen, node)
	err := gen.textTemplate.ExecuteTemplate(w, name, ctx)
	if err != nil {
		return "", &GeneratorError{node, fmt.Sprintf("Error executing template for tag '%v': %v", name, err)}
	}
	return w.String(), nil
}

func (gen *HTMLGenerator) executeEntityTemplate(name string, node *EntityNode) (string, error) {
	w := bytes.NewBuffer(nil)
	ctx := wrapEntity(gen, node)
	err := gen.textTemplate.ExecuteTemplate(w, name, ctx)
	if err != nil {
		return "", &GeneratorError{node, fmt.Sprintf("Error executing template for entity '%v': %v", name, err)}
	}
	return w.String(), nil
}

func (gen *HTMLGenerator) executeStyleTemplate(name string, node *StyleNode) (string, error) {
	w := bytes.NewBuffer(nil)
	ctx := wrapStyle(gen, node)
	err := gen.textTemplate.ExecuteTemplate(w, name, ctx)
	if err != nil {
		return "", &GeneratorError{node, fmt.Sprintf("Error executing template for style '%v': %v", name, err)}
	}
	return w.String(), nil
}

func (gen *HTMLGenerator) outerHTML(node *DocumentNode) (string, error) {
	if node.Parent != nil { // Not the root node?
		tag := node.TagDefinition
		if tag == nil {
			return "", errors.New("Unknown tag: " + node.Tag)
		}
		str, err := gen.executeTemplate(tag.Name, node)
		if err != nil {
			return "", err
		}
		return str, nil
	}
	html := ""
	for _, c := range node.Children {
		tmp, err := gen.outerHTML(c)
		if err != nil {
			return "", err
		}
		html += tmp
	}
	return html, nil
}

func (gen *HTMLGenerator) innerHTML(node *DocumentNode) (string, error) {
	if node.Tag == "#pie" {
		return generatePieChart(node), nil
	} else if node.Tag == "#chart" {
		return generateLineBarChart(node), nil
	}

	result, err := gen.innerText(node)
	if err != nil {
		return "", err
	}

	for _, c := range node.Children {
		tmp, err := gen.outerHTML(c)
		if err != nil {
			return "", err
		}
		result += tmp
	}

	return result, nil
}

func (gen *HTMLGenerator) innerTags(node *DocumentNode) (string, error) {
	if node.Tag == "#pie" {
		return generatePieChart(node), nil
	} else if node.Tag == "#chart" {
		return generateLineBarChart(node), nil
	}

	result := ""
	for _, c := range node.Children {
		tmp, err := gen.outerHTML(c)
		if err != nil {
			return "", err
		}
		result += tmp
	}

	return result, nil
}

func (gen *HTMLGenerator) innerText(node NodeWithText) (string, error) {
	// Get the containing DocumentNode
	docNode := node.documentNode()

	var result = ""
	var style = make([]string, 5)
	var styleLevel = 0
	for _, t := range node.TextChildren() {
		switch t.(type) {
		case *TextNode:
			// Ignore text in a media context
			if docNode.TagDefinition.SectionMode == sectionMedia {
				break
			}
			result += html.EscapeString(t.(*TextNode).Text)
		case *MathNode:
			// Ignore text in a media context
			if docNode.TagDefinition.SectionMode == sectionMedia {
				break
			}
			if docNode.TagDefinition.SectionMode == sectionMath {
				result += html.EscapeString(t.(*MathNode).Text)
			} else {
				result += `<span class="math">\(`
				result += html.EscapeString(t.(*MathNode).Text)
				result += `\)</span>`
			}
		case *CodeNode:
			// Ignore text in a media context
			if docNode.TagDefinition.SectionMode == sectionMedia {
				break
			}
			if docNode.TagDefinition.SectionMode == sectionCode {
				result += html.EscapeString(t.(*CodeNode).Text)
			} else {
				result += `<code>`
				result += html.EscapeString(t.(*CodeNode).Text)
				result += `</code>`
			}
		case *StyleNode:
			// Ignore styles in a media context
			if docNode.TagDefinition.SectionMode == sectionMedia {
				break
			}
			stylenode := t.(*StyleNode)
			if stylenode.Name == "*" {
				result += "<strong>"
				text, err := gen.innerText(stylenode)
				if err != nil {
					return "", err
				}
				result += text + "</strong>"
				break
			} else if stylenode.Name == "_" {
				result += "<em>"
				text, err := gen.innerText(stylenode)
				if err != nil {
					return "", err
				}
				result += text + "</em>"
				break
			}
			styledef := gen.page.Grammar.GetStyle(stylenode.Name)
			if styledef != nil {
				str, err := gen.executeStyleTemplate(stylenode.Name, stylenode)
				if err != nil {
					return "", err
				}
				result += str
				break
			}
			span := "<span style=\"" + stylenode.Style() + "\""
			if stylenode.Class() != "" {
				span += " class=\"" + stylenode.Class() + "\""
			}
			span += ">"
			result += span
			text, err := gen.innerText(stylenode)
			if err != nil {
				return "", err
			}
			result += text + "</span>"
		case *EntityNode:
			class := ""
			style := ""
			id := ""
			entity := t.(*EntityNode)
			for k, v := range entity.Attributes {
				if k == entity.Name {
					continue
				}
				if k == "label" {
					id = v
				} else if v == "" {
					if class != "" {
						class += " "
					}
					class += k
				} else {
					style += k + ":" + v + ";"
				}
			}
			if class != "" {
				class = fmt.Sprintf(` class="%v"`, class)
			}
			if style != "" {
				style = fmt.Sprintf(` style="%v"`, style)
			}
			if id != "" {
				id = fmt.Sprintf(` id="%v"`, id)
			}

			edef := gen.page.Grammar.GetEntity(entity.Name)
			if entity.Name == "img" {
				if docNode.TagDefinition.SectionMode == sectionMedia {
					result += `<li><a class="thumbnail" href="` + entity.Attributes["img"] + `"><img` + class + id + style + ` ` + gen.imgSrc(entity.Attributes["img"]) + `></a></li>`
				} else {
					result += `<img` + class + id + style + ` ` + gen.imgSrc(entity.Attributes["img"]) + `>`
				}
			} else if entity.Name == "a" {
				result += `<a` + class + id + style + ` href="` + entity.Attributes["a"] + `">` + entity.Attributes["a"] + `</a>`
			} else if entity.Name == "bib" {
				bibtext := entity.Attributes["bib"]
				bibs := strings.Split(bibtext, ",")
				result += `[`
				for i, b := range bibs {
					if i > 0 {
						result += ","
					}
					bibnode := docNode.Document.NodeByID(b)
					if bibnode == nil {
						log.Printf("Could not find bibliography '%v'", b)
						result += "?"
					} else {
						counter, ok := bibnode.Counters["bib"]
						if !ok {
							log.Printf("Bibliography entry '%v' has no 'bib' counter", b)
							result += "?"
						} else {
							result += fmt.Sprintf(`<a href="#%v">%v</a>`, b, strconv.Itoa(counter))
						}
					}
				}
				result += `]`
			} else if entity.Name == "progress" {
				c := "progress progress-inline"
				args := strings.Split(entity.Attributes["progress"], ":")
				w := args[0]
				t := args[0]
				if len(args) > 1 {
					t = args[1]
				}
				if class != "" {
					for _, x := range strings.Split(class[8:len(class)-1], " ") {
						if x == "active" {
							c += " progress-striped active"
						} else if x == "striped" {
							c += " progress-striped"
						} else if x == "info" {
							c += " progress-info"
						} else if x == "success" {
							c += " progress-success"
						} else if x == "warn" {
							c += " progress-warning"
						} else if x == "important" {
							c += " progress-danger"
						}
					}
				}
				result += `<div class="` + c + `"` + style + id + `><div class="bar" style="width:` + w + `">` + t + `</div></div>`
			} else if entity.Name == "br" {
				result += `<br>`
			} else if edef != nil {
				str, err := gen.executeEntityTemplate(entity.Name, entity)
				if err != nil {
					log.Printf("Error executing template for entity '%v': %v", entity.Name, err)
				}
				result += str
			} else {
				return "", fmt.Errorf("Unknown entity: %v", entity.Name)
			}
		}
	}
	for ; styleLevel >= 0; styleLevel-- {
		result += style[styleLevel]
	}

	return strings.TrimSpace(result), nil
}
