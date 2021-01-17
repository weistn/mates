package main

import (
	"path/filepath"

	"github.com/spf13/afero"
)

type site struct {
	// This file system is rooted at `path` in the global file system.
	siteFs afero.Fs
	// The directory on the global file system where the site is stored.
	path string
	// Name of the site as specified in site.yaml, or the name of the file in case the site consists of a single file only
	name string
	// The output path in `fs` as specified by site.yaml, or the default "public".
	// Command line arguments can override this value, however.
	outputPath string
	// The content path in `fs` as specified by site.yaml, or the default "content".
	contentPath string
	// Parsed "site.yaml"
	config map[string]interface{}
	ctx    *SiteContext
	tags   *Tags
}

// SiteContext is passed to page templates as .Site context.
type SiteContext struct {
	Title  string
	Author string
	Params map[string]interface{}
	// A list of all pages belonging to the site
	Pages []interface{}
	// Folder of the homepage.
	Folder *FolderContext
	site   *site
}

// FolderContext is passed to page templates as .Folder context.
type FolderContext struct {
	defaultPageType       *pageType
	homepagePageType      *pageType
	folderPageType        *pageType
	categoryPageType      *pageType
	categoryValuePageType *pageType
	Params                map[string]interface{}
	Pages                 []interface{}
	Page                  *PageContext
	Title                 string
	// Title and Name are the same by default.
	// Using markdown it is possible to change the title
	Name string
	// Slash-separated path of the folder on the file system, like "/recipes/beef".
	// The slash-separated path to the content-file generated on the file system, like "/recipes/beef/index.html",
	// can be obtained via the `Page.RelURL` property.
	RelURL string
}

// Tags returns a data structure that describes all tag types, values and associated pages.
func (s *SiteContext) Tags() *Tags {
	return s.site.tags
}

// PagesByType returns all pages which use the specified page type.
func (s *SiteContext) PagesByType(pageTypeName string) []interface{} {
	var list []interface{}
	for _, p := range s.Pages {
		if p.(*PageContext).Type() == pageTypeName {
			list = append(list, p)
		}
	}
	return list
}

// Search for the site.yaml file `path` or one of its parent directories.
// Returns a `site` object with its config already loaded.
func lookupSite(fs afero.Fs, path string) (*site, error) {
	orig := path
	var dir string
	// Loop until site.yaml has been found
	for {
		s, err := fs.Stat(path)
		if err == nil {
			if s.IsDir() {
				// Does a site.yaml file exists in `path`? If yes, it is the site.
				s, err = fs.Stat(filepath.Join(path, "site.yaml"))
				if err == nil {
					// Parse the site.yaml file
					config, err := loadYamlFile(fs, filepath.Join(path, "site.yaml"))
					if err != nil {
						return nil, err
					}
					// Determine the output path
					var outputPath string
					if v, ok := config["Output"]; ok {
						outputPath, err = yamlString("Output", v, "site.yaml")
						if err != nil {
							return nil, err
						}
					} else {
						outputPath = "public"
					}
					// Determine the content path
					var contentPath string
					if v, ok := config["Content"]; ok {
						outputPath, err = yamlString("Content", v, "site.yaml")
						if err != nil {
							return nil, err
						}
					} else {
						contentPath = "content"
					}
					siteFs := afero.NewBasePathFs(fs, path)
					// Create the site context
					ctx := &SiteContext{Params: make(map[string]interface{})}
					tags := newTags()
					// TODO: In untrusted mode, the output must be within the site file system
					site := &site{siteFs: siteFs, ctx: ctx, tags: tags, path: path, name: filepath.Base(orig), config: config, outputPath: outputPath, contentPath: contentPath}
					ctx.site = site
					return site, nil
				}
				if dir == "" {
					dir = path
				}
			}
		}
		if path == "." {
			// No site.yaml has been found.
			// Assume that an empty site.yaml existed at `dir`.
			siteFs := afero.NewBasePathFs(fs, path).(*afero.BasePathFs)
			// Create the site context
			ctx := &SiteContext{Params: make(map[string]interface{})}
			tags := newTags()
			site := &site{siteFs: siteFs, ctx: ctx, tags: tags, path: dir, name: filepath.Base(orig), config: make(map[string]interface{}), outputPath: "public", contentPath: "content"}
			ctx.site = site
			return site, nil
		}
		path = filepath.Dir(path)
	}
}
