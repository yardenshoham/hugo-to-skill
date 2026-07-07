package site

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/config"
	"github.com/gohugoio/hugo/config/allconfig"
	"github.com/spf13/afero"
)

// siteConfig holds the config values the generator needs before the site is
// assembled — content-dir resolution and language selection. Everything with a
// post-assembly equivalent (title, baseURL, description) is read off the built
// hugolib.Site instead. contentDir and defaultLang always carry Hugo's defaults
// ("content", "en") so consumers never re-apply them.
type siteConfig struct {
	contentDir  string
	defaultLang string // defaultContentLanguage, lowercase
	// languages maps each configured language (lowercased) to its per-language
	// contentDir override, "" when the language inherits the base contentDir.
	languages map[string]string
}

// newFlags builds the base flag layer every config load and build shares. Hugo
// re-applies these over the site's own config, so they neutralize settings that
// would otherwise fail on a site we assemble without its build toolchain:
//
//   - security.allowContent: Hugo's default rejects raw *.html content files
//     (which real sites like letsencrypt ship); we extract rather than serve,
//     so every media type is safe.
//   - caches / outputs: sites pinned to a different Hugo version, or relying on
//     an un-fetched theme module (e.g. kubernetes.io's Docsy), reference cache
//     names and output formats that our Hugo build cannot validate, failing
//     config load. Neither affects the content tree, so we drop them and let
//     Hugo fall back to its defaults.
func newFlags(dir string) config.Provider {
	flags := config.New()
	flags.Set("workingDir", dir)
	flags.Set("security.allowContent", []string{".*"})
	flags.Set("caches", nil)
	flags.Set("outputs", nil)
	return flags
}

// loadConfig loads the site config through allconfig, Hugo's top-level config
// loader, so config discovery, merge semantics, HUGO_* env overrides, and
// language resolution behave exactly like real Hugo. The caller supplies the
// flag layer (see newFlags), letting the loader inject content-dir and build
// overrides before the authoritative load that feeds the hugolib build. found
// is false when the site has no config file, which is not fatal; the returned
// config then carries Hugo's defaults.
func loadConfig(ctx context.Context, flags config.Provider, logger *slog.Logger) (*siteConfig, *allconfig.Configs, bool, error) {
	configs, err := allconfig.LoadConfig(allconfig.ConfigSourceDescriptor{
		Fs:          afero.NewOsFs(),
		Flags:       flags,
		ConfigDir:   "config",
		Environment: "production",
		Logger:      loggers.NewDefault(),
		// Sites importing non-vendored theme modules should still load.
		IgnoreModuleDoesNotExist: true,
	})
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to load site config: %w", err)
	}
	found := len(configs.LoadingInfo.ConfigFiles) > 0
	logger.DebugContext(ctx, "loaded config", "files", configs.LoadingInfo.ConfigFiles)

	base := configs.Base
	cfg := &siteConfig{
		contentDir:  base.ContentDir,
		defaultLang: strings.ToLower(base.DefaultContentLanguage),
		languages:   make(map[string]string, len(configs.LanguageConfigMap)),
	}
	for lang, langCfg := range configs.LanguageConfigMap {
		contentDir := langCfg.ContentDir
		if contentDir == base.ContentDir {
			// Inherited, not a per-language override; a set contentDir marks
			// the language as having its own content tree.
			contentDir = ""
		}
		cfg.languages[strings.ToLower(lang)] = contentDir
	}
	return cfg, configs, found, nil
}
