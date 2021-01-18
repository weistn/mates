package main

import (
	"fmt"
)

// ParentHood defines whether a tag can have child tags besides
// those tags which explicitly claim the tag as parent.
type ParentHood int

const (
	// NoParentHood means that the tags of this type have no child tags, except for those
	// tag types which explicitly claim the tags of this type as parent.
	NoParentHood ParentHood = 0
	// RootParentHood means that all tags of this type that have a DefaultParent of nil
	// can be a child of this tag.
	RootParentHood ParentHood = 1
	// DefaultRootParentHood is the same as RootParentHood.
	// In addition, this tag will become the DefaultParent to all those possible child tags.
	DefaultRootParentHood ParentHood = 2
	// IndentParentHood is similar to RootParentHood. The difference is that
	// all child tags must be indented.
	IndentParentHood ParentHood = 3
	// TextParentHood means that tags of this type can only contain text, styles and entities,
	// but no child tags.
	TextParentHood ParentHood = 4
)

// Grammar defines all tags and entities that can be used by a markdown file.
type Grammar struct {
	// Definition of tags, e.g. #row, #span4, #warn etc.
	Tags               []*TagDefinition
	Entities           []*EntityDefinition
	Styles             []*StyleDefinition
	currentDefaultRoot *TagDefinition
	tokenPos           ScannerRange
	token              Token
	tokenStr           string
	s                  *Scanner
}

// TagDefinition of a tag. These tag definitions build a grammar.
type TagDefinition struct {
	// Grammar in which the tag has been defined
	grammar *Grammar
	// Name of the tag
	Name string
	// The HTML template to execute for this tag
	HTMLTemplate string
	// The name of the default parent tag or nil (if the tag is at level 0)
	DefaultParent *string
	// Whether to parse the content as table, math, or normal text
	SectionMode int
	// Names of parent tags that are acceptable as parents.
	PossibleParents []string
	// Name of the counter to increase whenever this tag is encountered
	Counter string
	// Names of counters to set back to 0 when this tag is encountered
	ResetCounter []string
	// If the tag is followed by ":" then the value to the right is the value of the default attribute.
	// If no default attribute is specified, it is an error to use the ":" behind the tag name
	DefaultAttribute string
	ClassShortcuts   map[string]string
	// A list of resources required in case the tag is used
	Resources  []*Resource
	Config     map[string]interface{}
	ParentHood ParentHood
	// FirstChild of true forces a parent listed in PossibleParents to host this tag as its first child tag.
	// If this is not possible, the parent tag is closed and reopened.
	FirstChild bool
	// Determines whether the tag can be used (directly or via its default parents) as a child of a tag with ParentHood.
	// This value is derived from the default parents. Unless these insist on being tied to #root,
	// a tag can be used in block scope.
	allowedInParentHood bool
}

// EntityDefinition of an entity, e.g. ~note:Hello\ World~
type EntityDefinition struct {
	// Name of the entity
	Name string
	// The HTML template to execute for this entity
	HTMLTemplate string
	// Name of the counter to increase whenever this entity is encountered
	Counter        string
	ClassShortcuts map[string]string
	// A list of resources required in case the entity is used
	Resources []*Resource
}

// StyleDefinition of a style, e.g. {a:http://www.uni-due.de
type StyleDefinition struct {
	// Name of the entity
	Name string
	// The HTML template to execute for this entity
	HTMLTemplate   string
	ClassShortcuts map[string]string
	// A list of resources required in case the entity is used
	Resources []*Resource
}

// GrammarError is returned if a custom defined grammar is wrong.
type GrammarError struct {
	Text string
}

// Error returns the string representation of the GrammarError
func (err GrammarError) Error() string {
	return err.Text
}

// NewGrammar returns a new grammar.
func NewGrammar() *Grammar {
	tmpl := &Grammar{ /* TextGrammars: make(map[string]string) */ }
	tmpl.loadBuiltinTags()
	return tmpl
}

