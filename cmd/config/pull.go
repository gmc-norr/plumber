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
	pullName string

	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Download config files",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configRepo := viper.GetString("config-repo")
			configVersion := viper.GetString("config-version")
			configDir := viper.GetString("config-home")
			if pullName == "" {
				slog.Error("no local name provided")
				os.Exit(1)
			}
			path := filepath.Join(configDir, pullName)
			slog.Debug("flags", "pullRepo", configRepo, "pullRev", configVersion)
			config := plumber.NewConfig(configRepo, configVersion, path)
			if config.Exists() {
				slog.Warn("config already exists", "path", path)
				return
			}
			if err := config.Clone(); err != nil {
				slog.Error("error cloning config", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	pullCmd.Flags().StringVarP(&pullName, "name", "n", "", "local config name to use")
}
