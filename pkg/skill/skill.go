// Package skill generates an Agent Skills-compatible skill directory from a
// loaded Hugo site: a SKILL.md with hierarchical indexes plus a references/
// tree that mirrors the content tree with byte-for-byte page copies.
package skill

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gohugoio/hugo/common/paths"
	"github.com/gohugoio/hugo/parser"
	"github.com/gohugoio/hugo/parser/metadecoders"
	"github.com/yardenshoham/hugo-to-skill/pkg/site"
)

// Config holds configuration for skill generation.
type Config struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	AllowedTools  string
	Metadata      map[string]string
	Notes         []string
}

// frontMatter is the SKILL.md frontmatter, serialized in field order with map
// keys sorted by Hugo's YAML marshaler.
type frontMatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
}

// refsDirName is the skill subdirectory holding the mirrored content tree;
// writeSection writes it and every generated link points into it.
const refsDirName = "references"

// fullListingMaxPages is the two-branch index rule's threshold: sites with at
// most this many content files get every page listed in SKILL.md; larger
// sites get top-level sections only. Either way SKILL.md stays well under the
// spec's 500-line guidance.
const fullListingMaxPages = 100

// maxDescriptionLen is the Agent Skills spec's limit for the description
// frontmatter field.
const maxDescriptionLen = 1024

// maxNameLen is the Agent Skills spec's limit for the name frontmatter field.
const maxNameLen = 64

var nameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// GenerateDir generates the skill directory on the filesystem. The
// references/ subdirectory is deleted and recreated so stale pages from a
// previous run cannot linger.
func GenerateDir(ctx context.Context, s *site.Site, dir string, config Config, logger *slog.Logger) error {
	name, err := skillName(s, config)
	if err != nil {
		return fmt.Errorf("failed to determine skill name: %w", err)
	}
	if base := filepath.Base(dir); base != name {
		logger.WarnContext(ctx, "skill name does not match the output directory name; the spec expects them to match",
			"name", name, "directory", base)
	}

	refsDir := filepath.Join(dir, refsDirName)
	if err := os.RemoveAll(refsDir); err != nil {
		return fmt.Errorf("failed to clear references directory: %w", err)
	}
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create references directory: %w", err)
	}

	skillPath := filepath.Join(dir, "SKILL.md")
	logger.InfoContext(ctx, "writing SKILL.md", "path", skillPath)
	f, err := os.Create(skillPath)
	if err != nil {
		return fmt.Errorf("failed to create SKILL.md: %w", err)
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	if err := generateSkill(s, buf, config, name); err != nil {
		return fmt.Errorf("failed to generate SKILL.md: %w", err)
	}
	if err := buf.Flush(); err != nil {
		return fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	return writeSection(ctx, s.Root, refsDir, logger)
}

// writeSection writes a section's _index.md and page copies, then recurses.
func writeSection(ctx context.Context, sec *site.Section, refsDir string, logger *slog.Logger) error {
	secDir := filepath.Join(refsDir, filepath.FromSlash(sec.Path))
	if err := os.MkdirAll(secDir, 0o755); err != nil {
		return fmt.Errorf("failed to create section directory %s: %w", sec.Path, err)
	}

	if hasIndex(sec) {
		indexPath := filepath.Join(secDir, "_index.md")
		logger.DebugContext(ctx, "writing section index", "path", indexPath)
		var source []byte
		if sec.Page != nil {
			var err error
			if source, err = os.ReadFile(sec.Page.SourcePath); err != nil {
				return fmt.Errorf("failed to read section index %s: %w", sec.Page.Path, err)
			}
		}
		f, err := os.Create(indexPath)
		if err != nil {
			return fmt.Errorf("failed to create section index: %w", err)
		}
		buf := bufio.NewWriter(f)
		err = GenerateSectionIndex(sec, source, buf)
		if err == nil {
			err = buf.Flush()
		}
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to generate section index for %s: %w", sec.Path, err)
		}
	}

	for _, page := range sec.Pages {
		dst := filepath.Join(refsDir, filepath.FromSlash(page.Path))
		logger.DebugContext(ctx, "copying page", "page", page.Path)
		if err := copyFile(page.SourcePath, dst); err != nil {
			return fmt.Errorf("failed to copy page %s: %w", page.Path, err)
		}
	}

	for _, child := range sec.Sections {
		if err := writeSection(ctx, child, refsDir, logger); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies src to dst byte-for-byte, creating parent directories.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// generateSkill renders SKILL.md for an already-validated skill name.
func generateSkill(s *site.Site, w io.Writer, config Config, name string) error {
	title := effectiveTitle(s)
	description := cmp.Or(config.Description, autoDescription(s, title))
	if len(description) > maxDescriptionLen {
		description = description[:maxDescriptionLen-1] + "…"
	}
	baseURL := displayBaseURL(s.BaseURL)

	if err := parser.InterfaceToFrontMatter(frontMatter{
		Name:          name,
		Description:   description,
		License:       config.License,
		Compatibility: config.Compatibility,
		AllowedTools:  config.AllowedTools,
		Metadata:      config.Metadata,
	}, metadecoders.YAML, w); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// Body
	fmt.Fprintf(w, "\n# %s\n", title)
	if s.Description != "" {
		fmt.Fprintf(w, "\n%s\n", ensureSentence(s.Description))
	}

	fmt.Fprintf(w, "\n## How to use this skill\n\n")
	fmt.Fprintf(w, "- Find the most relevant page in the contents below and read it before answering.\n")
	fmt.Fprintf(w, "- For keyword lookups across all pages, grep the `references/` directory.\n")
	fmt.Fprintf(w, "- Pages are verbatim copies of the site's Hugo source: they start with front matter\n"+
		"  metadata and may contain shortcodes like `{{< note >}}`; read through them.\n")
	fmt.Fprintf(w, "- Links inside pages: relative links point at sibling files here; `{{< relref \"x\" >}}` /\n"+
		"  `{{< ref \"x\" >}}` and site-absolute links name a content page — find the matching file\n"+
		"  under `references/`%s.\n", browseHint(baseURL))
	if baseURL != "" {
		fmt.Fprintf(w, "- The live URL of a page ≈ %s + its `references/`-relative path without `.md`\n"+
			"  (`_index.md` → the directory URL); use that when citing sources.\n", baseURL)
	}
	fmt.Fprintf(w, "- Content was extracted from the site at generation time and may have drifted since.\n")

	if len(config.Notes) > 0 {
		fmt.Fprintf(w, "\n## Notes\n\n")
		for _, note := range config.Notes {
			fmt.Fprintf(w, "- %s\n", note)
		}
	}

	fmt.Fprintf(w, "\n## Contents\n")
	if s.Root.TotalPages() <= fullListingMaxPages {
		writeFullListing(w, s.Root, title)
	} else {
		writeSectionListing(w, s.Root, title)
	}

	if baseURL != "" {
		fmt.Fprintf(w, "\nAnything not covered here: %s\n", baseURL)
	}
	return nil
}

// writeFullListing lists every page, grouped by section, depth-first.
func writeFullListing(w io.Writer, root *site.Section, rootTitle string) {
	var walk func(sec *site.Section)
	walk = func(sec *site.Section) {
		if hasIndex(sec) {
			title := sec.Title()
			if sec.Path == "" {
				title = rootTitle
			}
			fmt.Fprintf(w, "\n### [%s](%s) — %s\n", title, indexLink(sec.Path), pageCount(sec.TotalPages()))
			if len(sec.Pages) > 0 {
				fmt.Fprintf(w, "\n")
				for _, page := range sec.Pages {
					fmt.Fprintf(w, "- [%s](%s)%s\n", page.Title, pageLink(page.Path), blurb(page.Description))
				}
			}
		}
		for _, child := range sec.Sections {
			walk(child)
		}
	}
	walk(root)
}

// writeSectionListing lists top-level sections only, one line each. When the
// tree funnels through a single childless section (e.g. --content-path deep
// in the tree), it descends to the first level with real branching so the
// listing has more than one line.
func writeSectionListing(w io.Writer, root *site.Section, rootTitle string) {
	for len(root.Sections) == 1 && len(root.Pages) == 0 && !hasIndex(root) &&
		len(root.Sections[0].Sections) > 0 {
		root = root.Sections[0]
	}
	fmt.Fprintf(w, "\n")
	if hasIndex(root) {
		title := rootTitle
		if root.Path != "" {
			title = root.Title()
		}
		fmt.Fprintf(w, "- [%s](%s) — %s\n", title, indexLink(root.Path), pageCount(len(root.Pages)))
	}
	for _, sec := range root.Sections {
		fmt.Fprintf(w, "- [%s](%s) — %s%s\n", sec.Title(), indexLink(sec.Path), pageCount(sec.TotalPages()), sectionBlurb(sec.Description()))
	}
}

// GenerateSectionIndex writes a section's _index.md to w: the source index
// file verbatim (may be nil) followed by generated Pages/Subsections
// listings with links relative to the section directory.
func GenerateSectionIndex(sec *site.Section, source []byte, w io.Writer) error {
	if len(source) > 0 {
		if _, err := w.Write(source); err != nil {
			return err
		}
		if !bytes.HasSuffix(source, []byte("\n")) {
			fmt.Fprintf(w, "\n")
		}
	} else {
		fmt.Fprintf(w, "# %s\n", sec.Title())
	}

	// rel makes a subtree path relative to the section directory.
	rel := func(p string) string {
		if sec.Path == "" {
			return p
		}
		return strings.TrimPrefix(p, sec.Path+"/")
	}

	if len(sec.Pages) > 0 {
		fmt.Fprintf(w, "\n## Pages\n\n")
		for _, page := range sec.Pages {
			fmt.Fprintf(w, "- [%s](%s)%s\n", page.Title, rel(page.Path), blurb(page.Description))
		}
	}

	if len(sec.Sections) > 0 {
		fmt.Fprintf(w, "\n## Subsections\n\n")
		for _, child := range sec.Sections {
			fmt.Fprintf(w, "- [%s](%s/_index.md) — %s%s\n", child.Title(), rel(child.Path), pageCount(child.TotalPages()), sectionBlurb(child.Description()))
		}
	}
	return nil
}

// hasIndex reports whether a section gets its own _index.md in references/:
// the root only when it has content of its own, every other section because
// parent listings link to it. The generated listings use the same predicate
// so every emitted link has a matching file.
func hasIndex(sec *site.Section) bool {
	return sec.Path != "" || sec.Page != nil || len(sec.Pages) > 0
}

// skillName returns the validated skill name: the explicit override, else a
// slug derived from the site title and content-path scope.
func skillName(s *site.Site, config Config) (string, error) {
	if config.Name != "" {
		if len(config.Name) > maxNameLen || !nameRegexp.MatchString(config.Name) {
			return "", fmt.Errorf("invalid skill name %q: must be 1-%d lowercase letters, digits, and single hyphens", config.Name, maxNameLen)
		}
		return config.Name, nil
	}
	parts := []string{slugify(s.Title)}
	for component := range strings.SplitSeq(s.Scope, "/") {
		if component != "" {
			parts = append(parts, slugify(component))
		}
	}
	name := collapseHyphens(strings.Join(parts, "-"))
	if len(name) > maxNameLen {
		name = strings.Trim(name[:maxNameLen], "-")
	}
	if !nameRegexp.MatchString(name) {
		return "", fmt.Errorf("could not derive a valid skill name from site title %q; pass --name", s.Title)
	}
	return name, nil
}

// slugify derives a name fragment from a title using Hugo's own
// slugification, then narrows it to the spec's [a-z0-9-] alphabet — Sanitize
// keeps case and dots, so lower it and map leftover runes to hyphens.
func slugify(title string) string {
	s := strings.ToLower(paths.Sanitize(title))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return collapseHyphens(b.String())
}

// collapseHyphens squeezes hyphen runs and trims leading/trailing hyphens.
func collapseHyphens(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// effectiveTitle is what the skill is about: the scoped section's title when
// --content-path narrows the site, else the site title.
func effectiveTitle(s *site.Site) string {
	if s.Scoped != nil {
		return s.Scoped.Title()
	}
	return s.Title
}

// autoDescription builds the default frontmatter description from the
// effective title and the best available description text.
func autoDescription(s *site.Site, title string) string {
	desc := s.Description
	if s.Scoped != nil && s.Scoped.Description() != "" {
		desc = s.Scoped.Description()
	}
	if desc == "" {
		return fmt.Sprintf("%s. Use when answering questions about %s.", title, title)
	}
	return fmt.Sprintf("%s — %s Use when answering questions about %s.", title, ensureSentence(desc), title)
}

// ensureSentence makes sure s ends with sentence punctuation.
func ensureSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsRune(".!?", rune(s[len(s)-1])) {
		return s
	}
	return s + "."
}

// displayBaseURL normalizes the configured baseURL for display, with a
// trailing slash so path concatenation reads correctly.
func displayBaseURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	return strings.TrimSuffix(baseURL, "/") + "/"
}

// browseHint phrases the fallback for links that have no matching file.
func browseHint(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf(", or browse %s + the path", baseURL)
}

// pageLink is the SKILL.md-relative link to a page copy.
func pageLink(pagePath string) string {
	return refsDirName + "/" + pagePath
}

// indexLink is the SKILL.md-relative link to a section's index.
func indexLink(sectionPath string) string {
	return pageLink(path.Join(sectionPath, "_index.md"))
}

// pageCount formats a page tally.
func pageCount(n int) string {
	if n == 1 {
		return "1 page"
	}
	return fmt.Sprintf("%d pages", n)
}

// blurb formats an optional one-line description for a page listing entry.
func blurb(desc string) string {
	if desc == "" {
		return ""
	}
	return " — " + strings.TrimSuffix(desc, ".")
}

// sectionBlurb formats an optional description after a section's page count.
func sectionBlurb(desc string) string {
	if desc == "" {
		return ""
	}
	return ", " + strings.TrimSuffix(desc, ".")
}
