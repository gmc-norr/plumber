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
	fetchName string

	fetchCmd = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch updates from the remote",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			if fetchName != "all" {
				path := filepath.Join(configDir, fetchName)
				c, err := plumber.ConfigFromPath(path)
				if err != nil {
					slog.Error(err.Error())
				}
				slog.Info("fetching updates", "name", fetchName)
				if err := c.Fetch(); err != nil {
					slog.Error(err.Error())
				}
			}
			didError := false
			files, err := os.ReadDir(configDir)
			if err != nil {
				slog.Error(err.Error())
			}
			for _, f := range files {
				if !f.IsDir() {
					continue
				}
				c, err := plumber.ConfigFromPath(filepath.Join(configDir, f.Name()))
				if err != nil {
					didError = true
					slog.Error("error initialising config", "config", f.Name(), "error", err.Error())
					continue
				}
				slog.Info("fetching updates", "name", f.Name())
				if err := c.Fetch(); err != nil {
					didError = true
					slog.Error("error when fetching updates", "error", err.Error())
				}
			}
			if didError {
				os.Exit(1)
			}
		},
	}
)

func init() {
	fetchCmd.Flags().StringVarP(&fetchName, "name", "n", "all", "config to fetch updates for")
}
