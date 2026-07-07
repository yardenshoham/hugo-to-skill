package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/yardenshoham/hugo-to-skill/pkg/site"
	"github.com/yardenshoham/hugo-to-skill/pkg/skill"
	"github.com/yardenshoham/hugo-to-skill/pkg/source"
)

func newGenerateCmd() *cobra.Command {
	var (
		output        string
		contentPath   string
		lang          string
		name          string
		description   string
		license       string
		compatibility string
		allowedTools  string
		metadata      map[string]string
		notes         []string
		includeDrafts bool
	)

	cmd := &cobra.Command{
		Use:   "generate SITE_PATH_OR_GIT_URL",
		Short: "Generate an AI Agent skill from a Hugo-based website",
		Long: `Generate an agentskills.io-compatible skill directory from a Hugo-based website.

The site source may be a local directory or a git URL (shallow-cloned with go-git).
Content pages are copied verbatim into the skill's references/ directory, mirroring
the Hugo content tree, and a SKILL.md with hierarchical indexes is generated
following the Agent Skills specification for progressive disclosure.`,
		Example: `  # Local clone, whole site
  hugo-to-skill generate ./website --output ./skills/longhorn

  # Straight from GitHub, scoped to one content section
  hugo-to-skill generate https://github.com/longhorn/website --content-path kb --output ./skills/longhorn-kb

  # Multilingual site, one language
  hugo-to-skill generate https://github.com/kubernetes/website --lang en --content-path docs --output ./skills/k8s-docs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, _ := cmd.Context().Value(loggerKey{}).(*slog.Logger)
			if logger == nil {
				logger = slog.New(slog.DiscardHandler)
			}

			logger.InfoContext(cmd.Context(), "resolving site source", "source", args[0])
			dir, cleanup, err := source.Resolve(cmd.Context(), args[0], logger)
			if err != nil {
				return fmt.Errorf("failed to resolve site source: %w", err)
			}
			defer cleanup()

			loaded, err := site.Load(cmd.Context(), dir, site.LoadOptions{
				ContentPath:   contentPath,
				Lang:          lang,
				IncludeDrafts: includeDrafts,
			}, logger)
			if err != nil {
				return fmt.Errorf("failed to load site: %w", err)
			}

			config := skill.Config{
				Name:          name,
				Description:   description,
				License:       license,
				Compatibility: compatibility,
				AllowedTools:  allowedTools,
				Metadata:      metadata,
				Notes:         notes,
			}

			if err := skill.GenerateDir(cmd.Context(), loaded, output, config, logger); err != nil {
				return fmt.Errorf("failed to generate skill: %w", err)
			}

			logger.InfoContext(cmd.Context(), "skill generation complete", "output", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output directory for the generated skill")
	_ = cmd.MarkFlagRequired("output")
	cmd.Flags().StringVar(&contentPath, "content-path", "", "Restrict extraction to a subpath of the content dir (e.g. kb, docs)")
	cmd.Flags().StringVar(&lang, "lang", "", "Language for multilingual sites (default: site's defaultContentLanguage)")
	cmd.Flags().StringVar(&name, "name", "", "Override skill name (default: slugified site title)")
	cmd.Flags().StringVar(&description, "description", "", "Override skill description (default: built from site title and params.description)")
	cmd.Flags().StringVar(&license, "license", "", "License identifier (e.g., Apache-2.0)")
	cmd.Flags().StringVar(&compatibility, "compatibility", "", "Compatibility requirements description")
	cmd.Flags().StringVar(&allowedTools, "allowed-tools", "", "Allowed tools specification")
	cmd.Flags().StringToStringVar(&metadata, "metadata", nil, "Metadata key=value pairs (can be repeated)")
	cmd.Flags().StringSliceVar(&notes, "notes", nil, "Usage notes (can be repeated)")
	cmd.Flags().BoolVar(&includeDrafts, "include-drafts", false, "Include pages Hugo would not publish (draft/future/expired)")

	return cmd
}
