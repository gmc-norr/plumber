package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewInitCmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:        "init",
		Short:      "Initialise plumber",
		Deprecated: "initialisation is now done in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			configHome := v.GetString("config-home")
			info, err := os.Stat(configHome)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.MkdirAll(configHome, 0o755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}
					slog.Info("created config home directory", "config-home", configHome)
					return nil
				}
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("config home directory already exists: %s", configHome)
			}
			return nil
		},
	}
}
