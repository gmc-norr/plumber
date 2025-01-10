package config

import (
	"errors"
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
	Short: "List existing configs",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		configHome := viper.GetString("config-home")
		fmt.Fprintf(os.Stdout, "# config home directory: %s\n", configHome)
		files, err := os.ReadDir(configHome)
		if err != nil {
			slog.Error("error listing directories", "error", err.Error())
			os.Exit(1)
		}
		tw := tabwriter.NewWriter(os.Stdout, 1, 2, 2, ' ', 0)
		fmt.Fprint(tw, "name\tpipeline\tversion\tsource-repo\trevision\n")
		fmt.Fprint(tw, "====\t========\t=======\t===========\t========\n")
		didError := false
		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			configDir := filepath.Join(configHome, f.Name())
			pf, err := plumber.ReadPlumberFile(configDir)
			if err != nil {
				var pfError plumber.PlumberFileNotFound
				if errors.As(err, &pfError) {
					// Ignore the directory if no plumber file is found
					slog.Debug("no plumber file found", "error", err.Error())
				} else {
					didError = true
					slog.Error("error initialising config", "config", f.Name(), "error", err.Error(), "directory", configDir)
				}
			} else {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.Name(), pf.Pipelines[0].Name, pf.Pipelines[0].Version, pf.Source, pf.Revision)
			}
		}
		tw.Flush()
		if didError {
			os.Exit(1)
		}
	},
}
