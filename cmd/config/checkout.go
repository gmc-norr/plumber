package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout NAME",
	Short: "Checkout a commitish for an existing config",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configVersion := viper.GetString("config-version")
		configDir := viper.GetString("config-home")
		path := filepath.Join(configDir, args[0])
		c, err := plumber.ConfigFromPath(path)
		if err != nil {
			slog.Error("error initialising config", "config", args[0])
			os.Exit(1)
		}
		if err := c.Checkout(configVersion); err != nil {
			slog.Error("error checking out revision", "config", args[0], "path", path, "error", err.Error())
			os.Exit(1)
		}
	},
}
