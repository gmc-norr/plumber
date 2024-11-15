package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing config directories",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		configDir := viper.GetString("config-home")
		fmt.Fprintf(os.Stdout, "# config home directory: %s\n", configDir)
		files, err := os.ReadDir(configDir)
		if err != nil {
			slog.Error("error listing directories", "error", err.Error())
			os.Exit(1)
		}
		tw := tabwriter.NewWriter(os.Stdout, 1, 2, 1, ' ', 0)
		fmt.Fprint(tw, "config\tparent repository\tversion\n")
		fmt.Fprint(tw, "======\t=================\t=======\n")
		didError := false
		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			c, err := plumber.ConfigFromPath(filepath.Join(configDir, f.Name()))
			if err != nil {
				didError = true
				slog.Error("error initialising config", "config", f.Name(), "error", err.Error())
			} else {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Name(), c.Repo, c.Version)
			}
		}
		tw.Flush()
		if didError {
			os.Exit(1)
		}
	},
}
