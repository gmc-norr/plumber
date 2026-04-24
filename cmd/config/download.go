package config

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewDownloadCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download PIPELINE VERSION",
		Short: "Download config files for a specific version of a pipeline",
		Args: func(cmd *cobra.Command, args []string) error {
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := v.GetString("config-home")
			forceDownload, _ := cmd.Flags().GetBool("force")
			repo, err := plumber.NewGitRepo(configRepo)
			if err != nil {
				return fmt.Errorf("error initialising git repo: %w", err)
			}
			slog.Debug("flags", "repo", configRepo, "version", configVersion)
			pipeline, err := plumber.ParsePipelineName(args[0])
			if err != nil {
				return fmt.Errorf("error parsing pipeline name: %w", err)
			}

			var pf plumber.PlumberFile
			pf.Source = repo.Url.String()
			pf.Revision = configVersion
			pf.Pipelines = append(pf.Pipelines, plumber.PipelineMetadata{
				Pipeline: pipeline,
				Version:  args[1],
			})

			h := pf.Hash()
			pf.Path = filepath.Join(configDir, fmt.Sprintf("%x", h))

			if pf.Exists() && !forceDownload {
				return fmt.Errorf("config already exists: %s", pf.Path)
			}
			if err := plumber.DownloadConfig(ctx, repo, configVersion, &pf, v.GetString("cache-home")); err != nil {
				return fmt.Errorf("error downloading config: %w", err)
			}
			slog.Info("pipeline config downloaded", "engine", pf.Pipelines[0].Engine, "name", pf.Pipelines[0].Pipeline, "path", pf.Path)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "overwrite existing config")

	return cmd
}
