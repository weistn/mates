package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"vs.uni-due.de/weis/mates"

	"github.com/spf13/afero"
)

// Builder runs the process of parsing all files and generating the output
type Builder struct {
	// File system to use
	fs afero.Fs
	// Paths in which to look for bundles and page types.
	// This list is computed from `options.searchPath`.
	searchPaths []string
	outputFs    afero.Fs
	contentFs   afero.Fs
	// The site to build
	site       *site
	generators map[string]*mates.HTMLGenerator
	options    *options
	// A cache of folderContext objects
	folderContext map[string]*FolderContext
	pageTypes     map[string]*pageType
	copied        map[string]bool
	// May be nil
	bundle                *bundle
	defaultPageType       *pageType
	homepagePageType      *pageType
	folderPageType        *pageType
	categoryPageType      *pageType
	categoryValuePageType *pageType
}

func newBuilder(options *options) (*Builder, error) {
	// Create the builder
	pageTypes := make(map[string]*pageType)
	folderContext := make(map[string]*FolderContext)
	b := &Builder{generators: make(map[string]*mates.HTMLGenerator), options: options, pageTypes: pageTypes, folderContext: folderContext, copied: make(map[string]bool)}

	// The file system to use for the build.
	// Input and output files are located here.
	b.fs = afero.NewOsFs()

	// The "none" page type always exists.
	b.pageTypes["none"] = newNonePageType()

	// Determine the location of the site and load its config (if there is any)
	var err error
	b.site, err = lookupSite(b.fs, options.buildPath)
	if err != nil {
		return nil, err
	}

	// Lookup the bundle name in site.yaml, otherwise look for the "default" bundle.
	// It is ok, that the default bundle is missing.
	if v, ok := b.site.config["Bundle"]; ok {
		bndlName, err := yamlString("Bundle", v, "site.yaml")
		if err != nil {
			return nil, err
		}
		err = b.lookupBundle(bndlName)
		if err != nil {
			return nil, err
		}
	} else {
		err = b.lookupBundle("default")
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Determine the output directory. Command line arguments override site settings.
	if options.outputPath != "" {
		b.outputFs = afero.NewBasePathFs(b.fs, options.outputPath)
	} else {
		b.outputFs = afero.NewBasePathFs(b.site.siteFs, b.site.outputPath)
	}

	b.contentFs = afero.NewBasePathFs(b.site.siteFs, b.site.contentPath)

	// Initialize page types as specified by bundle.yaml.
	if b.bundle != nil {
		b.defaultPageType = b.bundle.defaultPageType
		b.homepagePageType = b.bundle.homepagePageType
		b.folderPageType = b.bundle.folderPageType
		b.categoryPageType = b.bundle.categoryPageType
		b.categoryValuePageType = b.bundle.categoryValuePageType

		b.site.tags.cloneFrom(b.bundle.tags)
	}

	// Apply all settings of site.yaml.
	// site.yaml overrides bundle.yaml.
	for k, v := range b.site.config {
		switch k {
		case "Author":
			b.site.ctx.Author, err = yamlString(k, v, "site.yaml")
		case "Title":
			b.site.ctx.Title, err = yamlString(k, v, "site.yaml")
		case "Bundle":
			// Ignore, because it has been handled before.
			break
		case "Output":
			// Ignore, because it has been handled before.
			break
		case "Page":
			pageTypeName, err := yamlString(k, v, "site.yaml")
			if err != nil {
				break
			}
			b.defaultPageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, fmt.Errorf("In site.yaml Page: %v", err.Error())
			}
		case "Homepage":
			pageTypeName, err := yamlString(k, v, "site.yaml")
			if err != nil {
				break
			}
			b.homepagePageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, fmt.Errorf("In site.yaml Homepage: %v", err.Error())
			}
		case "Folder":
			pageTypeName, err := yamlString(k, v, "site.yaml")
			if err != nil {
				break
			}
			b.folderPageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, fmt.Errorf("In site.yaml Folder: %v", err.Error())
			}
		case "TagType":
			pageTypeName, err := yamlString(k, v, "site.yaml")
			if err != nil {
				break
			}
			b.categoryPageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, fmt.Errorf("In site.yaml TagType: %v", err.Error())
			}
		case "TagValue":
			pageTypeName, err := yamlString(k, v, "site.yaml")
			if err != nil {
				break
			}
			b.categoryValuePageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, fmt.Errorf("In site.yaml TagValue: %v", err.Error())
			}
		case "Tags":
			err = b.site.tags.addFromYaml(v, "site.yaml")
			if err != nil {
				return nil, err
			}
		default:
			b.site.ctx.Params[k] = v
		}
		if err != nil {
			return nil, err
		}
	}

	// If page types are missing in bundle.yaml and site.yaml, use builtin defaults.
	if b.defaultPageType == nil {
		b.defaultPageType = newDefaultPageType()
	}
	if b.folderPageType == nil {
		b.folderPageType = newDefaultFolderPageType()
	}
	if b.homepagePageType == nil {
		b.homepagePageType = b.folderPageType
	}
	if b.categoryPageType == nil {
		b.categoryPageType = newDefaultCategoryPageType()
	}
	if b.categoryValuePageType == nil {
		b.categoryValuePageType = newDefaultCategoryValuePageType()
	}

	err = b.site.tags.loadPageTypes(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Parse all files and generate the output.
func (b *Builder) build() error {
	if err := b.parse(); err != nil {
		return err
	}
	if err := b.generate(); err != nil {
		return err
	}
	return nil
}

func (b *Builder) parse() error {
	ignorePath := "tags" + string(filepath.Separator)
	// Parse all files in the "content" directory (including all sub-directories)
	walk := func(path string, info os.FileInfo, err error) error {
		// Determine the folder-context
		kind := normalPageType
		var folderContext *FolderContext
		if info.IsDir() {
			// Ignore the "tags/" directory. It is handled later on.
			if path == "tags" || strings.HasPrefix(path, ignorePath) {
				return nil
			}
			// Process a directory. The content root directory is called the "homepage".
			// All other sub-directories are called "folder".
			// Homepage and folders can use different page types (as defined in bundle.yaml and site.yaml).
			println("Processing dir", path, "...")
			if path == "." {
				kind = homepagePageType
			} else {
				kind = folderPageType
			}
			folderContext, err = b.newFolderContext(path)
			if err != nil {
				return err
			}
			// A directory can have an "index.md" file.
			// If it does not exist, `parseFile` will gracefully ignore it.
			path = filepath.Join(path, "index.md")
		} else if !strings.HasSuffix(path, ".md") {
			// Ignore anything, but markdown
			return nil
		} else if filepath.Base(path) == "index.md" {
			// Ignore, has been handled by the directory already.
			return nil
		} else {
			folderContext, err = b.newFolderContext(filepath.Dir(path))
			if err != nil {
				return err
			}
		}

		// Parse the file.
		return b.parseFile(path, kind, folderContext)
	}

	err := afero.Walk(b.contentFs, ".", walk)
	if err != nil {
		return err
	}

	// Create all category pages and their children category-value pages
	for _, tagType := range b.site.tags.Types {
		var tagValuePages []interface{}
		// Destination path of the tag folder
		tagFolderPath := filepath.Join("tags", tagType.Name)
		tagType.Folder = &FolderContext{categoryPageType: tagType.pageType, categoryValuePageType: tagType.valuePageType, Name: tagType.Name, RelURL: "/" + filepath.ToSlash(tagFolderPath)}

		// Parse the pages for all tag values
		for _, tagValue := range tagType.Values {
			// Source path of the tag value markdown file (if it exists)
			tagValPath := filepath.Join(filepath.Join("tags", tagType.Name), tagValue.Name+".md")
			// XX tagValue.Folder = &FolderContext{categoryValuePageType: tagType.valuePageType, Pages: tagValue.Pages, RelURL: tagFolderPath}
			// XX err = b.parseFile(tagValPath, categoryValuePageType, tagValue.Folder)
			err = b.parseFile(tagValPath, categoryValuePageType, tagType.Folder)
			if err != nil {
				return err
			}
			tagValuePage := b.generators[tagValPath]
			tagValue.Page = tagValuePage.PageContext()
			tagValue.Page.TagType = tagType
			tagValue.Page.TagValue = tagValue
			tagValue.Name, _ = tagValue.Page.Title()
			tagValuePages = append(tagValuePages, tagValue.Page)
		}

		// Source path of the tag markdown file (if it exists)
		tagPath := filepath.Join(filepath.Join("tags", tagType.Name), "index.md")
		// Add all tag value pages to the pages of this folder
		tagType.Folder.Pages = tagValuePages
		// Parse the page for the tag type itself
		err = b.parseFile(tagPath, categoryPageType, tagType.Folder)
		if err != nil {
			return err
		}
		tagPage := b.generators[tagPath]
		tagType.Title, _ = tagPage.PageContext().Title()
		tagType.Folder.Page.TagType = tagType
		tagType.Folder.Title = tagType.Title
	}
	return nil
}

// Parses the markdown file located at `path` and creates a generator that can produce output for the file.
func (b *Builder) parseFile(path string, kind pageTypeKind, folderContext *FolderContext) error {
	g := mates.NewGrammar()
	var markdown []byte
	var pt *pageType
	var res []*mates.Resource
	var parser *mates.Parser
	var doc *mates.DocumentNode
	var err error
	var frontmatter map[string]interface{}
	tags := make(map[string][]string)

	contentResolver := func(res *mates.Resource) error {
		return resolveContentResource(b, res)
	}

	println("Processing file", path, "...")
	// Read and process the markdown file (if one exists)
	markdown, err = afero.ReadFile(b.contentFs, path)
	if err != nil {
		// The "index.md" does not exist? This is ok for the homepage and folders.
		if (kind == homepagePageType || kind == folderPageType || kind == categoryPageType || kind == categoryValuePageType) && os.IsNotExist(err) {
			// OK
			println("    markdown file is missing")
			markdown = []byte{}
		} else {
			return err
		}
	}
	// Create a parser for the page
	parser = mates.NewParser(g, markdown, contentResolver)

	// Parse and process frontmatter
	frontmatter, err = parser.ParseFrontmatter()
	if err != nil {
		return fmt.Errorf("In %v: YAML parsing error: %v", path, err)
	}
	if frontmatter == nil {
		frontmatter = make(map[string]interface{})
	}

	for k, v := range frontmatter {
		switch k {
		case "Scripts":
		case "Styles":
			if list, ok := v.([]interface{}); ok {
				for _, resurl := range list {
					if strurl, ok := resurl.(string); ok {
						u, err := url.Parse(strurl)
						if err != nil {
							return fmt.Errorf("In %v %v: URL is malformed: %v", path, k, err)
						}
						var r *mates.Resource
						if k == "Scripts" {
							r = &mates.Resource{Type: mates.ResourceTypeScript, URL: u}
						} else {
							r = &mates.Resource{Type: mates.ResourceTypeStyle, URL: u}
						}
						err = contentResolver(r)
						if err != nil {
							return fmt.Errorf("In %v %v: resource resolution error: %v", path, k, err)
						}
						res = append(res, r)
					} else {
						return fmt.Errorf("In %v %v: YAML field 'Scripts' must be a string or list of string", path, k)
					}
				}
			} else if strurl, ok := v.(string); ok {
				u, err := url.Parse(strurl)
				if err != nil {
					return fmt.Errorf("In %v %v: URL is malformed: %v", path, k, err)
				}
				var r *mates.Resource
				if k == "Scripts" {
					r = &mates.Resource{Type: mates.ResourceTypeScript, URL: u}
				} else {
					r = &mates.Resource{Type: mates.ResourceTypeStyle, URL: u}
				}
				err = contentResolver(r)
				if err != nil {
					return fmt.Errorf("In %v %v: resource resolution error: %v", path, k, err)
				}
				res = append(res, r)
			} else {
				return fmt.Errorf("In %v %v: YAML field 'Scripts' must be a string or list of string", path, k)
			}
		case "Title":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("In %v %v: YAML field 'Title' must be a string", path, k)
			}
		case "Type":
			pageTypeName, err := yamlString("Type", v, path)
			if err != nil {
				return err
			}
			pt, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return fmt.Errorf("In %v Type: %v", path, err.Error())
			}
		default:
			if list, ok := v.([]interface{}); ok {
				for _, tagValue := range list {
					if tagValueName, ok := tagValue.(string); ok {
						tags[k] = append(tags[k], tagValueName)
					} else {
						return fmt.Errorf("Value of category must be a string or a list of strings")
					}
				}
			} else {
				vstr, err := yamlString(k, v, path)
				if err != nil {
					return fmt.Errorf("Value of category must be a string or a list of strings")
				}
				tags[k] = []string{vstr}
			}
		}
	}

	// Select a page type if none has been specified
	if pt == nil {
		pt = b.selectPageType(kind, folderContext)
	}

	println("    page Type: ", pt.name)
	// Lookup syntax extension (if available)
	markdownSyntaxData, _, err := b.lookupMarkdownSyntax(pt)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Process syntax extension
	for i := 0; i < len(markdownSyntaxData); i++ {
		markdownSyntax := markdownSyntaxData[i]
		// markdownSyntaxFile := markdownSyntaxFiles[i]
		syntaxResolver := func(res *mates.Resource) error {
			return pt.resolveStaticResource(res, pt)
		}
		// println("SYNTAX: ", markdownSyntaxFile)
		syntaxParser := mates.NewParser(g, markdownSyntax, syntaxResolver)
		// Parse the frontmatter of the syntax extension. But currently it is ignored
		_, err = syntaxParser.ParseFrontmatter()
		if err != nil {
			return err
		}
		// Parse markdown of the syntax extension
		_, err = syntaxParser.ParseMarkdown()
		if err != nil {
			return err
		}
	}

	// Parse markdown
	if parser != nil {
		doc, err = parser.ParseMarkdown()
		if err != nil {
			return err
		}
	} else {
		doc = mates.NewDocument(g)
	}

	var base string
	var baseFile string
	var layouts []string
	if !pt.isNone() {
		// Lookup base and layout(s) HTML files.
		// Find all resources referenced in these HTML files.
		base, baseFile, err = b.lookupBaseHTML(pt, kind, folderContext)
		if err != nil {
			return err
		}
		if baseFile != "" {
			baseResolver := func(res *mates.Resource) error {
				return pt.resolveStaticResource(res, pt)
			}
			var baseRes []*mates.Resource
			base, baseRes, err = mates.ParseHTMLResources(base, baseResolver)
			if err != nil {
				return err
			}
			res = append(res, baseRes...)
		}
		layoutData, _, err := b.lookupLayoutHTML(pt, kind, folderContext)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for i := 0; i < len(layoutData); i++ {
			layoutStr := layoutData[i]
			layoutResolver := func(res *mates.Resource) error {
				return pt.resolveStaticResource(res, pt)
			}
			var layoutRes []*mates.Resource
			layoutStr, layoutRes, err = mates.ParseHTMLResources(layoutStr, layoutResolver)
			if err != nil {
				return err
			}
			layouts = append(layouts, layoutStr)
			res = append(res, layoutRes...)
		}
	}

	// Compute a sensible title
	if _, ok := frontmatter["Title"]; !ok {
		title := stripSuffix(filepath.Base(path))
		if title == "index" {
			if path == "." {
				title = b.site.ctx.Title
			} else {
				title = filepath.Base(filepath.Dir(path))
			}
		}
		frontmatter["Title"] = title
	}

	// Create the page
	page := &mates.Page{Grammar: g, Document: doc, Fname: path, Resources: res, Params: frontmatter, PageTypeName: pt.name}
	if pt.isNone() {
		// Do not generate a file for this page
		page.Fname = ""
	} else {
		// Set the RelURL property
		outpath := strings.TrimSuffix(path, ".md") + ".html"
		page.RelURL = "/" + filepath.ToSlash(outpath)
	}

	// Create the generator for the page
	gen := mates.NewHTMLGenerator(page, base, layouts, contentResolver, folderContext, b.site.ctx, &b.options.Options)
	b.generators[path] = gen

	// Parse all templates and determine resources
	err = gen.Prepare()
	if err != nil {
		return err
	}

	// Add page specific resources
	ptTemp := pt
	for ptTemp != nil {
		page.Resources = append(page.Resources, ptTemp.resources...)
		ptTemp = ptTemp.inheritPageType
	}

	// Determine all tags to which the generated page belongs
	for tagType, values := range tags {
		t, ok := b.site.tags.Types[tagType]
		if ok {
			for _, value := range values {
				t.addPage(value, gen.PageContext())
			}
		}
	}

	if kind == homepagePageType {
		b.site.ctx.Folder = folderContext
		folderContext.Page = gen.PageContext()
	} else if kind == folderPageType || kind == categoryPageType {
		folderContext.Page = gen.PageContext()
	}

	return nil
}

