package site

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gohugoio/hugo/config"
	"github.com/gohugoio/hugo/config/allconfig"
	"github.com/gohugoio/hugo/deps"
	"github.com/gohugoio/hugo/hugofs"
	"github.com/gohugoio/hugo/hugolib"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/spf13/afero"
)

// assembled is the result of running a site through Hugo: the built sites and
// the resolved config, plus a cleanup that removes the temporary stub theme.
type assembled struct {
	sites   *hugolib.HugoSites
	cfg     *siteConfig
	cleanup func()
}

// assemble runs the site at absDir through Hugo's assemble pipeline
// (SkipRender: no templates rendered, no layouts required) and returns the
// fully-resolved sites. It delegates content-tree walking, bundle/section
// resolution, translation handling, publishable filtering, and sorting to Hugo.
//
// Two pre-steps make real third-party sites assemble without their build
// toolchain present:
//
//   - Content-dir resolution: for the multilingual content/<lang> convention we
//     inject per-language contentDir; when pointed straight at a content
//     directory with no config we set contentDir=".".
//   - Stub layouts: we point layoutDir at a throwaway directory instead of the
//     site's own. This does two jobs at once. It bypasses the project's real
//     layouts — whose render hooks and partials routinely reference an
//     un-fetched theme module (e.g. Docsy on kubernetes.io) and would fail
//     assemble — and it is where we drop no-op stub templates for every
//     shortcode found in the content, since Hugo hard-fails on a shortcode whose
//     template is missing. We render nothing, so discarding the real layouts is
//     free.
func assemble(ctx context.Context, absDir string, opts LoadOptions, logger *slog.Logger) (*assembled, error) {
	// Pass 1: read the config as-is to learn languages, contentDir, and whether
	// a config file exists, so we can compute the overrides Hugo needs.
	flags := newFlags(absDir)
	cfg, _, found, err := loadConfig(ctx, flags, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect site config: %w", err)
	}

	overrides, roots, err := contentOverrides(absDir, cfg, found)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve content directories: %w", err)
	}

	shortcodes, err := discoverShortcodes(roots)
	if err != nil {
		return nil, err
	}
	layoutsDir, cleanup, err := writeStubLayouts(shortcodes)
	if err != nil {
		return nil, err
	}

	// Pass 2: the authoritative load, with the overrides added to the same flag
	// layer (Hugo only reads from it during load), feeds the build.
	for k, v := range overrides {
		flags.Set(k, v)
	}
	flags.Set("layoutDir", layoutsDir)
	if opts.IncludeDrafts {
		flags.Set("buildDrafts", true)
		flags.Set("buildFuture", true)
		flags.Set("buildExpired", true)
	}

	cfg, configs, _, err := loadConfig(ctx, flags, logger)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to reload site config: %w", err)
	}

	sites, err := buildSites(ctx, configs, flags, logger)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to build sites: %w", err)
	}
	logger.DebugContext(ctx, "assembled site", "languages", len(sites.Sites), "shortcodeStubs", len(shortcodes))
	return &assembled{sites: sites, cfg: cfg, cleanup: cleanup}, nil
}

// buildSites wires the fs and runs a SkipRender build. The fs constructor reads
// workingDir/publishDir from a config.Provider (not *allconfig.Configs), so we
// seed the flag layer with the resolved publishDir and use an in-memory
// destination (SkipRender writes nothing, but this keeps the run hermetic).
func buildSites(ctx context.Context, configs *allconfig.Configs, flags config.Provider, logger *slog.Logger) (*hugolib.HugoSites, error) {
	base := configs.Base
	flags.Set("publishDir", base.PublishDir)
	flags.Set("publishDirStatic", base.PublishDir)
	flags.Set("publishDirDynamic", base.PublishDir)

	fs := hugofs.NewFromSourceAndDestination(hugofs.Os, afero.NewMemMapFs(), flags)
	sites, err := hugolib.NewHugoSites(deps.DepsCfg{Configs: configs, Fs: fs})
	if err != nil {
		return nil, err
	}
	if err := sites.Build(hugolib.BuildCfg{SkipRender: true}); err != nil {
		// Hugo fails the build when any error was merely *logged*, even after
		// assembly finished — common on sites pinned to an older Hugo whose
		// now-removed config or front-matter keys (e.g. "_build") log a
		// deprecation error. The page tree is fully assembled by then, and we
		// only read its structure and copy source files verbatim, so tolerate
		// this aggregate error when the sites are actually populated. Genuine
		// assembly failures (missing shortcode, bad bundle) return a different,
		// earlier error and are not swallowed.
		if !isLoggedErrorsAggregate(err) || len(sites.Sites) == 0 {
			return nil, err
		}
		logger.WarnContext(ctx, "site assembled with logged errors (tolerated)", "error", err)
	}
	return sites, nil
}

