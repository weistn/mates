package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/kylelemons/go-gypsy/yaml"
)

const (
	sectionNormal = iota
	sectionMedia
	sectionTable
	sectionCode
	sectionMath
	sectionParagraph
)

// Parser parses markdown files
type Parser struct {
	grammar   *Grammar
	tokenPos  ScannerRange
	token     Token
	tokenStr  string
	s         *Scanner
	counters  map[string]int
	variables map[string]string
	rootNode  *DocumentNode
	resolver  ResourceResolver
}

// NewDocument creates an empty document
func NewDocument(grammar *Grammar) *DocumentNode {
	return &DocumentNode{Tag: "Root", TagDefinition: grammar.GetTag("#root")}
}

// NewParser creates a new parser based on a grammar.
func NewParser(grammar *Grammar, markdown []byte, resolver ResourceResolver) *Parser {
	root := NewDocument(grammar)
	s := NewScanner(markdown)
	return &Parser{grammar: grammar, counters: make(map[string]int), variables: make(map[string]string), rootNode: root, s: s, resolver: resolver}
}

// ParseFrontmatter strips YAML frontmatter from the markdown data and parses it.
func (parser *Parser) ParseFrontmatter() (map[string]interface{}, error) {
	// No YAML?
	if !parser.s.skipString(yamlSeparator) {
		return nil, nil
	}
	// Skip all remaining dashes
	for ; parser.s.ch == '-'; parser.s.next() {
	}
	parser.s.skipWhitespace(false)
	start := parser.s.offset
	// Skip along until the separator is found
	for parser.s.ch != -1 {
		if parser.s.skipString(yamlSeparator) {
			break
		}
		parser.s.next()
	}
	var end int
	if parser.s.ch == -1 {
		end = parser.s.offset
	} else {
		end = parser.s.offset - 4
	}
	// Skip all remaining dashes
	for ; parser.s.ch == '-'; parser.s.next() {
	}
	parser.s.skipWhitespace(true)
	reader := bytes.NewReader(parser.s.src[start:end])
	fmNode, err := yaml.Parse(reader)
	if err != nil {
		return nil, err
	}
	if fmNode != nil {
		if fmMap, ok := fmNode.(yaml.Map); ok {
			return yamlToMap(fmMap), nil
		}
		return nil, fmt.Errorf("Expected a YAML map")
	}
	return nil, nil
}