// GetTag searches for a TagDefinition by its name.
func (grammar *Grammar) GetTag(tagname string) *TagDefinition {
	// Search tags in reverse order, because tags can be overriden
	for i := len(grammar.Tags) - 1; i >= 0; i-- {
		tag := grammar.Tags[i]
		if tag.Name == tagname {
			return tag
		}
	}
	return nil
}

// GetEntity searches for an EntityDefinition by its name.
func (grammar *Grammar) GetEntity(entityname string) *EntityDefinition {
	for _, e := range grammar.Entities {
		if e.Name == entityname {
			return e
		}
	}
	return nil
}

// GetStyle searches for a StyleDefinition by its name.
func (grammar *Grammar) GetStyle(stylename string) *StyleDefinition {
	for _, e := range grammar.Styles {
		if e.Name == stylename {
			return e
		}
	}
	return nil
}

func (grammar *Grammar) addBuiltinTag(tag *TagDefinition) {
	if t := grammar.GetTag(tag.Name); t != nil {
		return
	}
	grammar.Tags = append(grammar.Tags, tag)
}

func (grammar *Grammar) addBuiltinEntity(entity *EntityDefinition) {
	if t := grammar.GetEntity(entity.Name); t != nil {
		return
	}
	grammar.Entities = append(grammar.Entities, entity)
}

