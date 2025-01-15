package config

import (
	"errors"
	"fmt"
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
		pf, err := plumber.ReadPlumberFile(path)
		if err != nil {
			var pfNotFound plumber.PlumberFileNotFound
			if errors.As(err, &pfNotFound) {
				slog.Error("no such config", "path", path, "error", pfNotFound)
				os.Exit(1)
			}
			slog.Error("error reading plumber file", "path", path, "error", err)
			os.Exit(1)
		}
		if err := os.RemoveAll(pf.Path); err != nil {
			slog.Error("error removing config", "config", args[0], "error", err.Error())
			os.Exit(1)
		}
		slog.Info("removed config", "id", fmt.Sprintf("%x", pf.Hash()), "path", path)
	},
}