// ParseMarkdown parses the passed data and returns a document tree.
func (parser *Parser) ParseMarkdown() (*DocumentNode, error) {
	doc, err := parser.parse()
	// Error while scanning?
	if len(parser.s.Errors) > 0 {
		return nil, parser.s.Errors[0]
	}
	// Error while parsing?
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (parser *Parser) next() {
	parser.tokenPos, parser.token, parser.tokenStr = parser.s.Scan()
	//  log.Printf("'%v:%v'\n", parser.token, parser.tokenStr)
}

func (parser *Parser) newDocumentNode(tag string, indent int) *DocumentNode {
	tagdef := parser.grammar.GetTag(tag)
	if tagdef == nil {
		log.Printf("Unknown tag type '%v'", tag)
		tagdef = parser.grammar.GetTag("#p")
	}
	return &DocumentNode{Tag: tag, TagDefinition: tagdef, Document: parser.rootNode, Indent: indent}
}

func (parser *Parser) parse() (doc *DocumentNode, err error) {
	parser.next()

	tagStack := []*TagDefinition{}
	nodeStack := []*DocumentNode{parser.rootNode}
	styleStack := []*StyleNode{}
	secmode := sectionNormal
	// Used to distinguish between being in <thead> or <tbody>
	tableInbody := false

	parseAttributes := func(tag *TagDefinition) (attributes map[string]string) {
		attributes = make(map[string]string)
		for parser.next(); parser.token == TokenValue; parser.next() {
			pos := strings.Index(parser.tokenStr, ":")
			// The colon must not be the first or last character and the value must not be the empty string
			if pos == 0 || len(parser.tokenStr) == 0 || pos == len(parser.tokenStr)-1 {
				continue
			}
			if pos != -1 {
				attributes[parser.tokenStr[:pos]] = parser.tokenStr[pos+1:]
			} else {
				name := parser.tokenStr
				if tag != nil && tag.ClassShortcuts != nil {
					if n, ok := tag.ClassShortcuts[name]; ok {
						name = n
					}
				}
				attributes[name] = ""
			}
		}
		return
	}

	parseConfig := func() (map[string]interface{}, error) {
		if parser.token == TokenConfigText {
			reader := strings.NewReader(parser.tokenStr)
			parser.next()
			configNode, err := yaml.Parse(reader)
			if err != nil {
				return nil, err
			}
			if configNode != nil {
				if configMap, ok := configNode.(yaml.Map); ok {
					return yamlToMap(configMap), nil
				}
				return nil, errors.New("Expected a YAML map")
			}
		}
		return nil, nil
	}

	copyCounter := func(c *DocumentNode) {
		c.Counters = make(map[string]int)
		for k, v := range parser.counters {
			c.Counters[k] = v
		}
	}

	incCounter := func(counter string) {
		if counter == "" {
			return
		}
		c, ok := parser.counters[counter]
		if !ok {
			parser.counters[counter] = 1
		} else {
			parser.counters[counter] = c + 1
		}
	}

	resetCounter := func(resetCounter []string) {
		for _, c := range resetCounter {
			_, ok := parser.counters[c]
			if ok {
				delete(parser.counters, c)
			}
		}
	}

	appendChild := func(parent, child *DocumentNode) {
		child.Parent = parent
		if len(parent.Children) > 0 {
			childPrev := parent.Children[len(parent.Children)-1]
			childPrev.NextSibling = child
			child.PrevSibling = childPrev
		}
		parent.Children = append(parent.Children, child)
	}

	closeStyles := func() {
		for i := len(styleStack) - 1; i >= 0; i-- {
			s := styleStack[i]
			n := s.Name
			if n == "" {
				n = "{"
			}
			log.Printf("Style '%v' is not properly closed", n)
		}
		styleStack = styleStack[:0]
	}

	closeStyle := func(styleName string) {
		if len(styleStack) == 0 {
			log.Printf("Closing style '%v' has no corresponding opening style", styleName)
			return
		}
		s := styleStack[len(styleStack)-1]
		n := s.Name
		if n == "" {
			n = "{"
		}
		switch styleName {
		case "*":
			if styleName != "*" {
				log.Printf("Mismatching closing style '%v'. Corresponding opening style is '%v'", styleName, n)
				return
			}
		case "_":
			if styleName != "_" {
				log.Printf("Mismatching closing style '%v'. Corresponding opening style is '%v'", styleName, n)
				return
			}
		default:
			if styleName != "}" {
				log.Printf("Mismatching closing style '%v'. Corresponding opening style is '%v'", styleName, n)
				return
			}
		}
		styleStack = styleStack[:len(styleStack)-1]
	}

	tagLevel := func(tag *TagDefinition, tag2 *TagDefinition) int {
		for i := len(tagStack) - 1; i >= 0; i-- {
			if tagStack[i] == tag || tagStack[i] == tag2 {
				return i
			}
		}
		return -1
	}

	closeTags := func(level int) {
		closeStyles()
		for len(tagStack) > level {
			tagStack = tagStack[:len(tagStack)-1]
			nodeStack = nodeStack[:len(nodeStack)-1]
		}
		if len(tagStack) == 0 {
			secmode = sectionNormal
		} else {
			secmode = tagStack[len(tagStack)-1].SectionMode
		}
	}

	closeTableCell := func(colspan int) {
		// Close all tags up to the table cell level
		tag := parser.grammar.GetTag("#tbody-cell")
		tag2 := parser.grammar.GetTag("#thead-cell")
		level := tagLevel(tag, tag2)
		if level != -1 {
			nodeStack[level+1].Colspan = colspan
			closeTags(level)
		}
	}

	closeTableRow := func(colspan int) {
		closeTableCell(colspan)
		// Close all tags up to the table row level
		tag := parser.grammar.GetTag("#tbody-row")
		tag2 := parser.grammar.GetTag("#thead-row")
		level := tagLevel(tag, tag2)
		if level != -1 {
			closeTags(level)
		}
		// Now we left the header for sure
		tableInbody = true
	}

	var pushNodeOnStack func(node *DocumentNode)
	pushNodeOnStack = func(node *DocumentNode) {
		parent := nodeStack[len(nodeStack)-1]
		// if node.TagDefinition.FirstChild {
		//	println("SPLIT?", len(parent.Children), parent.TagDefinition.Name)
		//}
		if node.TagDefinition.FirstChild && len(parent.Text)+len(parent.Children) > 0 && parent.TagDefinition.Name != "#root" {
			//println("SPLITTING", parent.TagDefinition.Name)
			// Remove the top most node from the stack
			closeTags(len(tagStack) - 1)
			parent = parser.newDocumentNode(parent.TagDefinition.Name, parent.Indent)
			pushNodeOnStack(parent)
		}
		appendChild(parent, node)
		resetCounter(node.TagDefinition.ResetCounter)
		incCounter(node.TagDefinition.Counter)
		copyCounter(node)
		tagStack = append(tagStack, node.TagDefinition)
		nodeStack = append(nodeStack, node)
	}

	openTag2 := func(tag *TagDefinition, node *DocumentNode, fixParents bool) {
		closeStyles()
		// The new tag has mandatory parents? Then close all tags that are not in line with parser
		// TODO This for loop could be pre-computed to speed-up compilation
		if fixParents {
			defaults := []*TagDefinition{tag}
			for p := tag; p.DefaultParent != nil; {
				if *p.DefaultParent == "#root" {
					break
				}
				p = parser.grammar.GetTag(*p.DefaultParent)
				if p == nil {
					p = parser.grammar.GetTag("#p")
				}
				defaults = append(defaults, p)
			}
			// Close all tags that do not match the possible parents of 'tag'.
			// Consider the possible parents of 'tag' as well as the possible parents of the default parents.
			insertDefaultsDepth := len(defaults) - 1
			for i := len(tagStack) - 1; i >= 0; i-- {
				t := tagStack[i]
				n := nodeStack[i+1]
				found := false
			done:
				for j, def := range defaults {
					if def.allowedInParentHood && (t.ParentHood == DefaultRootParentHood || t.ParentHood == RootParentHood || (t.ParentHood == IndentParentHood && n.Indent < node.Indent)) {
						// println(tag.Name, "PARENT", def.Name, " can be in block scope of", t.Name, parser.s.lineCount)
						insertDefaultsDepth = j
						found = true
						break
					}
					for _, p := range def.PossibleParents {
						if p == t.Name {
							// println(tag.Name, "POSSPARENT", def.Name, " can be a child of", t.Name, parser.s.lineCount)
							insertDefaultsDepth = j
							found = true
							break done
						}
					}
				}
				if found {
					break
				}
				closeTags(i)
			}
			// Need to open some default-parent-tags?
			for i := insertDefaultsDepth; i > 0; i-- {
				def := defaults[i]
				defNode := parser.newDocumentNode(def.Name, node.Indent)
				pushNodeOnStack(defNode)
			}
		}
		// Open the new tag
		pushNodeOnStack(node)
		secmode = tag.SectionMode
	}

	openTag := func(tag *TagDefinition) *DocumentNode {
		node := parser.newDocumentNode(tag.Name, parser.s.indent)
		node.Attributes = parseAttributes(tag)
		openTag2(tag, node, true)
		return node
	}

	openTableCell := func() {
		// Already opened the cell?
		if tagLevel(parser.grammar.GetTag("#tbody-cell"), parser.grammar.GetTag("#thead-cell")) != -1 {
			return
		}

		var tag *TagDefinition
		if tableInbody {
			tag = parser.grammar.GetTag("#tbody-cell")
		} else {
			tag = parser.grammar.GetTag("#thead-cell")
		}
		openTag2(tag, parser.newDocumentNode(tag.Name, parser.s.indent), true)
	}

	isPossibleParent := func(parent *DocumentNode, child *TagDefinition, childIndent int) int {
		depth := 0
		for child != nil {
			if child.allowedInParentHood && parent.TagDefinition.ParentHood == IndentParentHood && parent.Indent < childIndent {
				return depth
			}
			if child.allowedInParentHood && (parent.TagDefinition.ParentHood == DefaultRootParentHood || parent.TagDefinition.ParentHood == RootParentHood) && parent.Indent <= childIndent {
				return depth
			}
			for _, childParent := range child.PossibleParents {
				if childParent == parent.TagDefinition.Name {
					return depth
				}
			}
			depth++
			if child.DefaultParent == nil {
				break
			}
			if *child.DefaultParent == "#root" {
				break
			}
			child = parser.grammar.GetTag(*child.DefaultParent)
		}
		return -1
	}

	/*
		openDefaultParents := func(child *TagDefinition, depth int, indent int) {
			for i := depth; i > 0; i-- {
				def := child
				for j := 0; j < depth; j++ {
					def = parser.grammar.GetTag(*child.DefaultParent)
				}
				defNode := parser.newDocumentNode(def.Name, indent)
				appendChild(nodeStack[len(nodeStack)-1], defNode)
				resetCounter(def.ResetCounter)
				incCounter(def.Counter)
				copyCounter(defNode)
				tagStack = append(tagStack, def)
				nodeStack = append(nodeStack, defNode)
			}
		}
	*/

	yamlString := func(k string, v interface{}) (string, error) {
		if str, ok := v.(string); ok {
			return strings.TrimSpace(str), nil
		}
		return "", errors.New("Expected attribute " + k + " to be a string")
	}

	yamlStrings := func(k string, v interface{}) ([]string, error) {
		var result []string
		if list, ok := v.([]interface{}); ok {
			for _, e := range list {
				if str, ok := e.(string); ok {
					result = append(result, strings.TrimSpace(str))
				} else {
					return nil, errors.New("Expected attribute " + k + " to be a list of strings")
				}
			}
		} else {
			return nil, errors.New("Expected attribute " + k + " to be a list of strings")
		}
		return result, nil
	}

	yamlStringOrStrings := func(k string, v interface{}) ([]string, error) {
		str, err := yamlString(k, v)
		if err == nil {
			return []string{str}, nil
		}
		return yamlStrings(k, v)
	}

	parent := func() NodeWithText {
		if len(styleStack) > 0 {
			return styleStack[len(styleStack)-1]
		}
		return nodeStack[len(nodeStack)-1]
	}

	for parser.token != TokenEOF {
		switch parser.token {
		case TokenSection:
			if len(parser.tokenStr) >= 9 && parser.tokenStr[:9] == "#define:#" { // Parsed "#define:#xxxxxx" ?
				section := parser.tokenStr[8:]
				var text string
				var parents []string
				var counter string
				var resetCounters []string
				var defattr string
				var shortcss map[string]string
				var secmode = sectionNormal
				var resources []*Resource
				var firstChild bool
				parentHood := NoParentHood
				// Do not parse CSS styles. Instead expect YAML config text
				parser.s.mode = modeConfig
				parser.s.layout = layoutCode
				parser.s.skipWhitespace(false)
				parser.next()
				config, err := parseConfig()
				if err != nil {
					return nil, err
				}
				for k, v := range config {
					switch k {
					case "Parents":
						parents, err = yamlStringOrStrings(k, v)
					case "Counter":
						counter, err = yamlString(k, v)
					case "ResetCounters":
						resetCounters, err = yamlStringOrStrings(k, v)
					case "Scripts":
						var js []string
						js, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range js {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeScript, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "Styles":
						var css []string
						css, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range css {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeStyle, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "Resources":
						var css []string
						css, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range css {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeUnknown, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "DefaultAttrib":
						defattr, err = yamlString(k, v)
					case "FirstChild":
						var str string
						str, err = yamlString(k, v)
						if err == nil && str == "true" {
							firstChild = true
						}
					case "Parenthood":
						var bs string
						bs, err = yamlString(k, v)
						if err != nil {
							return nil, err

						}
						switch bs {
						case "none":
							parentHood = NoParentHood
						case "indent":
							parentHood = IndentParentHood
						case "root":
							parentHood = RootParentHood
						case "default-root":
							parentHood = DefaultRootParentHood
						case "text":
							parentHood = TextParentHood
						default:
							return nil, fmt.Errorf("Unknown ParentHood value: %v", bs)
						}
					case "Mode":
						var str string
						str, err = yamlString(k, v)
						if err == nil {
							switch str {
							case "normal":
								secmode = sectionNormal
							case "table":
								secmode = sectionTable
							case "math":
								secmode = sectionMath
							case "code":
								secmode = sectionCode
							case "media":
								secmode = sectionMedia
							case "paragraph":
								secmode = sectionParagraph
							default:
								return nil, fmt.Errorf("%v: Invalid mode attribute '%v' in template section '%v'", parser.tokenPos.FromLine, str, section)
							}
						}
					case "ShortStyles":
						var list []string
						list, err = yamlStringOrStrings(k, v)
						if err == nil {
							shortcss = make(map[string]string)
							for _, kv := range list {
								if idx := strings.Index(kv, "="); idx != -1 {
									k := kv[:idx]
									v := kv[idx+1:]
									shortcss[k] = v
								}
							}
						}
					}
					if err != nil {
						return nil, err
					}
				}
				// if parents == nil && defaultParent != nil {
				//	parents = []string{*defaultParent}
				//}
				// if defaultParent == nil && len(parents) > 0 {
				//	defaultParent = &parents[0]
				//}
				var defaultParent *string
				if len(parents) > 0 {
					defaultParent = &parents[0]
				}
				for ; parser.token == TokenCodeText; parser.next() {
					text += parser.tokenStr
				}
				text = strings.TrimSpace(text)
				text2, htmlResources, err := ParseHTMLResources(text, parser.resolver)
				if err != nil {
					return nil, err
				}
				resources = append(resources, htmlResources...)
				tagDef := &TagDefinition{Name: section, DefaultParent: defaultParent, HTMLTemplate: text2, Counter: counter, ResetCounter: resetCounters, PossibleParents: parents, ClassShortcuts: shortcss, DefaultAttribute: defattr, SectionMode: secmode, Resources: resources, ParentHood: parentHood, FirstChild: firstChild}
				err = parser.grammar.addCustomTag(tagDef)
				if err != nil {
					return nil, err
				}
			} else if len(parser.tokenStr) >= 9 && parser.tokenStr[:9] == "#define:~" { // Parsed "#define:~xxxxxx" ?
				entity := parser.tokenStr[9:]
				// Do not parse CSS styles. Instead expect YAML config text
				parser.s.mode = modeConfig
				parser.s.layout = layoutCode
				parser.s.skipWhitespace(false)
				parser.next()
				config, err := parseConfig()
				if err != nil {
					return nil, err
				}
				var counter string
				var shortcss map[string]string
				var resources []*Resource

				for k, v := range config {
					switch k {
					case "Counter":
						counter, err = yamlString(k, v)
					case "Scripts":
						var js []string
						js, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range js {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeScript, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "Styles":
						var css []string
						css, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range css {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeStyle, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "ShortStyles":
						var list []string
						list, err = yamlStringOrStrings(k, v)
						if err == nil {
							shortcss = make(map[string]string)
							for _, kv := range list {
								if idx := strings.Index(kv, "="); idx != -1 {
									k := kv[:idx]
									v := kv[idx+1:]
									shortcss[k] = v
								}
							}
						}
					}
					if err != nil {
						return nil, err
					}
				}
				var text string
				for ; parser.token == TokenCodeText; parser.next() {
					text += parser.tokenStr
				}
				text = strings.TrimSpace(text)
				text2, htmlResources, err := ParseHTMLResources(text, parser.resolver)
				if err != nil {
					return nil, err
				}
				resources = append(resources, htmlResources...)
				entityDef := &EntityDefinition{Name: entity, HTMLTemplate: text2, Counter: counter, Resources: resources, ClassShortcuts: shortcss}
				parser.grammar.addCustomEntity(entityDef)
			} else if len(parser.tokenStr) >= 9 && parser.tokenStr[:9] == "#define:{" { // Parsed "#define:{xxxxxx" ?
				style := parser.tokenStr[9:]
				// Do not parse CSS styles. Instead expect YAML config text
				parser.s.mode = modeConfig
				parser.s.layout = layoutCode
				parser.s.skipWhitespace(false)
				parser.next()
				config, err := parseConfig()
				if err != nil {
					return nil, err
				}
				var shortcss map[string]string
				var resources []*Resource

				for k, v := range config {
					switch k {
					case "Scripts":
						var js []string
						js, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range js {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeScript, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "Styles":
						var css []string
						css, err = yamlStringOrStrings(k, v)
						if err == nil {
							for _, resourceURL := range css {
								u, err := url.Parse(resourceURL)
								if err != nil {
									return nil, err
								}
								r := &Resource{Type: ResourceTypeStyle, URL: u}
								err = parser.resolver(r)
								if err != nil {
									return nil, err
								}
								resources = append(resources, r)
							}
						}
					case "ShortStyles":
						var list []string
						list, err = yamlStringOrStrings(k, v)
						if err == nil {
							shortcss = make(map[string]string)
							for _, kv := range list {
								if idx := strings.Index(kv, "="); idx != -1 {
									k := kv[:idx]
									v := kv[idx+1:]
									shortcss[k] = v
								}
							}
						}
					}
					if err != nil {
						return nil, err
					}
				}
				var text string
				for ; parser.token == TokenCodeText; parser.next() {
					text += parser.tokenStr
				}
				text = strings.TrimSpace(text)
				text2, htmlResources, err := ParseHTMLResources(text, parser.resolver)
				if err != nil {
					return nil, err
				}
				resources = append(resources, htmlResources...)
				styleDef := &StyleDefinition{Name: style, HTMLTemplate: text2, Resources: resources, ClassShortcuts: shortcss}
				parser.grammar.addCustomStyle(styleDef)
			} else if len(parser.tokenStr) >= 8 && parser.tokenStr[:8] == "#define:" {
				return nil, fmt.Errorf("#define can only define tags (starting with #), entities (starting with ~) and styles (starting with {)")
			} else {
				tagname := parser.tokenStr
				pos := strings.Index(tagname, ":")
				var v string
				if pos != -1 {
					v = tagname[pos+1:]
					tagname = tagname[:pos]
				}
				tag := parser.grammar.GetTag(tagname)
				if tag == nil {
					log.Printf("%v: Unknown tag: '%v'", parser.tokenPos.FromLine, tagname)
					tag = parser.grammar.GetTag("#p")
				}
				if tag.SectionMode == sectionTable {
					parser.s.layout = layoutTable
				} else if tag.SectionMode == sectionCode {
					parser.s.layout = layoutCode
				} else if tag.SectionMode == sectionMath {
					parser.s.layout = layoutMath
				} else if tag.SectionMode == sectionParagraph {
					parser.s.layout = layoutParagraph
				}
				node := openTag(tag)
				if pos != -1 {
					if tag.DefaultAttribute == "" {
						log.Printf("The tag %v has no default attribute", tagname)
					} else {
						node.Attributes[tag.DefaultAttribute] = v
					}
				}
				if tag.SectionMode == sectionTable {
					tableInbody = false
				}
			}
		case TokenEnum:
			indent := parser.s.indent
			//			println("INDENT", indent)
			tag := parser.grammar.GetTag("#li")
			var enumTag *TagDefinition
			if parser.tokenStr == "-" {
				enumTag = parser.grammar.GetTag("#ul")
			} else {
				enumTag = parser.grammar.GetTag("#ol")
			}
			// println("ENUM, default parent", *enumTag.DefaultParent)
			// Find a possible sibling #li, parent #ol/#ul, or a parent with block scope
			parentEnum := -1
			siblingEnum := -1
			depth := -1
			for i := len(tagStack) - 1; i >= 0; i-- {
				t := tagStack[i]
				n := nodeStack[i+1]
				// println("CHECK", t.Name)
				depth = isPossibleParent(n, enumTag, indent)
				if depth != -1 {
					// println("   Parent", i, n.Indent)
					parentEnum = i
					break
				}
				if n.Indent >= indent && (t.Name == "#ul" || t.Name == "#ol") {
					siblingEnum = i
					// println("   Sibling", i)
				}
				// if t.Name != "#ul" && t.Name != "#ol" && t.ParentHood == NoParentHood {
				//	continue
				//}
				/*
					if (t.ParentHood == IndentParentHood && n.Indent < indent) || t.ParentHood == DefaultRootParentHood || t.ParentHood == RootParentHood {
						parentEnum = i
						break
					}
					if n.Indent >= indent && (t.Name == "#ul" || t.Name == "#ol") {
						siblingEnum = i
					}
				*/
			}
			// Open #ul or #ol or find an appropriate one.
			if siblingEnum != -1 {
				if tagStack[siblingEnum] == enumTag {
					//					println("SIB1", siblingEnum)
					closeTags(siblingEnum + 1)
					nodeStack[siblingEnum+1].Indent = indent
				} else {
					//					println("SIB2", siblingEnum)
					closeTags(siblingEnum)
					//					openDefaultParents(enumTag, depth, indent)
					openTag2(enumTag, parser.newDocumentNode(enumTag.Name, indent), true)
				}
			} else if parentEnum != -1 {
				// println("PAR", parentEnum, tagStack[parentEnum].Name)
				closeTags(parentEnum + 1)
				//				openDefaultParents(enumTag, depth, indent)
				openTag2(enumTag, parser.newDocumentNode(enumTag.Name, indent), true)
			} else {
				//				println("NEW", len(nodeStack), len(tagStack))
				closeTags(0)
				//				openDefaultParents(enumTag, depth, indent)
				openTag2(enumTag, parser.newDocumentNode(enumTag.Name, indent), true)
			}
			// #li
			node := parser.newDocumentNode(tag.Name, indent)
			node.Attributes = parseAttributes(tag)
			openTag2(tag, node, false)
			//			println("STACK", len(tagStack))
			//			parser.next()
		case TokenText:
			if secmode == sectionTable {
				openTableCell()
			}
			if secmode != sectionMedia { // Ignore text in a media context
				p := parent()
				// The parent cannot contain text? Open a #p and add the text there
				if node, ok := p.(*DocumentNode); ok && node.TagDefinition.ParentHood != TextParentHood {
					if len(strings.TrimSpace(parser.tokenStr)) == 0 {
						parser.next()
						continue
					}
					// println("REPARENT text for", node.TagDefinition.Name, node.Indent, parser.s.indent)
					parag := parser.newDocumentNode("#p", node.Indent+1)
					openTag2(parag.TagDefinition, parag, true)
					p = parag
				}
				text := &TextNode{Text: parser.tokenStr, Parent: p}
				p.AppendText(text)
				// Replace a #p with a #dl tag if the text ends with ":"
				if parag, ok := p.(*DocumentNode); ok && parag.TagDefinition.Name == "#p" {
					if tagStack[len(tagStack)-1].SectionMode == sectionParagraph && len(text.Text) > 0 && text.Text[len(text.Text)-1] == ':' {
						closeTags(len(tagStack) - 1)
						parag.Parent.RemoveChild(parag)
						// This will implicitly open a #dl if none exists so far
						dlNode := parser.newDocumentNode("#dt", parag.Indent)
						dlNode.Text = parag.Text
						openTag2(dlNode.TagDefinition, dlNode, true)
						closeTags(len(tagStack) - 1)
						ddNode := parser.newDocumentNode("#dd", parag.Indent)
						ddNode.Text = parag.Text
						openTag2(ddNode.TagDefinition, ddNode, true)
						// t := parser.grammar.GetTag("#def")
						// tagStack[len(tagStack)-1] = t
						// nodeStack[len(nodeStack)-1].Tag = t.Name
						// nodeStack[len(nodeStack)-1].TagDefinition = t
					}
				}
			}
			parser.next()
		case TokenMathText:
			if secmode == sectionTable {
				openTableCell()
			}
			if secmode != sectionMedia { // Ignore text in a media context
				p := parent()
				text := &MathNode{Text: parser.tokenStr, Parent: p}
				p.AppendText(text)
			}
			parser.next()
		case TokenCodeText:
			if secmode == sectionTable {
				openTableCell()
			}
			if secmode != sectionMedia { // Ignore text in a media context
				p := parent()
				text := &CodeNode{Text: parser.tokenStr, Parent: p}
				p.AppendText(text)
			}
			parser.next()
		case TokenStyle:
			if secmode == sectionTable {
				openTableCell()
			}
			if parser.tokenStr == "}" {
				closeStyle("}")
				parser.next()
				break
			}
			p := parent()
			if parser.tokenStr == "*" {
				parser.next()
				if len(styleStack) > 0 && styleStack[len(styleStack)-1].Name == "*" {
					closeStyle("*")
					break
				}
				node := &StyleNode{Name: "*", Parent: p, Attributes: make(map[string]string), StyleDefinition: nil}
				p.AppendText(node)
				styleStack = append(styleStack, node)
				break
			}
			if parser.tokenStr == "_" {
				parser.next()
				if len(styleStack) > 0 && styleStack[len(styleStack)-1].Name == "_" {
					closeStyle("_")
					break
				}
				node := &StyleNode{Name: "_", Parent: p, Attributes: make(map[string]string), StyleDefinition: nil}
				p.AppendText(node)
				styleStack = append(styleStack, node)
				break
			}
			parser.next()
			if parser.token != TokenValue {
				log.Print("Expected style specificiation after {")
			} else {
				name := parser.tokenStr
				var value string
				if pos := strings.Index(name, ":"); pos != -1 {
					value = name[pos+1:]
					name = name[:pos]
				}
				var node *StyleNode
				styledef := parser.grammar.GetStyle(name)
				if styledef == nil {
					node = &StyleNode{Name: "-css-", Parent: p, Attributes: make(map[string]string), StyleDefinition: nil}
					node.Attributes[name] = value
				} else {
					node = &StyleNode{Name: name, Value: value, Parent: p, Attributes: make(map[string]string), StyleDefinition: styledef}
				}
				p.AppendText(node)
				for parser.next(); parser.token == TokenValue; parser.next() {
					if pos := strings.Index(parser.tokenStr, ":"); pos != -1 {
						node.Attributes[parser.tokenStr[:pos]] = parser.tokenStr[pos+1:]
					} else {
						node.Attributes[parser.tokenStr] = ""
					}
				}
				styleStack = append(styleStack, node)
			}
		case TokenEntity:
			if secmode == sectionTable {
				openTableCell()
			}
			name := parser.tokenStr
			var value string
			if pos := strings.Index(name, ":"); pos != -1 {
				value = name[pos+1:]
				name = name[:pos]
			}
			attribs := parseAttributes(nil)
			edef := parser.grammar.GetEntity(name)
			if edef == nil {
				log.Printf("%v:%v: Unknown entity: '%v'", parser.s.lineCount, parser.s.lineOffset, name)
			} else {
				p := parent()
				node := &EntityNode{Parent: p, Attributes: attribs, Name: name, EntityDefinition: edef, Value: value}
				incCounter(edef.Counter)
				node.Counters = make(map[string]int)
				for k, v := range parser.counters {
					node.Counters[k] = v
				}
				p.AppendText(node)
			}
		case TokenConfigText:
			/*
				reader := strings.NewReader(parser.tokenStr)
				configNode, err := yaml.Parse(reader)
				if err != nil {
					log.Printf("Error parsing yaml: %v", err)
				}
				if configNode != nil {
					if configMap, ok := configNode.(yaml.Map); ok {
						m := yamlToMap(configMap)
						if len(tagStack) == 0 {
							log.Printf("Additional YAML outside of a parent tag is ignored")
						} else {
							// println("TAG CONFIG '" + tagStack[len(tagStack)-1].Name + "'" + parser.tokenStr)
							nodeStack[len(nodeStack)-1].Config = m
						}
					} else {
						return nil, fmt.Errorf("Expected a YAML map")
					}
				}
			*/
			// Ignore, should not happen anyway
			parser.next()
		case TokenTableCell:
			closeTableCell(len(parser.tokenStr))
			openTableCell()
			parser.next()
		case TokenTableRow:
			closeTableRow(len(parser.tokenStr))
			parser.next()
			/*		case TokenVar:
					splits := strings.Split(parser.tokenStr, "=")
					if len(splits) > 1 {
						// Remove all spaces
						varname := strings.Replace(splits[0], " ", "", -1)
						// Match variable for validity
						matched, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", varname)
						if !matched {
							log.Printf("Parser: Found illegal variable %s to be set\n", splits[0])
							parser.next()
							continue
						}
						parser.variables[varname] = strings.Trim(splits[1], " ")
					} else {
						var variable = strings.Replace(parser.tokenStr, " ", "", -1)
						// Match variable for validity
						matched, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", variable)
						if !matched {
							log.Printf("Parser: Found illegal variable %s to be outputted\n", splits[0])
							parser.next()
							continue
						}
						// Output value of variable
						var value = parser.variables[variable]
						parent := nodeStack[len(nodeStack)-1]
						text := &TextNode{Text: value, Parent: parent}
						parent.Text = append(parent.Text, text)
					}
					parser.next() */
		}
	}
	closeTags(0)

	return parser.rootNode, nil
}

func removeBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

var srcRegex *regexp.Regexp

// ParseHTMLResources searches for occureneces of e.g. src="url".
// For the specified URL, the function resolves the sourceURL relative to sourceBaseURL.
// Then it computes the destination address relative to destBaseURL and replaces the URL in the text.
// The modified text is returned.
func ParseHTMLResources(data string, resolver ResourceResolver) (string, []*Resource, error) {
	if srcRegex == nil {
		srcRegex = regexp.MustCompile("\\ssrc\\s*=\\s*\"[^\"{}]*\"")
	}
	matches := srcRegex.FindAllStringIndex(data, -1)
	if len(matches) == 0 {
		return data, nil, nil
	}
	var resources []*Resource
	var newdata string
	pos := 0
	for _, match := range matches {
		i := match[0]
		for ; data[i] != '"'; i++ {
		}
		i++
		str := html.UnescapeString(string(data[i : match[1]-1]))
		u, err := url.Parse(str)
		if err != nil {
			return "", nil, fmt.Errorf("Malformed URL in HTML: %v", err)
		}
		if u.IsAbs() {
			continue
		}
		if i != pos {
			newdata += data[pos:i]
		}
		pos = match[1] - 1
		r := &Resource{Type: ResourceTypeUnknown, URL: u}
		err = resolver(r)
		resources = append(resources, r)
		newdata += html.EscapeString(r.URL.String())
	}
	newdata += data[pos:]
	return newdata, resources, nil
}
