package site

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func fixture(name string) string {
	return filepath.Join("testdata", "sites", name)
}

func load(t *testing.T, dir string, opts LoadOptions) *Site {
	t.Helper()
	site, err := Load(context.Background(), dir, opts, discardLogger())
	if err != nil {
		t.Fatalf("Load(%s) failed: %v", dir, err)
	}
	return site
}

// pagePaths flattens the subtree's page paths, section indexes included.
func pagePaths(sec *Section) []string {
	var out []string
	if sec.Page != nil {
		out = append(out, sec.Page.Path)
	}
	for _, page := range sec.Pages {
		out = append(out, page.Path)
	}
	for _, child := range sec.Sections {
		out = append(out, pagePaths(child)...)
	}
	return out
}

func assertPaths(t *testing.T, sec *Section, want []string) {
	t.Helper()
	if got := pagePaths(sec); !slices.Equal(got, want) {
		t.Fatalf("page paths = %v, want %v", got, want)
	}
}

func TestLoadKbFlat(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("kb-flat"), LoadOptions{})

	if site.Title != "Fixture" {
		t.Errorf("Title = %q, want Fixture", site.Title)
	}
	if site.Description != "Example knowledge base for testing" {
		t.Errorf("Description = %q", site.Description)
	}
	if site.BaseURL != "https://kb.example.com/" {
		t.Errorf("BaseURL = %q", site.BaseURL)
	}
	if site.Scope != "" {
		t.Errorf("Scope = %q, want empty", site.Scope)
	}

	// Root: promo.md page, kb section. Draft and notes.txt excluded.
	assertPaths(t, site.Root, []string{
		"promo.md",
		"kb/_index.md",
		"kb/maintenance-and-upgrade.md", // weight 1 first
		"kb/empty.md",                   // then by title
		"kb/no-front-matter.md",
		"kb/troubleshooting-volume-detached.md",
	})

	kb := findSection(site.Root, "kb")
	if kb == nil {
		t.Fatal("kb section not found")
	}
	if kb.Title() != "The Fixture Knowledge Base" {
		t.Errorf("kb title = %q", kb.Title())
	}
	if got := kb.TotalPages(); got != 5 {
		t.Errorf("kb TotalPages = %d, want 5", got)
	}

	byTitle := map[string]*Page{}
	for _, page := range kb.Pages {
		byTitle[page.Title] = page
	}
	if _, ok := byTitle["Manually titled page"]; !ok {
		t.Error("H1 title fallback failed for no-front-matter.md")
	}
	if _, ok := byTitle["Empty"]; !ok {
		t.Error("humanized filename fallback failed for empty.md")
	}
	if page, ok := byTitle["Best practices for node maintenance"]; !ok {
		t.Error("front matter title missing for maintenance-and-upgrade.md")
	} else {
		if page.Weight != 1 {
			t.Errorf("weight = %d, want 1", page.Weight)
		}
		if page.Description != "Draining and upgrading fixture nodes" {
			t.Errorf("description = %q", page.Description)
		}
	}
}

func TestLoadKbFlatIncludeDrafts(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("kb-flat"), LoadOptions{IncludeDrafts: true})
	if !slices.Contains(pagePaths(site.Root), "kb/draft-page.md") {
		t.Error("draft-page.md missing with IncludeDrafts")
	}
}

func TestLoadKbFlatScoped(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("kb-flat"), LoadOptions{ContentPath: "kb"})
	if site.Scope != "kb" {
		t.Errorf("Scope = %q, want kb", site.Scope)
	}
	if site.Root.Page != nil || len(site.Root.Pages) != 0 {
		t.Error("scoped root should have no pages of its own")
	}
	if len(site.Root.Sections) != 1 || site.Root.Sections[0].Path != "kb" {
		t.Fatalf("scoped root sections = %+v, want single kb section", site.Root.Sections)
	}
	if slices.Contains(pagePaths(site.Root), "promo.md") {
		t.Error("promo.md leaked into kb scope")
	}
}

// TestLoadDocsyLike exercises the failure modes of a real theme-module site
// (e.g. kubernetes.io on Docsy) whose build toolchain is not present: content
// uses shortcodes with no local templates (including a nested one), the
// project's render hooks reference a partial only the module provides, the
// config declares a cache key and output format our Hugo version no longer
// knows, and a page carries the removed "_build" front matter key. The loader
// must still assemble the tree.
func TestLoadDocsyLike(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("docsy-like"), LoadOptions{})

	if site.Title != "Docsy-like Fixture" {
		t.Errorf("Title = %q", site.Title)
	}
	assertPaths(t, site.Root, []string{
		"_index.md",
		"guide/_index.md",
		"guide/install.md",   // weight 1
		"guide/configure.md", // weight 2
	})

	guide := findSection(site.Root, "guide")
	if guide == nil {
		t.Fatal("guide section not found")
	}
	if guide.Pages[0].Title != "Install" {
		t.Errorf("guide first page = %q, want Install", guide.Pages[0].Title)
	}
}

