package site

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/gohugoio/hugo/resources/kinds"
	"github.com/gohugoio/hugo/resources/page"
	"github.com/spf13/cast"
)

// projectSection turns a Hugo section (or the home page) and its descendants
// into a Section. Hugo has already resolved bundles, filtered unpublishable
// pages, and ordered .Pages(), so we only classify children and copy fields.
// Returns nil for a section that ends up carrying no content.
func projectSection(hp page.Page) *Section {
	sec := &Section{Path: contentPath(hp)}
	if hp.File() != nil {
		sec.Page = projectPage(hp) // the _index.md
	}
	for _, child := range hp.Pages() {
		switch child.Kind() {
		case kinds.KindSection:
			if c := projectSection(child); c != nil {
				sec.Sections = append(sec.Sections, c)
			}
		case kinds.KindPage:
			sec.Pages = append(sec.Pages, projectPage(child))
		}
	}
	if sec.Page == nil && len(sec.Pages) == 0 && len(sec.Sections) == 0 {
		return nil
	}
	return sec
}

// projectPage copies a Hugo page (a regular page, a leaf bundle's index, or a
// section index) into a Page. Path and SourcePath come straight from Hugo's
// file view, so the references/ mirror matches the source tree.
func projectPage(p page.Page) *Page {
	f := p.File()
	return &Page{
		Path:        filepath.ToSlash(f.Path()),
		Title:       pageTitle(p),
		Description: pageDescription(p),
		Weight:      p.Weight(),
		SourcePath:  f.Filename(),
	}
}

// contentPath is a section's content-relative path ("" for the home page).
func contentPath(hp page.Page) string {
	return strings.TrimPrefix(hp.Path(), "/")
}

// pageTitle prefers the front matter title (linkTitle over title, as Hugo's
// LinkTitle does), then the first H1 in the body, then a humanized name — Hugo
// leaves a titleless page's title empty rather than inventing one.
func pageTitle(p page.Page) string {
	if t := p.LinkTitle(); t != "" {
		return t
	}
	if h := firstH1([]byte(p.RawContent())); h != "" {
		return h
	}
	return humanize(fallbackName(p.File().Path()))
}

// pageDescription is the front matter description, falling back to an explicit
// summary, whitespace-collapsed.
func pageDescription(p page.Page) string {
	params := p.Params()
	desc := cast.ToString(params["description"])
	if desc == "" {
		desc = cast.ToString(params["summary"])
	}
	return strings.Join(strings.Fields(desc), " ")
}

// fallbackName is the name to humanize for a titleless page: the bundle
// directory for index files, else the file's base name.
func fallbackName(relPath string) string {
	relPath = filepath.ToSlash(relPath)
	base := strings.TrimSuffix(path.Base(relPath), path.Ext(relPath))
	if base == "index" || base == "_index" {
		if dir := path.Base(path.Dir(relPath)); dir != "." && dir != "/" {
			return dir
		}
	}
	return base
}
