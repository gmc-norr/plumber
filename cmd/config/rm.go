package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRmCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir := v.GetString("config-home")
			path := filepath.Join(configDir, args[0])
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("error locating config: %w", err)
			}
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("error removing config: %s: %w", args[0], err)
			}
			slog.Info("removed config", "id", args[0], "path", path)
			return nil
		},
	}

	return cmd
}