func TestLoadDocsMultilang(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("docs-multilang"), LoadOptions{})

	if site.Title != "Docs Fixture" {
		t.Errorf("Title = %q, want Docs Fixture", site.Title)
	}

	// Default language en via content/en convention; drafts, future, expired,
	// and headless pages excluded by Hugo; leaf bundle resources excluded. A
	// build.render=never page (never-render.md) stays in the tree — Hugo keeps
	// it in its page collections, so we surface it too.
	assertPaths(t, site.Root, []string{
		"_index.md",
		"docs/_index.md",
		"docs/getting-started.md",    // weight 1
		"docs/install/index.md",      // weight 3 leaf bundle
		"docs/never-render.md",       // unweighted page, before sections
		"docs/concepts/_index.md",    // weight 2 section
		"docs/concepts/volumes.md",   // weight 2
		"docs/concepts/snapshots.md", // unweighted last
	})

	concepts := findSection(site.Root, "docs/concepts")
	if concepts == nil {
		t.Fatal("docs/concepts section not found")
	}
	if concepts.Pages[0].Title != "Volumes" {
		t.Errorf("linkTitle not preferred: got %q", concepts.Pages[0].Title)
	}
}

func TestLoadDocsMultilangZh(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("docs-multilang"), LoadOptions{Lang: "zh-cn"})
	if site.Title != "文档夹具" {
		t.Errorf("per-language title override failed: %q", site.Title)
	}
	assertPaths(t, site.Root, []string{
		"_index.md",
		"docs/_index.md",
		"docs/getting-started.md",
	})
}

func TestLoadTomlSite(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("toml-site"), LoadOptions{})

	if site.Title != "TOML Fixture EN" {
		t.Errorf("per-language title override failed: %q", site.Title)
	}
	if site.BaseURL != "https://toml.example.com/" {
		t.Errorf("BaseURL = %q", site.BaseURL)
	}

	// contentDir comes from languages.en.toml; guides ordered by weight
	// (advanced 5, first-steps 10) then title for the unweighted.
	assertPaths(t, site.Root, []string{
		"_index.md",
		"about.md",
		"guides/_index.md",
		"guides/advanced.md",
		"guides/first-steps.md",
		"guides/alpha.md",
		"guides/beta.md",
	})
}

func TestLoadTomlSiteFrench(t *testing.T) {
	t.Parallel()
	site := load(t, fixture("toml-site"), LoadOptions{Lang: "fr"})
	if site.Title != "Fixture TOML" {
		t.Errorf("Title = %q, want Fixture TOML", site.Title)
	}
	assertPaths(t, site.Root, []string{
		"_index.md",
		"guides/premiers-pas.md",
	})
}

func TestLoadTranslationByFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("hugo.toml", "title = \"Suffix Site\"\ndefaultContentLanguage = \"en\"\n[languages.en]\nweight = 1\n[languages.fr]\nweight = 2\n")
	write("content/_index.md", "---\ntitle: Home\n---\n")
	write("content/about.md", "---\ntitle: About\n---\n")
	write("content/about.fr.md", "---\ntitle: A propos\n---\n")
	write("content/contact.fr.md", "---\ntitle: Contact\n---\n")
	write("content/v1.2-notes.md", "---\ntitle: v1.2 notes\n---\n")

	en := load(t, dir, LoadOptions{})
	assertPaths(t, en.Root, []string{"_index.md", "about.md", "v1.2-notes.md"})

	fr := load(t, dir, LoadOptions{Lang: "fr"})
	assertPaths(t, fr.Root, []string{"about.fr.md", "contact.fr.md"})
}

func TestLoadContentDirDirectly(t *testing.T) {
	t.Parallel()
	site := load(t, filepath.Join(fixture("kb-flat"), "content", "kb"), LoadOptions{})
	if site.Title != "Kb" {
		t.Errorf("Title = %q, want humanized dir name Kb", site.Title)
	}
	if site.Root.Page == nil || site.Root.Page.Title != "The Fixture Knowledge Base" {
		t.Error("content root _index.md not loaded")
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, dir string
		opts      LoadOptions
	}{
		{"escaping content path", fixture("kb-flat"), LoadOptions{ContentPath: "../secrets"}},
		{"missing content path", fixture("kb-flat"), LoadOptions{ContentPath: "nope"}},
		{"empty directory", t.TempDir(), LoadOptions{}},
		{"unconfigured language", fixture("docs-multilang"), LoadOptions{Lang: "de"}},
	}
	for _, tt := range tests {
		if _, err := Load(context.Background(), tt.dir, tt.opts, discardLogger()); err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}
