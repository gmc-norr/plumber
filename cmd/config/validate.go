package config

import (
	"log/slog"
	"os"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate PLUMBERFILE",
	Short: "Validate a plumberfile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		slog.Info("validating configuration", "plumberfile", args[0])
		if err := plumber.ValidatePlumberYaml(args[0]); err != nil {
			slog.Error("plumberfile validation failed", "error", err)
			os.Exit(1)
		}
		slog.Info("validation successful", "plumberfile", args[0])
	},
}
