package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise plumber",
	Run: func(cmd *cobra.Command, args []string) {
		configHome := viper.GetString("config-home")
		info, err := os.Stat(configHome)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(configHome, 0o755); err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
				slog.Info("created config home directory", "config-home", configHome)
				os.Exit(0)
			}
			slog.Error(err.Error())
			os.Exit(1)
		}
		if info.IsDir() {
			slog.Warn("config home directory already exists", "config-home", configHome)
			os.Exit(1)
		}
	},
}
