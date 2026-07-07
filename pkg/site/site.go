// Package site loads a Hugo site's config and content tree into a model the
// skill generator can consume. It runs the site through Hugo's own assemble
// pipeline (hugolib with SkipRender: content is read, bundles and sections are
// resolved, unpublishable pages are filtered, and pages are ordered — but no
// templates are rendered), then projects the result onto a small tree model.
// Page source files themselves are never modified; the generator copies them
// byte-for-byte.
package site

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gohugoio/hugo/hugolib"
	"github.com/spf13/cast"
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

	built, err := assemble(ctx, absDir, opts, logger)
	if err != nil {
		return nil, err
	}
	defer built.cleanup()
	cfg := built.cfg

	selectedLang := strings.ToLower(cmp.Or(opts.Lang, cfg.defaultLang))
	hugoSite, err := selectSite(built.sites.Sites, selectedLang, cfg)
	if err != nil {
		return nil, err
	}
	logger.DebugContext(ctx, "selected language", "lang", selectedLang)

	root := projectSection(hugoSite.Home())
	if root == nil {
		return nil, fmt.Errorf("no content pages found for language %s", selectedLang)
	}

	scope, err := cleanScope(opts.ContentPath)
	if err != nil {
		return nil, err
	}
	var scoped *Section
	if scope != "" {
		scoped = findSection(root, scope)
		if scoped == nil {
			return nil, fmt.Errorf("content path %s not found in the content tree", scope)
		}
		root = &Section{Sections: []*Section{scoped}}
	}

	// Hugo merges the base title into each language's config and applies any
	// per-language override, so the selected site's Title() is already the
	// effective one; fall back to the directory name only when it is empty.
	title := hugoSite.Title()
	if title == "" {
		title = humanize(filepath.Base(absDir))
	}

	site := &Site{
		Title:       title,
		Description: cast.ToString(hugoSite.Params()["description"]),
		BaseURL:     hugoSite.BaseURL(),
		Scope:       scope,
		Root:        root,
		Scoped:      scoped,
	}
	logger.InfoContext(ctx, "site loaded", "title", site.Title, "pages", root.TotalPages())
	return site, nil
}

// selectSite picks the built site for the selected language. A site with
// languages configured must have the selected one; an unconfigured single
// language falls through to Hugo's sole site.
func selectSite(sites []*hugolib.Site, selectedLang string, cfg *siteConfig) (*hugolib.Site, error) {
	if len(cfg.languages) > 0 {
		if _, ok := cfg.languages[selectedLang]; !ok {
			return nil, fmt.Errorf("language %s is not configured for this site", selectedLang)
		}
	}
	for _, s := range sites {
		if strings.EqualFold(s.Language().Lang, selectedLang) {
			return s, nil
		}
	}
	if len(sites) > 0 && len(cfg.languages) == 0 {
		return sites[0], nil
	}
	return nil, fmt.Errorf("language %s produced no site", selectedLang)
}

// findSection returns the section at the given content-relative path.
func findSection(root *Section, target string) *Section {
	if root.Path == target {
		return root
	}
	for _, child := range root.Sections {
		if found := findSection(child, target); found != nil {
			return found
		}
	}
	return nil
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

// firstH1 returns the text of the first ATX H1 in body, if any.
func firstH1(body []byte) string {
	for line := range strings.SplitSeq(string(body), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if rest, ok := strings.CutPrefix(trimmed, "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
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
