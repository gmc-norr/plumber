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

func NewListCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing configs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			listRemote, _ := cmd.Flags().GetBool("remote")
			tw := tabwriter.NewWriter(os.Stdout, 1, 2, 2, ' ', 0)

			if listRemote {
				configRepo, _ := cmd.Flags().GetString("config-repo")
				configVersion, _ := cmd.Flags().GetString("config-version")
				slog.Debug("repo", "url", configRepo, "version", configVersion)
				repo, err := plumber.NewGitRepo(configRepo)
				if err != nil {
					return fmt.Errorf("error initialising git repo: %w", err)
				}
				pf, err := plumber.DownloadConfigRepo(&repo, configVersion, v.GetString("cache-home"))
				if err != nil {
					return fmt.Errorf("error getting config repo: %w", err)
				}

				if _, err := fmt.Fprint(tw, "pipeline\tversion\tengine\n"); err != nil {
					slog.Error("failed to write header", "error", err)
				}
				if _, err := fmt.Fprint(tw, "========\t=======\t======\n"); err != nil {
					slog.Error("failed to write header separator", "error", err)
				}
				for _, pipeline := range pf.Pipelines {
					if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", pipeline.Pipeline.Repo, pipeline.Version, pipeline.Engine, pf.Source, pf.Revision); err != nil {
						slog.Error("failed to write pipeline line", "error", err)
					}
				}
				_ = tw.Flush()
				return nil
			}

			configHome := v.GetString("config-home")
			if _, err := fmt.Fprintf(os.Stdout, "# config home directory: %s\n", configHome); err != nil {
				slog.Error("failed to write config directory", "error", err)
			}
			files, err := os.ReadDir(configHome)
			if err != nil {
				return fmt.Errorf("error listing directories: %w", err)
			}
			didError := false
			if _, err := fmt.Fprint(tw, "id\tpipeline\tversion\tengine\tsource-repo\trevision\tvalid\n"); err != nil {
				slog.Error("failed to write header", "error", err)
			}
			if _, err := fmt.Fprint(tw, "==\t========\t=======\t======\t===========\t========\t=====\n"); err != nil {
				slog.Error("failed to write header separator", "error", err)
			}
			for _, f := range files {
				if !f.IsDir() {
					continue
				}
				configDir := filepath.Join(configHome, f.Name())
				pf, err := plumber.ReadPlumberFile(filepath.Join(configDir, plumber.PlumberFileName))
				if err != nil {
					var pfError plumber.PlumberFileNotFound
					if errors.As(err, &pfError) {
						// Ignore the directory if no plumber file is found
						slog.Debug("no plumber file found", "error", err.Error())
					} else if !errors.Is(err, plumber.ErrPlumberFileFormat) {
						didError = true
						slog.Error("error initialising config", "config", f.Name(), "error", err, "directory", configDir)
					} else {
						if _, err := fmt.Fprintf(tw, "%s\t\t\t\t%s\t%s\t%s\n", f.Name(), pf.Source, pf.Revision, "❌"); err != nil {
							slog.Error("failed to write config line", "error", err)
						}
					}
				} else {
					if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", f.Name(), pf.Pipelines[0].Pipeline.Repo, pf.Pipelines[0].Version, pf.Pipelines[0].Engine, pf.Source, pf.Revision, "✔️"); err != nil {
						slog.Error("failed to write config line", "error", err)
					}
				}
			}
			_ = tw.Flush()
			if didError {
				return fmt.Errorf("encountered errors while listing configs")
			}
			return nil
		},
	}

	cmd.Flags().Bool("remote", false, "list available remote configs")

	return cmd
}
