package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

type pageTypeKind int

const (
	normalPageType        pageTypeKind = 1
	homepagePageType      pageTypeKind = 2
	folderPageType        pageTypeKind = 3
	categoryPageType      pageTypeKind = 4
	categoryValuePageType pageTypeKind = 5
)

type pageType struct {
	name string
	// Path of the page-type.
	// `path` is relative to `fs`.
	path string
	// The file system that contains the page type files and all resources it can address.
	fs afero.Fs
	// The bundle in which the page type has been defined (or nil if the page type has been defined by the site).
	bundle *bundle
	// A page type can inherit all settings from a parent page type. May be empty.
	inheritPageTypeName string
	// The parent page type or nil.
	// A cache for speed-up.
	inheritPageType *pageType
	// A list of resources required by the page, such as CSS, JS, images etc.
	resources []*Resource
}

// Default HTML tempalte to use for a page in case nothing else has been specified.
var defaultBaseHTML = `<!doctype html>
<html>
	<!-- Default template -->
	<head>
	<meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
	{{block "css" .}}{{.Styles}}{{end}}
	{{block "js" .}}{{.Scripts}}{{end}}
	<title>{{ block "title" . }}{{ .Site.Title }}{{ end }}</title>
	</head>
	<body>
		{{block "main" .}}{{.Content}}{{end}}
	</body>
</html>
`

func newNonePageType() *pageType {
	return &pageType{name: "__none__"}
}

func newDefaultPageType() *pageType {
	return &pageType{name: "__default__"}
}

func newDefaultFolderPageType() *pageType {
	return &pageType{name: "__default_folder__"}
}

func newDefaultCategoryPageType() *pageType {
	return &pageType{name: "__default_cat__"}
}

func newDefaultCategoryValuePageType() *pageType {
	return &pageType{name: "__default_catval__"}
}

// Load "page.yaml" and lookup all resources.
func newPageType(fs afero.Fs, path string, name string, bundle *bundle, b *Builder) (p *pageType, err error) {
	p = &pageType{fs: fs, path: path, name: name, bundle: bundle}
	// Parse the page.yaml file (if it exists)
	configFilePath := filepath.Join(p.path, "page.yaml")
	yamlFile, err := loadYamlFile(p.fs, configFilePath)
	if os.IsNotExist(err) {
		return p, nil
	} else if err != nil {
		return nil, err
	}

	// Search for a parent page type (if one has been specified).
	// This must happen before resources are looked up
	for k, v := range yamlFile {
		switch k {
		case "Inherit":
			p.inheritPageTypeName, err = yamlString(k, v, configFilePath)
			if err != nil {
				break
			}
			p.inheritPageType, err = b.lookupPageType(p.inheritPageTypeName)
			if err != nil {
				return nil, err
			}
		}
	}

	// Lookup all resources
	for k, v := range yamlFile {
		switch k {
		case "Inherit":
			// Do nothing by intention
		case "Scripts":
			fallthrough
		case "Styles":
			fallthrough
		case "Resources":
			list, err := yamlStringOrStrings(k, v, configFilePath)
			if err != nil {
				break
			}
			for _, strurl := range list {
				u, err := url.Parse(strurl)
				if err != nil {
					return nil, err
				}
				var r *Resource
				if k == "Scripts" {
					r = &Resource{Type: ResourceTypeScript, URL: u}
				} else if k == "Styles" {
					r = &Resource{Type: ResourceTypeStyle, URL: u}
				} else {
					r = &Resource{Type: ResourceTypeUnknown, URL: u}
				}
				err = p.resolveStaticResource(r, p)
				if err != nil {
					return nil, err
				}
				p.resources = append(p.resources, r)
			}
		default:
			log.Printf("Unknown attribute %v in page %v", k, configFilePath)
		}
	}
	return p, nil
}

func (p *pageType) isNone() bool {
	return p.name == "__none__"
}

func (p *pageType) isDefault() bool {
	return p.name == "__default__" || p.name == "__default_folder__" || p.name == "__default_cat__" || p.name == "__default_catval__"
}

