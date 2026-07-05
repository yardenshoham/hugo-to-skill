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

// siteConfig holds the config values the generator needs, extracted from
// Hugo's own config pipeline. contentDir and defaultLang always carry Hugo's
// defaults ("content", "en") so consumers never re-apply them.
type siteConfig struct {
	title       string
	baseURL     string
	description string // params.description
	contentDir  string
	defaultLang string // defaultContentLanguage, lowercase
	languages   map[string]langConfig
}

// langConfig holds a language's overrides from languages.<lang>.*.
type langConfig struct {
	contentDir string
	title      string
}

// loadConfig loads the site config through allconfig, Hugo's top-level config
// loader, so config discovery, merge semantics, HUGO_* env overrides, and
// language resolution behave exactly like real Hugo. found is false when the
// site has no config file, which is not fatal; the returned config then
// carries Hugo's defaults.
func loadConfig(ctx context.Context, dir string, logger *slog.Logger) (*siteConfig, bool, error) {
	flags := config.New()
	flags.Set("workingDir", dir)

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
		return nil, true, fmt.Errorf("loading site config: %w", err)
	}
	found := len(configs.LoadingInfo.ConfigFiles) > 0
	logger.DebugContext(ctx, "loaded config", "files", configs.LoadingInfo.ConfigFiles)

	base := configs.Base
	cfg := &siteConfig{
		title:       base.Title,
		baseURL:     base.BaseURL,
		contentDir:  base.ContentDir,
		defaultLang: strings.ToLower(base.DefaultContentLanguage),
		languages:   make(map[string]langConfig, len(configs.LanguageConfigMap)),
	}
	cfg.description, _ = base.Params["description"].(string)
	for lang, langCfg := range configs.LanguageConfigMap {
		contentDir := langCfg.ContentDir
		if contentDir == base.ContentDir {
			// Inherited, not a per-language override; a set contentDir marks
			// the language as having its own content tree.
			contentDir = ""
		}
		cfg.languages[strings.ToLower(lang)] = langConfig{
			contentDir: contentDir,
			title:      langCfg.Title,
		}
	}
	return cfg, found, nil
}
