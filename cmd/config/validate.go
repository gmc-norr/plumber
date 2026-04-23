package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewValidateCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate PLUMBERFILE",
		Short: "Validate a plumberfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			isHash, _ := cmd.Flags().GetBool("hash")
			var path string
			if isHash {
				configDir := v.GetString("config-home")
				path = filepath.Join(configDir, args[0], plumber.PlumberFileName)
			} else {
				path = args[0]
			}
			slog.Info("validating configuration", "plumberfile", path)
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			b, err := io.ReadAll(f)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			if err := plumber.ValidatePlumberFile(b); err != nil {
				return fmt.Errorf("plumberfile validation failed: %w", err)
			}
			slog.Info("validation successful", "plumberfile", path)
			return nil
		},
	}

	cmd.Flags().Bool("hash", false, "argument is a config hash, not a plumberfile")

	return cmd
}