// isLoggedErrorsAggregate reports whether err is Hugo's post-assembly
// "logged N error(s)" aggregate rather than a fatal assembly error.
func isLoggedErrorsAggregate(err error) bool {
	return strings.Contains(err.Error(), "error(s)")
}

// contentOverrides computes the config flags Hugo needs to read the site's
// content the way the generator expects, and the absolute content roots to scan
// for shortcodes. It returns an error when a config-less directory holds no
// content at all.
func contentOverrides(absDir string, cfg *siteConfig, found bool) (map[string]any, []string, error) {
	overrides := map[string]any{}
	rootSet := map[string]bool{}
	addRoot := func(dir string) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			rootSet[dir] = true
		}
	}

	if !found {
		// No config: read content directly from the directory when it holds
		// content files, else fall back to the conventional content/ dir.
		if hasContentFiles(absDir) {
			overrides["contentDir"] = "."
			addRoot(absDir)
			return overrides, slices.Collect(maps.Keys(rootSet)), nil
		}
		if info, err := os.Stat(filepath.Join(absDir, "content")); err != nil || !info.IsDir() {
			return nil, nil, fmt.Errorf("no Hugo config and no content found in %s", absDir)
		}
	}

	addRoot(filepath.Join(absDir, filepath.FromSlash(cfg.contentDir)))
	// Per-language contentDir: honor an explicit override, otherwise adopt the
	// content/<lang> convention when that directory exists. Injecting the
	// contentDir makes Hugo split languages by directory (as this tool always
	// has) rather than treating content/en as a section named "en".
	for lang, langContentDir := range cfg.languages {
		switch {
		case langContentDir != "":
			addRoot(filepath.Join(absDir, filepath.FromSlash(langContentDir)))
		default:
			conv := filepath.ToSlash(filepath.Join(cfg.contentDir, lang))
			if info, err := os.Stat(filepath.Join(absDir, filepath.FromSlash(conv))); err == nil && info.IsDir() {
				overrides["languages."+lang+".contentDir"] = conv
				addRoot(filepath.Join(absDir, filepath.FromSlash(conv)))
			}
		}
	}
	return overrides, slices.Collect(maps.Keys(rootSet)), nil
}

// discoverShortcodes returns every shortcode name used across the content
// roots, mapped to whether it is ever used with a closing tag ("inner"). Hugo
// fails assemble on a shortcode whose template is absent, so we need the full
// set up front to synthesize stubs — and it infers a shortcode's innerness from
// its template, so a stub must match how the content actually uses it (a
// self-closing {{< figure >}} needs a non-inner template; {{< note >}}…{{< /note >}}
// needs an inner one). Lexing (not building) is enough and never fails a build:
// an unparseable file is simply skipped.
func discoverShortcodes(roots []string) (map[string]bool, error) {
	inner := map[string]bool{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isContentFile(path) {
				return nil
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			items, lexErr := pageparser.ParseBytes(raw, pageparser.Config{})
			if lexErr != nil {
				return nil
			}
			// A shortcode name token is preceded by a close indicator only in a
			// closing tag ({{< /note >}}), which marks the shortcode as inner.
			for i, it := range items {
				if !it.IsShortcodeName() {
					continue
				}
				name := it.ValStr(raw)
				if _, seen := inner[name]; !seen {
					inner[name] = false
				}
				if i > 0 && items[i-1].IsShortcodeClose() {
					inner[name] = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s for shortcodes: %w", root, err)
		}
	}
	return inner, nil
}

// writeStubLayouts creates a throwaway layouts directory to use as the site's
// layoutDir: it holds a no-op template for every discovered shortcode (so
// content parsing never fails on a missing shortcode template) and nothing
// else, so pointing the site at it also discards the project's real layouts —
// render hooks and partials that may reference an un-fetched theme module.
// Inner shortcodes emit their body ({{ .Inner }}); the rest emit nothing. The
// directory is always created (even with no shortcodes) so the layout bypass
// always applies. The returned cleanup removes it.
func writeStubLayouts(shortcodes map[string]bool) (layoutsDir string, cleanup func(), err error) {
	cleanup = func() {}
	dir, err := os.MkdirTemp("", "hugo-to-skill-layouts-*")
	if err != nil {
		return "", cleanup, fmt.Errorf("failed to create stub layouts: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	scDir := filepath.Join(dir, "_shortcodes")
	for name, inner := range shortcodes {
		body := "{{/* hugo-to-skill stub */}}"
		if inner {
			body = "{{ .Inner }}"
		}
		// Nested shortcode names ("blocks/feature") map to nested files.
		file := filepath.Join(scDir, filepath.FromSlash(name)+".html")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("failed to create stub layouts: %w", err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("failed to create stub layouts: %w", err)
		}
	}
	return dir, cleanup, nil
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

// isContentFile reports whether name is a file Hugo treats as page content.
func isContentFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".html", ".htm":
		return true
	}
	return false
}
