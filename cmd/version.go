package cmd

import (
	"encoding/json"
	"runtime/debug"

	"github.com/spf13/cobra"
)

type versionInfo struct {
	Version   string
	GoVersion string
}

func newVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:     "version",
		Short:   "Print the version of hugo-to-skill",
		Example: "hugo-to-skill version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				panic("failed to read build info")
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(versionInfo{
				Version:   info.Main.Version,
				GoVersion: info.GoVersion,
			})
		},
	}
	return versionCmd
}
