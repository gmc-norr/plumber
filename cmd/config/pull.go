package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configRepo string
	configRev  string
	pullName   string

	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Download config files",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			if pullName == "" {
				pullName = fmt.Sprintf("%s-%s", strings.Split(configRepo, "/")[1], configRev)
			}
			path := filepath.Join(configDir, pullName)
			slog.Debug("flags", "pullRepo", configRepo, "pullRev", configRev)
			config := plumber.NewConfig(configRepo, configRev, path)
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
	pullCmd.Flags().StringVar(&configRepo, "repository", "gmc-norr/config-files", "repository to pull on the form \"<org>/<repo>\"")
	pullCmd.Flags().StringVarP(&configRev, "revision", "r", "main", "tag/branch/commit to check out")
	pullCmd.Flags().StringVarP(&pullName, "name", "n", "", "local config name to use")
}
