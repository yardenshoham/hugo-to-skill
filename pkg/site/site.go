// Package site loads a Hugo site's config and content tree into a model the
// skill generator can consume. It reads config through Hugo's own config
// package and front matter through Hugo's pageparser; page files themselves
// are never modified.
package site

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Site is the loaded model of a Hugo site, scoped to one language and
// optionally to a content subpath.
type Site struct {
	Title       string // site title, with the selected language's override applied
	Description string // params.description
	BaseURL     string
	Scope       string // the --content-path value ("" for the whole site)
	Root        *Section
	Scoped      *Section // the section Scope points at, nil when unscoped
}

// Section is a directory in the content tree: a branch bundle (_index.md) or
// a plain directory.
type Section struct {
	Path     string // content-relative, "" for root
	Page     *Page  // the _index.md, may be nil
	Pages    []*Page
	Sections []*Section
}

// Page is a content page: a regular .md file or a leaf bundle's index.md.
type Page struct {
	Path        string // content-relative source path
	Title       string
	Description string
	Weight      int
	SourcePath  string // absolute path of the file to copy
}

// LoadOptions control language selection, content scoping, and draft
// handling.
type LoadOptions struct {
	ContentPath   string // subpath of the content dir to extract, e.g. "kb"
	Lang          string // language for multilingual sites
	IncludeDrafts bool   // include pages Hugo would not publish
}

// Title returns the section's display title: its index page's title, else the
// humanized directory name.
func (s *Section) Title() string {
	if s.Page != nil && s.Page.Title != "" {
		return s.Page.Title
	}
	return humanize(path.Base(s.Path))
}

// Description returns the section index page's description, if any.
func (s *Section) Description() string {
	if s.Page != nil {
		return s.Page.Description
	}
	return ""
}

// TotalPages counts the content files in the subtree, section index pages
// included.
func (s *Section) TotalPages() int {
	n := len(s.Pages)
	if s.Page != nil {
		n++
	}
	for _, child := range s.Sections {
		n += child.TotalPages()
	}
	return n
}

// Load reads the Hugo site at dir into a Site.
func Load(ctx context.Context, dir string, opts LoadOptions, logger *slog.Logger) (*Site, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving site directory: %w", err)
	}

	cfg, found, err := loadConfig(ctx, absDir, logger)
	if err != nil {
		return nil, err
	}
	if !found {
		logger.WarnContext(ctx, "no Hugo config found, auto-detecting content root", "dir", absDir)
	}

	selectedLang := strings.ToLower(cmp.Or(opts.Lang, cfg.defaultLang))
	defaultLang := cfg.defaultLang

	contentRoot, langScoped, err := resolveContentRoot(absDir, cfg, found, selectedLang)
	if err != nil {
		return nil, err
	}
	if langScoped {
		// Inside a language-specific content dir, suffix-less files belong
		// to that language rather than the site default.
		defaultLang = selectedLang
	}
	logger.DebugContext(ctx, "resolved content root", "dir", contentRoot, "lang", selectedLang)

	scope, err := cleanScope(opts.ContentPath)
	if err != nil {
		return nil, err
	}
	walkRoot := contentRoot
	if scope != "" {
		walkRoot = filepath.Join(contentRoot, filepath.FromSlash(scope))
		info, err := os.Stat(walkRoot)
		if err != nil {
			return nil, fmt.Errorf("content path %s: %w", scope, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("content path %s is not a directory", scope)
		}
	}

	langs := make(map[string]bool, len(cfg.languages)+1)
	for lang := range cfg.languages {
		langs[strings.ToLower(lang)] = true
	}
	langs[defaultLang] = true
	if !langs[selectedLang] && len(cfg.languages) > 0 {
		return nil, fmt.Errorf("language %s is not configured for this site", selectedLang)
	}

	w := &walker{
		selectedLang:  selectedLang,
		defaultLang:   defaultLang,
		langs:         langs,
		includeDrafts: opts.IncludeDrafts,
		now:           time.Now(),
		logger:        logger,
		ctx:           ctx,
	}
	section, err := w.walkSection(walkRoot, scope)
	if err != nil {
		return nil, err
	}
	if section == nil {
		return nil, fmt.Errorf("no content pages found under %s", walkRoot)
	}

	root := section
	var scoped *Section
	if scope != "" {
		scoped = section
		root = &Section{Path: "", Sections: []*Section{section}}
	}

	title := cfg.title
	if langCfg, ok := cfg.languages[selectedLang]; ok && langCfg.title != "" {
		title = langCfg.title
	}
	if title == "" {
		title = humanize(filepath.Base(absDir))
	}

	site := &Site{
		Title:       title,
		Description: cfg.description,
		BaseURL:     cfg.baseURL,
		Scope:       scope,
		Root:        root,
		Scoped:      scoped,
	}
	logger.InfoContext(ctx, "site loaded", "title", site.Title, "pages", root.TotalPages())
	return site, nil
}