func (grammar *Grammar) loadBuiltinTags() {
	//	customTags := len(grammar.Tags)
	/*	if grammar.Schema == "slides" {
		grammar.addBuiltinTag(&TagDefinition{grammar, 0, "slide-deck", `{{.InnerHTML}}`, nil, sectionNormal, nil, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "slide", `<div id="{{.Counters.slide}}" class="slide {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}><div class="content">{{grammar "slide-start" .}}{{.InnerHTML}}{{grammar "slide-end" .}}</div>{{grammar "slide-comments" .}}</div>`, newString("slide-deck"), sectionNormal, []string{"slide-deck"}, -1, "slide", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "title-slide", `<div id="{{.Counters.slide}}" class="slide slide-title {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}><div class="content">{{grammar "slide-start" .}}{{.InnerHTML}}{{grammar "slide-end" .}}</div>{{grammar "slide-comments" .}}</div>`, newString("slide-deck"), sectionNormal, []string{"slide-deck"}, -1, "slide", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "title", `<div{{if .ID}} id="{{.ID}}"{{end}} class="sec-title {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("title-slide"), sectionNormal, []string{"title-slide"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "author", `<div{{if .ID}} id="{{.ID}}"{{end}} class="sec-author {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("title-slide"), sectionNormal, []string{"title-slide"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "two-column", `<table{{if .ID}} id="{{.ID}}"{{end}} class="sec-multi-column {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}><tbody><tr>{{.InnerHTML}}</tr></tbody></table>`, newString("slide"), sectionNormal, []string{"slide", "title-slide"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "one-column", `{{.InnerHTML}}`, newString("slide"), sectionNormal, []string{"slide", "title-slide"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "left", `<td{{if .ID}} id="{{.ID}}"{{end}} class="sec-column {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</td>`, newString("two-column"), sectionNormal, []string{"two-column"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "right", `<td{{if .ID}} id="{{.ID}}"{{end}} class="sec-column {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</td>`, newString("two-column"), sectionNormal, []string{"two-column"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "main", `<div{{if .ID}} id="{{.ID}}"{{end}} class="sec-main {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("one-column"), sectionNormal, []string{"one-column"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "deflist", `<dl{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dl>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "deft", `<dt{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dt>`, newString("deflist"), sectionNormal, []string{"deflist"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "defd", `<dd{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dd>`, newString("deflist"), sectionNormal, []string{"deflist"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "warn", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "info", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-info {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "important", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-error {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "success", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-success {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "ul1", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "ul2", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("ul1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 4, "ul3", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("ul2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "li1", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li1 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 4, "li2", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li2 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 5, "li3", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li3 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul3"), sectionNormal, []string{"ul3", "ol3"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "ol1", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "ol2", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("ol1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 4, "ol3", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("ol2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "#", `<div{{if .ID}} id="{{.ID}}"{{end}} class="sec-header {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("slide"), sectionNormal, []string{"slide"}, 1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "##", `<h2{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h2>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "###", `<h3{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h3>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "####", `<h4{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h4>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "table", `<table{{if .ID}} id="{{.ID}}"{{end}} class="table {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</table>`, newString("main"), sectionTable, []string{"main", "left", "right"}, -1, "", nil, "Table", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "tbody", `<tbody{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</tbody>`, newString("table"), sectionTable, []string{"table", "pie", "chart"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "thead", `<thead{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</thead>`, newString("table"), sectionTable, []string{"table", "pie", "chart"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 4, "tbody-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</tr>`, newString("tbody"), sectionTable, []string{"tbody"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 4, "thead-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</tr>`, newString("thead"), sectionTable, []string{"thead"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 5, "tbody-cell", `<td{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.InnerHTML}}</td>`, newString("tbody-row"), sectionTable, []string{"tbody-row"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 5, "thead-cell", `<th{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.InnerHTML}}</th>`, newString("thead-row"), sectionTable, []string{"thead-row"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "code", `<pre{{if .ID}} id="{{.ID}}"{{end}}{{if .HasClass "nosyntax"}} class="{{.Class}}"{{else}} class="prettyprint {{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</pre>`, newString("main"), sectionCode, []string{"main", "left", "right"}, -1, "", nil, "Sample", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "math", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}><span class="math">\[{{.InnerHTML}}\]</span></div>`, newString("main"), sectionMath, []string{"main", "left", "right"}, -1, "", nil, "Formula", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "progress", `<div{{if .ID}} id="{{.ID}}"{{end}} class="progress {{.Class}}"><div class="bar" style="{{.Style}}">{{.InnerHTML}}</div></div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "width", map[string]string{"active": "progress-striped active", "striped": "progress-striped", "info": "progress-info", "success": "progress-success", "warn": "progress-warning", "important": "progress-danger"}})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "pie", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionTable, []string{"main", "left", "right"}, -1, "", nil, "Chart", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "chart", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("main"), sectionTable, []string{"main", "left", "right"}, -1, "", nil, "Chart", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "caption", `<div{{if .ID}} id="{{.ID}}"{{end}} class="caption {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{if .PrevSibling}}{{if .PrevSibling.Caption}}<span class="{{.PrevSibling.Tag}}count">{{.PrevSibling.Caption}} {{index .Counters .PrevSibling.Tag}}:</span> {{end}}{{end}}{{.InnerHTML}}</div>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "media", `<ul{{if .ID}} id="{{.ID}}"{{end}} class="media-grid {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("main"), sectionMedia, []string{"main", "left", "right"}, -1, "", nil, "Figure", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "bib", `<p{{if .ID}} id="{{.ID}}"{{end}} class="bibentry {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>[{{.Counters.bib}}] {{.InnerHTML}}</p>`, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "bib", nil, "", "label", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 2, "comment", ``, newString("main"), sectionNormal, []string{"main", "left", "right"}, -1, "", nil, "", "", nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2 + 3, "reply", ``, newString("comment"), sectionNormal, []string{"comment"}, -1, "", nil, "", "", nil})
	} else if grammar.Schema == "" || grammar.Schema == "page" { */
	/*
		grammar.addBuiltinTag(&TagDefinition{grammar, 0, "page", `<div{{if .ID}} id="{{.ID}}"{{end}} class="row jumbotron {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, nil, sectionNormal, nil, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 0, "row", `<div{{if .ID}} id="{{.ID}}"{{end}} class="row {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, nil, sectionNormal, nil, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span1", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span1 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span2", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span2 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span3", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span3 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span4", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span4 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span5", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span5 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span6", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span6 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span7", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span7 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span8", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span8 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span9", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span9 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span10", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span10 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span11", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span11 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 1, "span12", `<div{{if .ID}} id="{{.ID}}"{{end}} class="span12 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("row"), sectionNormal, []string{"row", "page"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "deflist", `<dl{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dl>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "deft", `<dt{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dt>`, newString("deflist"), sectionNormal, []string{"deflist"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "defd", `<dd{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</dd>`, newString("deflist"), sectionNormal, []string{"deflist"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "warn", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "info", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-info {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "important", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-error {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "success", `<div{{if .ID}} id="{{.ID}}"{{end}} class="alert alert-success {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "pie", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionTable, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Chart", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "chart", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionTable, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Chart", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "", `<div{{if .ID}} id="{{.ID}}"{{end}} class="p {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "ul1", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "ul2", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("ul1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 4, "ul3", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("ul2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "li1", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li1 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 4, "li2", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li2 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 5, "li3", `<li{{if .ID}} id="{{.ID}}"{{end}} class="li3 {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</li>`, newString("ul3"), sectionNormal, []string{"ul3", "ol3"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "ol1", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "ol2", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("ol1"), sectionNormal, []string{"ul1", "ol1"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 4, "ol3", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ol>`, newString("ol2"), sectionNormal, []string{"ul2", "ol2"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "#", `<h1{{if .ID}} id="{{.ID}}"{{else}} id="{{.ForceID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h1>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "##", `<h2{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h2>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "###", `<h3{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h3>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "####", `<h4{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</h4>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "table", `<table{{if .ID}} id="{{.ID}}"{{end}} class="table {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</table>`, newString("span12"), sectionTable, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Table", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "tbody", `<tbody{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}{{if .Parent.HasClass "editable"}}<tr class="edit-hide edit-control">{{range .Parent.Columns}}<td><input type="text"></td>{{end}}<td><button class="btn btn-success edit-submitrow">Submit</button></td></tr>{{end}}</tbody>`, newString("table"), sectionTable, []string{"table", "pie", "chart"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 3, "thead", `<thead{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</thead>`, newString("table"), sectionTable, []string{"table", "pie", "chart"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 4, "tbody-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if .Parent.Parent.HasClass "editable"}} data-hash="{{hash .Source}}"{{end}}>{{.InnerHTML}}{{if .Parent.Parent.HasClass "editable"}}<td class="edit-hide edit-control"><button class="btn btn-primary edit-editrow">Edit</button> <button class="btn btn-danger edit-deleterow">Delete</button></td>{{end}}</tr>`, newString("tbody"), sectionTable, []string{"tbody"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 4, "thead-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}{{if .Parent.Parent.HasClass "editable"}}<th class="edit-hide edit-control"></th>{{end}}</tr>`, newString("thead"), sectionTable, []string{"thead"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 5, "tbody-cell", `<td{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.InnerHTML}}</td>`, newString("tbody-row"), sectionTable, []string{"tbody-row"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 5, "thead-cell", `<th{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.InnerHTML}}</th>`, newString("thead-row"), sectionTable, []string{"thead-row"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "code", `<pre{{if .ID}} id="{{.ID}}"{{end}}{{if .HasClass "nosyntax"}} class="{{.Class}}"{{else}} class="prettyprint {{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</pre>`, newString("span12"), sectionCode, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Sample", "class", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "math", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}><span class="math">\[{{.InnerHTML}}\]</span></div>`, newString("span12"), sectionMath, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Formula", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "progress", `<div{{if .ID}} id="{{.ID}}"{{end}} class="progress {{.Class}}"><div class="bar" style="{{.Style}}">{{.InnerHTML}}</div></div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "width", map[string]string{"active": "progress-striped active", "striped": "progress-striped", "info": "progress-info", "success": "progress-success", "warn": "progress-warning", "important": "progress-danger"}, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "caption", `<div{{if .ID}} id="{{.ID}}"{{end}} class="caption {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{if .PrevSibling}}{{if .PrevSibling.Caption}}<span class="{{.PrevSibling.Tag}}count">{{.PrevSibling.Caption}} {{index .Counters .PrevSibling.Tag}}:</span> {{end}}{{end}}{{.InnerHTML}}</div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "media", `<ul{{if .ID}} id="{{.ID}}"{{end}} class="thumbnails {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{.InnerHTML}}</ul>`, newString("span12"), sectionMedia, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "Media", "", nil, nil})
		grammar.addBuiltinTag(&TagDefinition{grammar, 2, "bib", `<p{{if .ID}} id="{{.ID}}"{{end}} class="bibentry {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>[{{.Counters.bib}}] {{.InnerHTML}}</p>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "bib", nil, "", "label", nil, nil}) */
	//	grammar.addBuiltinTag(&TagDefinition{grammar, 2, "subnav", `<div{{if .ID}} id="{{.ID}}"{{end}} class="subnav {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}><ul class="nav nav-pills">{{template "subnavlist" .}}{{.InnerHTML}}</ul></div>`, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil})
	//	grammar.addBuiltinTag(&TagDefinition{grammar, 2, "comment", ``, newString("span12"), sectionNormal, []string{"span1", "span2", "span3", "span4", "span5", "span6", "span7", "span8", "span9", "span10", "span11", "span12"}, -1, "", nil, "", "", nil})
	//	grammar.addBuiltinTag(&TagDefinition{grammar, 3, "reply", ``, newString("comment"), sectionNormal, []string{"comment"}, -1, "", nil, "", "", nil})
	//	} else {
	//		log.Fatalf("Unknown schema: %v", grammar.Schema)
	//	}

	createConfig := func(caption string) map[string]interface{} {
		m := make(map[string]interface{})
		m["caption"] = caption
		m["counter"] = caption
		return m
	}

	grammar.addBuiltinTag(&TagDefinition{grammar, "#root", `{{.Content}}`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, RootParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#p", `<p{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</p>`, nil, sectionParagraph, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#ul", `<ul{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</ul>`, nil, sectionNormal, []string{ /*"#ul", "#ol"*/ }, "", nil, "", nil, nil, nil, IndentParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#li", `<li{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</li>`, newString("#ul"), sectionNormal, []string{"#ul", "#ol"}, "", nil, "", nil, nil, nil, TextParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#ol", `<ol{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</ol>`, nil, sectionNormal, []string{ /*"#ul", "#ol"*/ }, "", nil, "", nil, nil, nil, IndentParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#", `<h1{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</h1>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "##", `<h2{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</h2>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "###", `<h3{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</h3>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "####", `<h4{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</h4>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#table", `<table{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</table>`, nil, sectionTable, []string{}, "Table", nil, "", nil, nil, createConfig("Table"), NoParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#tbody", `<tbody{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</tbody>`, newString("#table"), sectionTable, []string{"#table", "#pie", "#chart"}, "", nil, "", nil, nil, nil, NoParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#thead", `<thead{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</thead>`, newString("#table"), sectionTable, []string{"#table", "#pie", "#chart"}, "", nil, "", nil, nil, nil, NoParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#tbody-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</tr>`, newString("#tbody"), sectionTable, []string{"#tbody"}, "", nil, "", nil, nil, nil, NoParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#thead-row", `<tr{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</tr>`, newString("#thead"), sectionTable, []string{"#thead"}, "", nil, "", nil, nil, nil, NoParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#tbody-cell", `<td{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.Content}}</td>`, newString("#tbody-row"), sectionTable, []string{"#tbody-row"}, "", nil, "", nil, nil, nil, TextParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#thead-cell", `<th{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{if ne .Colspan 1}} colspan="{{.Colspan}}"{{end}}>{{.Content}}</th>`, newString("#thead-row"), sectionTable, []string{"#thead-row"}, "", nil, "", nil, nil, nil, TextParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#code", `<pre{{if .ID}} id="{{.ID}}"{{end}}{{if .HasClass "nosyntax"}} class="{{.Class}}"{{else}} class="prettyprint {{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</pre>`, nil, sectionCode, []string{}, "Sample", nil, "class", nil, nil, createConfig("Sample"), TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#math", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}><span class="math">\[{{.Content}}\]</span></div>`, nil, sectionMath, []string{}, "Equation", nil, "", nil, nil, createConfig("Equation"), TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#bib", `<p{{if .ID}} id="{{.ID}}"{{end}} class="bibentry {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>[{{.Counters.bib}}] {{.Content}}</p>`, nil, sectionNormal, []string{}, "bib", nil, "label", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#pie", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</div>`, nil, sectionTable, []string{}, "Chart", nil, "", nil, nil, createConfig("Chart"), NoParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#chart", `<div{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</div>`, nil, sectionTable, []string{}, "Chart", nil, "", nil, nil, createConfig("Chart"), NoParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#caption", `<div{{if .ID}} id="{{.ID}}"{{end}} class="caption {{.Class}}"{{if .Style}} style="{{.Style}}"{{end}}>{{if .PrevSibling}}{{if and .PrevSibling.TagParams.caption .PrevSibling.TagParams.counter}}<span class="{{.PrevSibling.TagName}}-counter counter">{{.PrevSibling.TagParams.caption}} {{index .Counters .PrevSibling.TagParams.counter}}:</span> {{end}}{{end}}{{.Content}}</div>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, TextParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, ">", `<blockquote{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</blockquote>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, IndentParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#dl", `<dl{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</dl>`, nil, sectionNormal, []string{}, "", nil, "", nil, nil, nil, IndentParentHood, false, true})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#dt", `<dt{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</dt>`, newString("#dl"), sectionNormal, []string{"#dl"}, "", nil, "", nil, nil, nil, TextParentHood, false, false})
	grammar.addBuiltinTag(&TagDefinition{grammar, "#dd", `<dd{{if .ID}} id="{{.ID}}"{{end}}{{if .Class}} class="{{.Class}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}>{{.Content}}</dd>`, newString("#dl"), sectionNormal, []string{"#dl"}, "", nil, "", nil, nil, nil, IndentParentHood, false, false})
}

func (grammar *Grammar) addCustomTag(tag *TagDefinition) error {
	grammar.Tags = append(grammar.Tags, tag)

	// Check that the default parent exists (if one is specified)
	if tag.DefaultParent != nil {
		if grammar.GetTag(*tag.DefaultParent) == nil {
			return &GrammarError{fmt.Sprintf("Unknown tag named %v is used as default parent for tag %v", *tag.DefaultParent, tag.Name)}
		}
	}
	// Check that all possible parents exist
	for _, t := range tag.PossibleParents {
		if grammar.GetTag(t) == nil {
			return &GrammarError{fmt.Sprintf("Unknown tag named %v is used as possible parent for tag %v", t, tag.Name)}
		}
	}
	// Determine whether the tag can be used in a block-scope directly or via its default parent.
	if tag.DefaultParent == nil && tag.Name != "#root" {
		tag.allowedInParentHood = true
	}

	if tag.ParentHood == DefaultRootParentHood {
		for _, child := range grammar.Tags {
			if (child.DefaultParent != nil && (grammar.currentDefaultRoot == nil || *child.DefaultParent != grammar.currentDefaultRoot.Name)) || child.ParentHood == DefaultRootParentHood || child == tag || child.Name == "#root" {
				continue
			}
			child.DefaultParent = newString(tag.Name)
			child.PossibleParents = append([]string{tag.Name}, child.PossibleParents...)
		}
		grammar.currentDefaultRoot = tag
	}
	if grammar.currentDefaultRoot != nil && tag.DefaultParent == nil && tag.Name != "#root" {
		tag.DefaultParent = newString(grammar.currentDefaultRoot.Name)
	}

	return nil
}

func (grammar *Grammar) addCustomEntity(entity *EntityDefinition) {
	grammar.Entities = append(grammar.Entities, entity)
}

func (grammar *Grammar) addCustomStyle(entity *StyleDefinition) {
	grammar.Styles = append(grammar.Styles, entity)
}

func newString(str string) *string {
	return &str
}
