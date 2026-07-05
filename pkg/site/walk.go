package site

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/spf13/cast"
)

// walker collects the content tree for one language.
type walker struct {
	selectedLang  string
	defaultLang   string
	langs         map[string]bool // configured language codes, lowercase
	includeDrafts bool
	now           time.Time
	logger        *slog.Logger
	ctx           context.Context
}

// walkSection reads the directory at absDir into a Section. relPath is the
// section's content-relative path ("" for the content root). Returns nil for
// directories that end up with no content.
func (w *walker) walkSection(absDir, relPath string) (*Section, error) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("reading content directory %s: %w", absDir, err)
	}
	return w.walkEntries(entries, absDir, relPath)
}

// walkEntries builds the Section for a directory whose listing was already
// read, so each directory is listed exactly once during the walk.
func (w *walker) walkEntries(entries []os.DirEntry, absDir, relPath string) (*Section, error) {
	sec := &Section{Path: relPath}
	var subdirs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			subdirs = append(subdirs, name)
			continue
		}
		if !isContentFile(name) {
			continue
		}
		base, lang := w.splitLang(stem(name))
		if !w.langMatches(lang) {
			w.logger.DebugContext(w.ctx, "skipping other-language file", "file", path.Join(relPath, name), "lang", lang)
			continue
		}
		page, err := w.buildPage(filepath.Join(absDir, name), path.Join(relPath, name))
		if err != nil {
			return nil, err
		}
		if page == nil {
			continue
		}
		if base == "_index" {
			sec.Page = page
		} else {
			sec.Pages = append(sec.Pages, page)
		}
	}

	for _, name := range subdirs {
		subAbs := filepath.Join(absDir, name)
		subRel := path.Join(relPath, name)
		subEntries, err := os.ReadDir(subAbs)
		if err != nil {
			return nil, fmt.Errorf("reading content directory %s: %w", subAbs, err)
		}
		if isBundle, bundleFile := w.leafBundleFile(subEntries); isBundle {
			if bundleFile == "" {
				w.logger.DebugContext(w.ctx, "skipping leaf bundle without selected language", "dir", subRel)
				continue
			}
			// The bundle directory is the page; other files in it are resources.
			page, err := w.buildPage(filepath.Join(subAbs, bundleFile), path.Join(subRel, bundleFile))
			if err != nil {
				return nil, err
			}
			if page != nil {
				sec.Pages = append(sec.Pages, page)
			}
			continue
		}
		child, err := w.walkEntries(subEntries, subAbs, subRel)
		if err != nil {
			return nil, err
		}
		if child != nil {
			sec.Sections = append(sec.Sections, child)
		}
	}

	if sec.Page == nil && len(sec.Pages) == 0 && len(sec.Sections) == 0 {
		return nil, nil
	}
	sortPages(sec.Pages)
	sortSections(sec.Sections)
	return sec, nil
}

// leafBundleFile reports whether a directory listing is a leaf bundle
// (contains an index.* content file) and returns the index filename for the
// selected language, or "" when the bundle has no version in that language.
func (w *walker) leafBundleFile(entries []os.DirEntry) (isBundle bool, filename string) {
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isContentFile(name) {
			continue
		}
		base, lang := w.splitLang(stem(name))
		if base != "index" {
			continue
		}
		isBundle = true
		if w.langMatches(lang) {
			filename = name
		}
	}
	return isBundle, filename
}