// resolveContentRoot picks the directory to walk: the selected language's
// content dir when configured (or present by convention), the site's content
// dir otherwise, or the site dir itself when it directly holds content.
// langScoped reports whether the returned dir holds a single language's
// content.
func resolveContentRoot(dir string, cfg *siteConfig, configFound bool, lang string) (root string, langScoped bool, err error) {
	if !configFound {
		if hasContentFiles(dir) {
			return dir, false, nil
		}
		contentDir := filepath.Join(dir, "content")
		if info, err := os.Stat(contentDir); err == nil && info.IsDir() {
			return contentDir, false, nil
		}
		return "", false, fmt.Errorf("no Hugo config and no content found in %s", dir)
	}

	contentDir := cfg.contentDir
	if langCfg, ok := cfg.languages[lang]; ok {
		if langCfg.contentDir != "" {
			return filepath.Join(dir, filepath.FromSlash(langCfg.contentDir)), true, nil
		}
		// Convention: per-language subdirectories like content/en, content/zh-cn.
		byConvention := filepath.Join(dir, filepath.FromSlash(contentDir), lang)
		if info, err := os.Stat(byConvention); err == nil && info.IsDir() {
			return byConvention, true, nil
		}
	}
	return filepath.Join(dir, filepath.FromSlash(contentDir)), false, nil
}

// hasContentFiles reports whether dir directly contains content files.
func hasContentFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && isContentFile(entry.Name()) {
			return true
		}
	}
	return false
}

// cleanScope normalizes a --content-path value to a clean slash-separated
// relative path.
func cleanScope(contentPath string) (string, error) {
	if contentPath == "" {
		return "", nil
	}
	scope := path.Clean(strings.Trim(filepath.ToSlash(contentPath), "/"))
	if scope == "." {
		return "", nil
	}
	if !filepath.IsLocal(filepath.FromSlash(scope)) {
		return "", fmt.Errorf("content path %s escapes the content directory", contentPath)
	}
	return scope, nil
}

// hugoLess is Hugo's default list order: weight ascending with zero/missing
// weights last, then lowercased title, then path.
func hugoLess(wi, wj int, ti, tj, pi, pj string) bool {
	if wi != wj {
		if wi == 0 || wj == 0 {
			return wj == 0
		}
		return wi < wj
	}
	if ti, tj := strings.ToLower(ti), strings.ToLower(tj); ti != tj {
		return ti < tj
	}
	return pi < pj
}

// sortPages orders pages the way Hugo lists them by default.
func sortPages(pages []*Page) {
	sort.SliceStable(pages, func(i, j int) bool {
		a, b := pages[i], pages[j]
		return hugoLess(a.Weight, b.Weight, a.Title, b.Title, a.Path, b.Path)
	})
}

// sortSections orders sections by their index page's weight, then title.
func sortSections(sections []*Section) {
	weight := func(s *Section) int {
		if s.Page != nil {
			return s.Page.Weight
		}
		return 0
	}
	sort.SliceStable(sections, func(i, j int) bool {
		a, b := sections[i], sections[j]
		return hugoLess(weight(a), weight(b), a.Title(), b.Title(), a.Path, b.Path)
	})
}

var titleReplacer = strings.NewReplacer("-", " ", "_", " ")

// humanize turns a file or directory name into a readable title.
func humanize(name string) string {
	words := strings.Fields(titleReplacer.Replace(name))
	if len(words) == 0 {
		return ""
	}
	s := strings.Join(words, " ")
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
