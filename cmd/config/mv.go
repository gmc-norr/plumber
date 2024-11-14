package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mvCmd = &cobra.Command{
	Use:   "mv FROM TO",
	Short: "Move a config file directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := viper.GetString("config-home")
		from := filepath.Join(configDir, args[0])
		to := filepath.Join(configDir, args[1])

		_, err := plumber.ConfigFromPath(from)
		if err != nil {
			return err
		}
		toConfig, err := plumber.ConfigFromPath(to)
		if err == nil && toConfig.Exists() {
			return fmt.Errorf("destination already exists")
		}

		return os.Rename(from, to)
	},
}
