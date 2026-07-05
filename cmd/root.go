package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

type loggerKey struct{}

func newRootCmd() *cobra.Command {
	var debug bool

	var rootCmd = &cobra.Command{
		Use:          "hugo-to-skill",
		Short:        "Generate an AI Agent skill from a Hugo-based website",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{
				Level: level,
			}))
			cmd.SetContext(context.WithValue(cmd.Context(), loggerKey{}, logger))
		},
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newGenerateCmd())
	return rootCmd
}

func Execute() {
	// cobra already prints the error (SilenceErrors is unset).
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
