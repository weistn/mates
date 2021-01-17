package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/afero"
)

// ResourceType defines the type of resource.
type ResourceType int

const (
	// ResourceTypeStyle determines that the resource is CSS
	ResourceTypeStyle ResourceType = iota
	// ResourceTypeScript determines that the resource is JavaScript
	ResourceTypeScript
	// ResourceTypeUnknown means that the type of resource is not known.
	// It could be an image, video, etc.
	ResourceTypeUnknown
)

// Resource represents a JS script or CSS file required by the generated HTML.
type Resource struct {
	SourceFs afero.Fs
	// The path is relative to SourceFs
	SourcePath string
	DestPath   string
	// The URL as specified in markdown or as used in HTML templates
	URL  *url.URL
	Type ResourceType
	// Resolved determines whether a ResourceResolver did already resolve the SourcePath and DestPath.
	Resolved bool
}

// ResourceResolver computes the SourcePath and DestPath of a component where applicable.
// It may as well rewrite the URL of the resource.
type ResourceResolver func(res *Resource) error

// ToHTMLLink returns the resource link into HTML document
func (res *Resource) ToHTMLLink() string {
	switch res.Type {
	case ResourceTypeStyle:
		return fmt.Sprintf("<link rel=\"stylesheet\" href=\"%v\" type=\"text/css\">\n", res.URL.String())
	case ResourceTypeScript:
		return fmt.Sprintf("<script type=\"text/javascript\" src=\"%v\"></script>\n", res.URL.String())
	case ResourceTypeUnknown:
		// Do nothing by intention
		return ""
	}
	panic("Programmer stupid")
}

// UniqueID returns an ID that exists only once for each resource.
// The use case of this ID is to deduplicate resources.
func (res *Resource) UniqueID() string {
	if res.URL.IsAbs() || res.DestPath == "" {
		return res.URL.String()
	}
	return res.DestPath
}