func (b *Builder) generate() error {
	// Collect a list of all pages of the site for the .Site.Pages property.
	// b.ctx.Pages = make([]*mates.PageContext, 0, len(b.generators))
	b.site.ctx.Pages = make([]interface{}, 0, len(b.generators))
	for _, gen := range b.generators {
		if gen.Page().Fname != "" {
			b.site.ctx.Pages = append(b.site.ctx.Pages, gen.PageContext())
		}
	}

	/*
		// Setup all generators
		for _, gen := range b.generators {
			err := gen.Prepare()
			if err != nil {
				return err
			}
		}
	*/

	// Generate HTML files
	for path, gen := range b.generators {
		if gen.Page().Fname == "" {
			continue
		}
		println("Generating", gen.Page().Fname, "...")
		html, err := gen.Generate()
		if err != nil {
			return err
		}
		outpath := strings.TrimSuffix(path, ".md") + ".html"
		dir := filepath.Dir(outpath)
		err = b.outputFs.MkdirAll(dir, 0775)
		if err != nil {
			return err
		}
		// println("Writing", outpath, "...")
		err = afero.WriteFile(b.outputFs, outpath, []byte(html), 0660)
		if err != nil {
			return err
		}
	}

	// Copy all resource to the output file system
	for _, gen := range b.generators {
		page := gen.Page()
		if page.Fname == "" {
			continue
		}
		for _, r := range page.Resources {
			if r.URL.IsAbs() {
				continue
			}
			id := r.UniqueID()
			if _, ok := b.copied[id]; ok {
				continue
			}
			b.copied[id] = true
			println("Copy", r.SourcePath, "to", r.DestPath)
			err := copyFile(r.SourceFs, r.SourcePath, b.outputFs, r.DestPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// newFolderContext returns a FolderContext object for the specified path.
// It searches for "folder.yaml" in this path and processes it, if it exists.
// Otherwise, default values are used.
func (b *Builder) newFolderContext(path string) (*FolderContext, error) {
	// Reuse an existing context
	if ctx, ok := b.folderContext[path]; ok {
		return ctx, nil
	}

	// Parse "folder.yaml" if it exists. If not, fill in some default values.
	yamlpath := filepath.Join(path, "folder.yaml")
	folder, err := loadYamlFile(b.contentFs, yamlpath)
	if os.IsNotExist(err) {
		folder = make(map[string]interface{})
	} else if err != nil {
		return nil, err
	}
	ctx := &FolderContext{}

	// Process "folder.yaml" and lookup all page types mentioned there.
	for k, v := range folder {
		switch k {
		case "Title":
			ctx.Title, err = yamlString(k, v, yamlpath)
		case "Page":
			pageTypeName, err := yamlString(k, v, yamlpath)
			if err != nil {
				break
			}
			ctx.defaultPageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, err
			}
		case "Homepage":
			pageTypeName, err := yamlString(k, v, yamlpath)
			if err != nil {
				break
			}
			ctx.homepagePageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, err
			}
		case "Folder":
			pageTypeName, err := yamlString(k, v, yamlpath)
			if err != nil {
				break
			}
			ctx.folderPageType, err = b.lookupPageType(pageTypeName)
			if err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, err
		}
	}

	// Fill in defaults from "bundle.yaml" or "site.yaml" where nothing has been specified in "folder.yaml".
	if ctx.defaultPageType == nil {
		ctx.defaultPageType = b.defaultPageType
	}
	if ctx.homepagePageType == nil {
		ctx.homepagePageType = b.homepagePageType
	}
	if ctx.folderPageType == nil {
		ctx.folderPageType = b.folderPageType
	}
	ctx.categoryPageType = b.categoryPageType
	ctx.categoryValuePageType = b.categoryValuePageType

	// Reuse the context for other files in the same path.
	b.folderContext[path] = ctx
	return ctx, nil
}

func (b *Builder) selectPageType(kind pageTypeKind, ctx *FolderContext) *pageType {
	if kind == homepagePageType && ctx.homepagePageType != nil {
		return ctx.homepagePageType
	}
	if kind == categoryPageType && ctx.categoryPageType != nil {
		return ctx.categoryPageType
	}
	if kind == categoryValuePageType && ctx.categoryValuePageType != nil {
		return ctx.categoryValuePageType
	}
	if (kind == folderPageType || kind == categoryPageType || kind == categoryValuePageType || kind == homepagePageType) && ctx.folderPageType != nil {
		return ctx.folderPageType
	}
	return ctx.defaultPageType
}

// Loads all syntax.md files relevant for the page type.
func (b *Builder) lookupMarkdownSyntax(pt *pageType) (data [][]byte, files []string, err error) {
	// Search in the page type and all of its parent page types
	for pt != nil {
		// Look in the page type
		p, path, err := pt.getMarkdownSyntax()
		if err == nil {
			data = append(data, p)
			files = append(files, path)
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
		pt = pt.inheritPageType
	}
	return
}

func (b *Builder) lookupBaseHTML(pt *pageType, kind pageTypeKind, ctx *FolderContext) (string, string, error) {
	for pt != nil {
		// Look in the page type
		p, path, err := pt.getBaseHTML()
		if err == nil {
			return p, path, nil
		} else if !os.IsNotExist(err) {
			return "", "", err
		}
		pt = pt.inheritPageType
	}
	return "", "", os.ErrNotExist
}

func (b *Builder) lookupLayoutHTML(pt *pageType, kind pageTypeKind, ctx *FolderContext) (data []string, files []string, err error) {
	for pt != nil {
		// Look in the page type
		p, path, err := pt.getLayoutHTML()
		if err == nil {
			data = append(data, p)
			files = append(files, path)
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
		pt = pt.inheritPageType
	}
	return
}

// Search the page type and load it.
func (b *Builder) lookupPageType(pageTypeName string) (*pageType, error) {
	// TODO: sanitize pageTypeName

	// Reuse a pageType that has already been found.
	if p, ok := b.pageTypes[pageTypeName]; ok {
		return p, nil
	}

	// Search for a file named "pageTypeName" in a directory named "pages".
	p := filepath.Join("pages", pageTypeName)
	var bndl *bundle
	var fs afero.Fs
	var ok bool

	// Search in the site for a dir named `p`
	s, err := b.site.siteFs.Stat(p)
	if err == nil && s.IsDir() {
		ok = true
		// The page is allowed to access all files of the site
		fs = b.site.siteFs
	}

	// Search in the bundle, if nothing has been found yet
	if !ok && b.bundle != nil {
		// println("Look in bundle for '", p, "'", b.bundle.path)
		s, err := b.bundle.bundleFs.Stat(p)
		ok = (err == nil && s.IsDir())
		if ok {
			// The page is allowed to access all files of the bundle
			bndl = b.bundle
			fs = bndl.bundleFs
		}
	}

	// Search in all search paths for a dir named `p`
	for _, searchPath := range b.searchPaths {
		s, err := b.fs.Stat(filepath.Join(searchPath, p))
		if err == nil && s.IsDir() {
			ok = true
			// The page is only allowed to access its own files and nothing else.
			fs = afero.NewBasePathFs(b.fs, filepath.Join(searchPath, p))
			p = ""
		}
	}

	// Error, if there is no specification for the page type.
	if !ok {
		//		panic("Oooops")
		return nil, fmt.Errorf("Could not find page type %v", pageTypeName)
	}
	pt, err := newPageType(fs, p, pageTypeName, bndl, b)
	b.pageTypes[pageTypeName] = pt
	return pt, err
}

// Search the bundle to use and load it.
func (b *Builder) lookupBundle(bundleName string) error {
	// TODO: sanitize bundle name

	// Reuse a bundle that has already been loaded
	if b.bundle != nil && b.bundle.name == bundleName {
		return nil
	}
	var ok bool
	var bundlePath string
	var fs afero.Fs
	p := filepath.Join("bundles", bundleName)

	// Search in the site for a dir named `p`
	bundlePath = p
	s, err := b.site.siteFs.Stat(bundlePath)
	if err == nil && s.IsDir() {
		ok = true
		fs = afero.NewBasePathFs(b.site.siteFs, bundlePath)
	}

	// Search in all search paths for a dir named `p`
	if !ok {
		for _, searchPath := range b.searchPaths {
			bundlePath = filepath.Join(searchPath, p)
			s, err := b.fs.Stat(bundlePath)
			if err == nil && s.IsDir() {
				ok = true
				fs = afero.NewBasePathFs(b.fs, bundlePath)
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("Bundle %v could not be found", bundleName)
	}
	b.bundle = newBundle(bundleName, bundlePath, fs, b)
	return b.bundle.load()
}
