package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing config directories",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		configDir := viper.GetString("config-home")
		fmt.Printf("# config home directory: %s\n", configDir)
		files, err := os.ReadDir(configDir)
		if err != nil {
			slog.Error("error listing directories", "error", err.Error())
			os.Exit(1)
		}
		didError := false
		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			c, err := plumber.ConfigFromPath(filepath.Join(configDir, f.Name()))
			if err != nil {
				didError = true
				slog.Error("error initialising config", "config", f.Name(), "error", err.Error())
			} else {
				fmt.Printf("%s (%s@%s)\n", f.Name(), c.Repo, c.Revision)
			}
		}
		if didError {
			os.Exit(1)
		}
	},
}
