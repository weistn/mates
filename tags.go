package main

import (
	"fmt"
)

// Tags holds information about tags.
type Tags struct {
	Types map[string]*TagType
}

// TagType describes a tag type and all of its (possible or existing) values
type TagType struct {
	Name              string
	Title             string
	Values            map[string]*TagValue
	Folder            *FolderContext
	pageTypeName      string
	pageType          *pageType
	valuePageTypeName string
	valuePageType     *pageType
}

// TagValue describes a tag value and all pages that are tagged with this value.
type TagValue struct {
	Name string
	// Pages  []*PageContext
	// All pages that are tagged with this tag type and tag value.
	Pages []interface{}
	// The page generated for the tag value.
	Page *PageContext
}

func newTags() *Tags {
	return &Tags{Types: make(map[string]*TagType)}
}

// Type returns the tag type page for `tagType`.
// The string must match the Title or Name of the TagType.
func (t *Tags) Type(tagType string) *TagType {
	for _, typ := range t.Types {
		if typ.Title == tagType {
			return typ
		}
		if typ.Name == tagType {
			return typ
		}
	}
	return nil
}

func (t *Tags) cloneFrom(t2 *Tags) {
	for _, tt2 := range t2.Types {
		tt, ok := t.Types[tt2.Name]
		if !ok {
			tt = t.getOrCreateType(tt2.Name)
			tt.pageType = tt2.pageType
			tt.valuePageType = tt2.pageType
			tt.pageTypeName = tt2.pageTypeName
			tt.valuePageTypeName = tt2.valuePageTypeName
			tt.Folder = tt2.Folder
		}
		for _, tv2 := range tt2.Values {
			tv, ok := tt.Values[tv2.Name]
			if !ok {
				tv = tt.getOrCreateValue(tv2.Name)
				tv.Page = tv2.Page
			}
		}
	}
}

func (t *Tags) addFromYaml(y interface{}, filename string) error {
	// The YAML is a list of tag types.
	// Each tag type is either just a string or a mapping
	list, ok := y.([]interface{})
	if !ok {
		return fmt.Errorf("%v Tags: expected a list", filename)
	}
	for _, ytag := range list {
		var tt *TagType
		if tagName, ok := ytag.(string); ok {
			tt = t.getOrCreateType(tagName)
		} else if mtag, ok := ytag.(map[string]interface{}); ok {
			for tagName, v := range mtag {
				println("FOO", tagName)
				tt = t.getOrCreateType(tagName)
				if values, ok := v.(map[string]interface{}); ok {
					for prop, value := range values {
						println("BAR", prop)
						var err error
						switch prop {
						case "TagType":
							tt.pageTypeName, err = yamlString(prop, value, filename)
							if err != nil {
								return fmt.Errorf("%v Tags: %v: %v: expected a string: %v", filename, tagName, prop, err)
							}
							println("TT", tt.pageTypeName)
						case "TagValue":
							tt.valuePageTypeName, err = yamlString(prop, value, filename)
							if err != nil {
								return fmt.Errorf("%v Tags: %v: %v: expected a string: %v", filename, tagName, prop, err)
							}
							println("TT", tt.valuePageTypeName)
						default:
							return fmt.Errorf("%v Tags: %v: %v: unknown tag", filename, tagName, prop)
						}
					}
				} else {
					return fmt.Errorf("%v Tags: %v: a tag must be a string or mapping", filename, tagName)
				}
				// Take the first entry only
				break
			}
		} else {
			return fmt.Errorf("%v Tags: a tag must be a string or mapping", filename)
		}
	}
	return nil
}

func (t *Tags) loadPageTypes(b *Builder) error {
	var err error
	for _, tt := range t.Types {
		if tt.pageTypeName == "" {
			println("!default tt")
			tt.pageType = b.categoryPageType
		} else {
			println("lookup", tt.pageTypeName)
			tt.pageType, err = b.lookupPageType(tt.pageTypeName)
			if err != nil {
				return fmt.Errorf("In Tags: %v TagType: %v", tt.Name, err.Error())
			}
		}
		if tt.valuePageTypeName == "" {
			println("!default tv")
			tt.valuePageType = b.categoryValuePageType
		} else {
			println("lookup v", tt.valuePageTypeName)
			tt.valuePageType, err = b.lookupPageType(tt.valuePageTypeName)
			if err != nil {
				return fmt.Errorf("In Tags: %v TagValue: %v", tt.Name, err.Error())
			}
		}
	}
	return nil
}

func (t *Tags) getOrCreateType(name string) *TagType {
	if typ, ok := t.Types[name]; ok {
		return typ
	}
	typ := newTagType(name)
	t.Types[name] = typ
	return typ
}

func newTagType(name string) *TagType {
	return &TagType{Values: make(map[string]*TagValue), Name: name, Title: name}
}

func (t *TagType) getOrCreateValue(name string) *TagValue {
	if v, ok := t.Values[name]; ok {
		return v
	}
	v := &TagValue{Name: name}
	t.Values[name] = v
	return v
}

func (t *TagType) addPage(name string, page *PageContext) {
	v := t.getOrCreateValue(name)
	v.Pages = append(v.Pages, page)
}