func (p *pageType) getMarkdownSyntax() ([]byte, string, error) {
	if p.isNone() || p.isDefault() {
		return nil, "", nil
	}
	path := filepath.Join(p.path, "syntax.md")
	data, err := afero.ReadFile(p.fs, path)
	if err != nil {
		return nil, "", err
	}
	// TODO: Return the real path instead of path
	return data, path, nil
}

func (p *pageType) getBaseHTML() (string, string, error) {
	if p.isNone() {
		return "", "", os.ErrNotExist
	}
	// if p.name == "__default__" || p.name == "__default_folder__" {
	if p.isDefault() {
		return defaultBaseHTML, "__builtin__", nil
	}
	path := filepath.Join(p.path, "base.html")
	data, err := afero.ReadFile(p.fs, path)
	if err != nil {
		return "", "", err
	}
	// TODO: Return the real path instead of path
	return string(data), path, nil
}

func (p *pageType) getLayoutHTML() (string, string, error) {
	if p.isNone() || p.isDefault() {
		return "", "", nil
	}
	path := filepath.Join(p.path, "layout.html")
	data, err := afero.ReadFile(p.fs, path)
	if err != nil {
		return "", "", err
	}
	// TODO: Return the real path instead of path
	return string(data), path, nil
}

// resolveStaticResource searches for the file mentioned in the resource and determines
// the source and destination path, i.e. where the resource file can be found and
// where it should be copied to.
func (p *pageType) resolveStaticResource(res *Resource, onBehalfOf *pageType) error {
	// Already done?
	if res.Resolved {
		return nil
	}
	// Nothing to do for absolute URLs. We will just use the URL and copy no files.
	if res.URL.IsAbs() {
		return nil
	}
	// The resource URL path is relative to "/". If not, force it.
	resPath := filepath.Clean(filepath.Join(string(filepath.Separator), filepath.FromSlash(res.URL.Path)))
	// Strip the first character of `p`, because it is now a slash.
	resPath = resPath[1:]
	// If the page is not part of a bundle, then the resource must be inside the page type directory.
	if p.bundle == nil {
		// Search in `p.path`/static/`resPath`
		searchFile := filepath.Join(p.path, "static", resPath)
		_, err := p.fs.Stat(searchFile)
		if os.IsNotExist(err) && p.inheritPageType != nil {
			// Perhaps the resource has been inherited from a parent page type?
			return p.inheritPageType.resolveStaticResource(res, onBehalfOf)
		} else if err != nil {
			// Resource file is missing
			return fmt.Errorf("Missing resource file %v in page type %v", res.URL.String(), onBehalfOf.name)
		}
		res.SourcePath = searchFile
		res.SourceFs = p.fs
		res.DestPath = filepath.Join(string(filepath.Separator), "_static", "pages", p.name, searchFile)
		res.URL.Path = filepath.ToSlash(res.DestPath)
		res.Resolved = true
		return nil
	}

	// In the bundle, search for `p.path`/static/`resPath`
	searchFile := filepath.Join(p.path, "static", resPath)
	_, err := p.bundle.bundleFs.Stat(searchFile)
	if err == nil {
		res.SourcePath = searchFile
		res.SourceFs = p.bundle.bundleFs
		res.DestPath = filepath.Join(string(filepath.Separator), "_static", "bundles", p.bundle.name, "pages", p.name, searchFile)
		res.URL.Path = filepath.ToSlash(res.DestPath)
		res.Resolved = true
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// In the bundle, search in /static/`resPath`
	searchFile = filepath.Join("static", resPath)
	_, err = p.bundle.bundleFs.Stat(searchFile)
	if err == nil {
		res.SourcePath = searchFile
		res.SourceFs = p.bundle.bundleFs
		res.DestPath = filepath.Join(string(filepath.Separator), "_static", "bundles", p.bundle.name, searchFile)
		res.URL.Path = filepath.ToSlash(res.DestPath)
		res.Resolved = true
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// Search the resource in the parent page (of one exists)
	if p.inheritPageType != nil {
		return p.inheritPageType.resolveStaticResource(res, onBehalfOf)
	}
	return fmt.Errorf("Missing resource file %v in page type %v", res.URL.String(), onBehalfOf.name)
}
