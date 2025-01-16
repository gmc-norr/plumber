package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var downloadCmd = &cobra.Command{
	Use:   "download PIPELINE VERSION",
	Short: "Download config files for a specific version of a pipeline",
	Args: func(cmd *cobra.Command, args []string) error {
		return cobra.ExactArgs(2)(cmd, args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		configRepo, _ := cmd.Flags().GetString("config-repo")
		configVersion, _ := cmd.Flags().GetString("config-version")
		configDir := viper.GetString("config-home")
		forceDownload, _ := cmd.Flags().GetBool("force")
		repo, err := plumber.NewGitRepo(configRepo)
		if err != nil {
			slog.Error("error initialising git repo", "error", err)
			os.Exit(1)
		}
		slog.Debug("flags", "repo", configRepo, "version", configVersion)
		pipeline, err := plumber.ParsePipelineName(args[0])
		if err != nil {
			slog.Error("error parsing pipeline name", "error", err)
			os.Exit(1)
		}

		pf := plumber.NewPlumberFile()
		pf.Source = repo.Url.String()
		pf.Revision = configVersion
		pf.Pipelines = append(pf.Pipelines, plumber.PipelineConfigMetadata{
			Pipeline: pipeline,
			Version:  args[1],
		})

		h := pf.Hash()
		pf.Path = filepath.Join(configDir, fmt.Sprintf("%x", h))

		if pf.Exists() && !forceDownload {
			slog.Error("config already exists", "path", pf.Path)
			os.Exit(1)
		}
		if err := plumber.DownloadConfig(repo, configVersion, &pf); err != nil {
			slog.Error("error downloading config", "error", err.Error())
			os.Exit(1)
		}
		slog.Info("pipeline config downloaded", "engine", pf.Pipelines[0].Engine, "name", pf.Pipelines[0].Pipeline, "path", pf.Path)
	},
}

func init() {
	downloadCmd.Flags().BoolP("force", "f", false, "overwrite existing config")
}
