package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var validateCmd = &cobra.Command{
	Use:   "validate PLUMBERFILE",
	Short: "Validate a plumberfile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		isHash, _ := cmd.Flags().GetBool("hash")
		var path string
		if isHash {
			configDir := viper.GetString("config-home")
			path = filepath.Join(configDir, args[0], plumber.PlumberFileName)
		} else {
			path = args[0]
		}
		slog.Info("validating configuration", "plumberfile", path)
		f, err := os.Open(path)
		if err != nil {
			slog.Error("failed to open file", "path", path, "error", err)
			os.Exit(1)
		}
		b, err := io.ReadAll(f)
		if err != nil {
			slog.Error("failed to read file", "path", path, "error", err)
			os.Exit(1)
		}
		if err := plumber.ValidatePlumberFile(b); err != nil {
			slog.Error("plumberfile validation failed", "error", err)
			os.Exit(1)
		}
		slog.Info("validation successful", "plumberfile", path)
	},
}

func init() {
	validateCmd.Flags().Bool("hash", false, "argument is a config hash, not a plumberfile")
}
