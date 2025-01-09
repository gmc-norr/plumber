package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	downloadName  string
	downloadForce bool

	downloadCmd = &cobra.Command{
		Use:   "download PIPELINE VERSION",
		Short: "Download config files for a specific version of a pipeline",
		Args: func(cmd *cobra.Command, args []string) error {
			return cobra.ExactArgs(2)(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			configRepo := viper.GetString("config-repo")
			configVersion := viper.GetString("config-version")
			configDir := viper.GetString("config-home")
			if downloadName == "" {
				slog.Error("no local name provided")
				os.Exit(1)
			}
			path := filepath.Join(configDir, downloadName)
			slog.Debug("flags", "repo", configRepo, "version", configVersion)
			config := plumber.NewConfig(configRepo, configVersion, path)
			if config.Exists() && !downloadForce {
				slog.Error("config already exists", "path", path)
				os.Exit(1)
			}
			if err := config.Download(args[0], args[1]); err != nil {
				slog.Error("error downloading config", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	downloadCmd.Flags().StringVarP(&downloadName, "name", "n", "", "local config name to use")
	downloadCmd.Flags().BoolVarP(&downloadForce, "force", "f", false, "local config name to use")
}
