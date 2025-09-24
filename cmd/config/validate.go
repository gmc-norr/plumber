package config

import (
	"io"
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
		f, err := os.Open(args[0])
		b, err := io.ReadAll(f)
		if err != nil {
			slog.Error("failed to open file", "path", args[0], "error", err)
			os.Exit(1)
		}
		if err := plumber.ValidatePlumberFile(b); err != nil {
			slog.Error("plumberfile validation failed", "error", err)
			os.Exit(1)
		}
		slog.Info("validation successful", "plumberfile", args[0])
	},
}
