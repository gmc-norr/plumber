package nextflow

import (
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pipelineVersion string
	nextflowConfig  string
	nextflowProfile string
	nextflowWorkdir string
	nextflowArgs    []string

	runCmd = &cobra.Command{
		Use:   "run PIPELINE",
		Short: "Run a Nextflow pipeline",
		Long:  `Run a Nextflow pipeline with a configuration managed by plumber. Any arguments passed after -- will be passed directly to Nextflow.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return err
			}
			var plumberArgs []string
			if n := cmd.ArgsLenAtDash(); n != -1 {
				plumberArgs = args[:cmd.ArgsLenAtDash()]
				nextflowArgs = args[cmd.ArgsLenAtDash():]
			} else {
				plumberArgs = args
			}
			if err := cobra.ExactArgs(1)(cmd, plumberArgs); err != nil {
				return err
			}
			if !plumber.ValidPipelineName(args[0]) {
				return fmt.Errorf("pipeline name should be on the form <org>/<repo>, spaces not allowed")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := viper.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision = pipelineVersion
			if err != nil {
				slog.Error("error parsing pipeline name", "error", err.Error())
			}
			// Check that a pipeline with this revision exists on Github
			if err := pipeline.Check(); err != nil {
				slog.Error("pipeline not found", "name", args[0], "error", err.Error())
				os.Exit(1)
			}
			h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s", configRepo, configVersion, pipeline.Repo, pipelineVersion)))
			path := filepath.Join(configDir, fmt.Sprintf("%x", h))
			slog.Debug("attempting to read config", "path", path)
			pf, err := plumber.ReadPlumberFile(path)
			if err != nil {
				slog.Info("no existing config found, attempting download")
				if configRepo == "" {
					slog.Error("no config found, and no repo given")
					os.Exit(1)
				}
				if configVersion == "" {
					slog.Error("no config found, and no version given")
					os.Exit(1)
				}
				pf = plumber.NewPlumberFile()
				pf.Path = path
				pf.Pipelines = append(pf.Pipelines, plumber.PipelineConfigMetadata{
					Pipeline: pipeline,
					Version:  pipeline.Revision,
				})
				repo, err := plumber.NewGitRepo(configRepo)
				if err != nil {
					slog.Error("error initialising git repo", "error", err)
					os.Exit(1)
				}
				err = plumber.DownloadConfig(repo, configVersion, &pf)
				if err != nil {
					slog.Error("error downloading config", "repo", pf.Source, "path", pf.Path, "error", err)
					os.Exit(1)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}
			if configRepo != "" && pf.Source != configRepo {
				slog.Warn("--config-repo doesn't match with loaded config, versions might mismatch", "config-repo", configRepo, "config", pf.Pipelines[0].Pipeline.Repo)
			}
			if configVersion != "" && pf.Revision != configVersion {
				// TODO: If the config versions are different, then a new config should be
				// downloaded to ensure that you're working on what you think you are working on.
				// This will require a change when it comes to how the directory names are generated.
				slog.Warn("config versions differ", "requested", configVersion, "existing", pf.Revision)
			}
			slog.Debug("nextflow config", "path", pf.Path, "version", pf.Pipelines[0].Version)

			nfPipeline := plumber.NewNextflowPipeline(pf)
			nfPipeline.SetEnv("PLUMBER_ASSETS_PATH", filepath.Join(pf.Path, "assets"))
			nfPipeline.Workdir = nextflowWorkdir
			if err := nfPipeline.Run(nextflowProfile, nextflowArgs); err != nil {
				slog.Error("error running pipeline", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	runCmd.Flags().StringVarP(&pipelineVersion, "version", "", "main", "tag/branch/commit of the pipeline to run")
	runCmd.Flags().StringVarP(&nextflowConfig, "config", "c", "", "name of config to use (defult: \"<org>-<pipeline>-<revision>\")")
	runCmd.Flags().StringVarP(&nextflowWorkdir, "workdir", "d", ".", "directory where the pipeline should be executed")
	runCmd.Flags().StringVarP(&nextflowProfile, "profile", "p", "", "comma-separated list of profiles to use for the execution")
}