// buildPage parses a content file's front matter into a Page. Returns nil
// (no error) for pages Hugo would not publish, unless drafts are included.
func (w *walker) buildPage(absPath, relPath string) (*Page, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading page %s: %w", relPath, err)
	}
	cfm, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(raw))
	if err != nil {
		w.logger.WarnContext(w.ctx, "failed to parse front matter, copying page with fallback metadata", "page", relPath, "error", err)
		cfm = pageparser.ContentFrontMatter{}
	}

	if !w.includeDrafts && !publishable(cfm.FrontMatter, w.now) {
		w.logger.DebugContext(w.ctx, "skipping unpublishable page", "page", relPath)
		return nil, nil
	}

	// pageparser returns an empty Content when a file has no front matter,
	// so fall back to the raw bytes for the body scan.
	body := cfm.Content
	if len(body) == 0 {
		body = raw
	}

	title := cast.ToString(fmLookup(cfm.FrontMatter, "linkTitle"))
	if title == "" {
		title = cast.ToString(fmLookup(cfm.FrontMatter, "title"))
	}
	if title == "" {
		title = firstH1(body)
	}
	if title == "" {
		title = humanize(w.pageName(relPath))
	}

	description := cast.ToString(fmLookup(cfm.FrontMatter, "description"))
	if description == "" {
		description = cast.ToString(fmLookup(cfm.FrontMatter, "summary"))
	}

	return &Page{
		Path:        relPath,
		Title:       title,
		Description: strings.Join(strings.Fields(description), " "),
		Weight:      cast.ToInt(fmLookup(cfm.FrontMatter, "weight")),
		SourcePath:  absPath,
	}, nil
}

// publishable reports whether Hugo would publish a page with this front
// matter: not a draft, not future-dated, not expired, not headless, and not
// build.render: never.
func publishable(fm map[string]any, now time.Time) bool {
	if cast.ToBool(fmLookup(fm, "draft")) {
		return false
	}
	if t, err := cast.ToTimeE(fmLookup(fm, "publishDate")); err == nil && !t.IsZero() && t.After(now) {
		return false
	}
	if t, err := cast.ToTimeE(fmLookup(fm, "expiryDate")); err == nil && !t.IsZero() && t.Before(now) {
		return false
	}
	if cast.ToBool(fmLookup(fm, "headless")) {
		return false
	}
	build := fmLookup(fm, "build")
	if build == nil {
		build = fmLookup(fm, "_build") // legacy alias
	}
	if buildMap, err := cast.ToStringMapE(build); err == nil {
		render := fmLookup(buildMap, "render")
		if b, ok := render.(bool); ok && !b {
			return false
		}
		if cast.ToString(render) == "never" {
			return false
		}
	}
	return true
}

// fmLookup finds a front matter value by key, case-insensitively — sites
// write publishDate, publishdate, or PublishDate interchangeably.
func fmLookup(fm map[string]any, key string) any {
	for k, v := range fm {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return nil
}

// firstH1 returns the text of the first ATX H1 in body, if any.
func firstH1(body []byte) string {
	for line := range bytes.Lines(body) {
		trimmed := bytes.TrimLeft(line, " ")
		if rest, ok := bytes.CutPrefix(trimmed, []byte("# ")); ok {
			return string(bytes.TrimSpace(rest))
		}
	}
	return ""
}

// langMatches reports whether a file's language suffix ("" for none) selects
// it for the walked language.
func (w *walker) langMatches(lang string) bool {
	if lang == "" {
		return w.selectedLang == w.defaultLang
	}
	return lang == w.selectedLang
}

// isContentFile reports whether name is a markdown content file.
func isContentFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// stem strips the content extension: "about.fr.md" → "about.fr".
func stem(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// splitLang splits a translation-by-filename suffix off a file stem:
// "about.fr" → ("about", "fr"). Only configured language codes count, so
// "v1.2-notes" stays whole.
func (w *walker) splitLang(stem string) (base, lang string) {
	i := strings.LastIndex(stem, ".")
	if i < 0 {
		return stem, ""
	}
	if suffix := strings.ToLower(stem[i+1:]); w.langs[suffix] {
		return stem[:i], suffix
	}
	return stem, ""
}

// pageName returns the name to humanize for a page's title fallback: the
// bundle directory name for index/_index files, else the file stem without
// any language suffix.
func (w *walker) pageName(relPath string) string {
	base, _ := w.splitLang(stem(path.Base(relPath)))
	if base == "index" || base == "_index" {
		if dir := path.Base(path.Dir(relPath)); dir != "." && dir != "/" {
			return dir
		}
	}
	return base
}
