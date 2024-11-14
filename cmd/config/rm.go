package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rmCmd = &cobra.Command{
	Use:   "rm NAME",
	Short: "Remove a config",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configDir := viper.GetString("config-home")
		path := filepath.Join(configDir, args[0])
		_, err := plumber.ConfigFromPath(path)
		if err != nil {
			slog.Error("error initialising config", "config", args[0], "error", err.Error())
			os.Exit(1)
		}
		if err := os.RemoveAll(path); err != nil {
			slog.Error("error removing config", "config", args[0], "error", err.Error())
			os.Exit(1)
		}
		slog.Info("removed config", "name", args[0], "path", path)
	},
}
