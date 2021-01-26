package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

type bundle struct {
	name string
	// path to the bundle in the global file system
	path string
	// File system that has access to files of the bundle only
	bundleFs afero.Fs
	// Parsed "bundle.yaml"
	config                map[string]interface{}
	b                     *Builder
	defaultPageType       *pageType
	homepagePageType      *pageType
	folderPageType        *pageType
	categoryPageType      *pageType
	categoryValuePageType *pageType
	// This data structure is only used to scan bundle.yaml and it is not fully populated.
	// Use the `Tags` structure attached to `site` instead.
	tags    *Tags
	varDefs map[string]*VarDef
}

func newBundle(name string, path string, fs afero.Fs, b *Builder) *bundle {
	return &bundle{name: name, path: path, bundleFs: fs, b: b, tags: newTags()}
}

func (bndl *bundle) load() error {
	// Parse the site.yaml file (if it exists)
	var err error
	bndl.config, err = loadYamlFile(bndl.bundleFs, "bundle.yaml")
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	for k, v := range bndl.config {
		switch k {
		case "Page":
			pageTypeName, err := yamlString(k, v, "bundle.yaml")
			if err != nil {
				break
			}
			bndl.defaultPageType, err = bndl.b.lookupPageType(pageTypeName)
			if err != nil {
				return err
			}
		case "Homepage":
			pageTypeName, err := yamlString(k, v, "bundle.yaml")
			if err != nil {
				break
			}
			bndl.homepagePageType, err = bndl.b.lookupPageType(pageTypeName)
			if err != nil {
				return err
			}
		case "Folder":
			pageTypeName, err := yamlString(k, v, "bundle.yaml")
			if err != nil {
				break
			}
			bndl.folderPageType, err = bndl.b.lookupPageType(pageTypeName)
			if err != nil {
				return err
			}
		case "TagType":
			pageTypeName, err := yamlString(k, v, "bundle.yaml")
			if err != nil {
				break
			}
			bndl.categoryPageType, err = bndl.b.lookupPageType(pageTypeName)
			if err != nil {
				return err
			}
		case "TagValue":
			pageTypeName, err := yamlString(k, v, "bundle.yaml")
			if err != nil {
				break
			}
			bndl.categoryValuePageType, err = bndl.b.lookupPageType(pageTypeName)
			if err != nil {
				return err
			}
		case "Tags":
			err = bndl.tags.addFromYaml(v, "bundle.yaml")
			if err != nil {
				return err
			}
		case "Vars":
			bndl.varDefs, err = yamlToVarDefs(v, filepath.Join(bndl.path, "bundle.yaml"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
