package hydra

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gmc-norr/plumber"
	"github.com/gmc-norr/plumber/pyenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	snakemakeArgs []string

	runCmd = &cobra.Command{
		Use:   "run PIPELINE",
		Short: "Run a Hydra Genetics pipeline",
		Long:  `Run a Hydra Genetics pipeline with a configuration managed by plumber. Any arguments passed after -- will be passed directly to Snakemake.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return err
			}
			var plumberArgs []string
			if n := cmd.ArgsLenAtDash(); n != -1 {
				plumberArgs = args[:cmd.ArgsLenAtDash()]
				snakemakeArgs = args[cmd.ArgsLenAtDash():]
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
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			if err != nil {
				slog.Error("error parsing pipeline name", "error", err.Error())
			}
			h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s", configRepo, configVersion, pipeline.Repo, pipeline.Revision)))
			path := filepath.Join(configDir, fmt.Sprintf("%x", h))
			slog.Debug("attempting to read config", "path", path)
			pf, err := plumber.ReadPlumberFile(filepath.Join(path, plumber.PlumberFileName))
			if err != nil {
				if errors.Is(err, plumber.ErrPlumberFileFormat) {
					slog.Error("plumberfile validation failed", "error", err)
					os.Exit(1)
				}
				slog.Info("no existing config found, attempting download")
				if configRepo == "" {
					slog.Error("no config found, and no repo given")
					os.Exit(1)
				}
				if configVersion == "" {
					slog.Error("no config found, and no version given")
					os.Exit(1)
				}
				pf = plumber.PlumberFile{}
				pf.Path = path
				pf.Pipelines = append(pf.Pipelines, plumber.PipelineMetadata{
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
					slog.Error("error downloading config", "repo", repo, "path", pf.Path, "error", err)
					os.Exit(1)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}

			slog.Debug("plumberfile", "struct", pf)
			slog.Debug("snakemake config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			slog.Debug("executor", "name", pf.Pipelines[0].Executor.Name, "version", pf.Pipelines[0].Executor.Version)

			// Check python environment
			env := pyenv.Environment{
				Version: pyenv.VersionFromString(pf.Pipelines[0].Executor.Version),
				Name:    fmt.Sprintf("%s-%s", strings.ToLower(pipeline.Pipeline), pipeline.Revision),
			}
			slog.Info("checking python virtual environment", "name", env.Name, "python_version", env.Version)
			exists, err := env.Exists()
			if err != nil {
				slog.Error("virtual environment error", "error", err)
				os.Exit(1)
			}
			if !exists {
				slog.Info("creating virtual environment", "name", env.Name, "python_version", env.Version)
				if err := env.Create(); err != nil {
					slog.Error("failed to set up python environment", "error", err)
					os.Exit(1)
				}
			} else {
				slog.Info("virtual environment already exists", "name", env.Name, "python_version", env.Version)
			}

			smPipeline := plumber.NewSnakemakePipeline(pf)
			smPipeline.Path = os.ExpandEnv(fmt.Sprintf("$HOME/.local/share/plumber/%s/%s-%s", pipeline.Organisation, pipeline.Pipeline, pipeline.Revision))

			if home, ok := os.LookupEnv("HOME"); ok {
				smPipeline.SetEnv("HOME", home)
			}
			smPipeline.SetEnv("PLUMBER_ASSETS_PATH", filepath.Join(pf.Path, "assets"))
			smPipeline.SetEnv("PYENV_VERSION", env.Name)
			smPipeline.SetEnv("PLUMBER_PIPELINE_HOME", smPipeline.Path)
			smPipeline.SetEnv("PLUMBER_PIPELINE_CONFIG", smPipeline.PlumberFile.Path)

			// Download the pipeline
			if err := smPipeline.Download(); err != nil {
				slog.Error("failed to download pipeline", "error", err)
				os.Exit(1)
			}

			// Install the pipeline
			if err := smPipeline.Install(); err != nil {
				slog.Error("failed to install pipeline", "error", err)
				os.Exit(1)
			}

			smPipeline.Workdir, _ = cmd.Flags().GetString("workdir")
			slog.Debug("pipeline environment", "env", smPipeline.Env)
			profiles, _ := cmd.Flags().GetString("profile")
			if err := smPipeline.Run(profiles, snakemakeArgs); err != nil {
				slog.Error("error running pipeline", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	runCmd.Flags().StringP("version", "", "main", "tag/branch/commit of the pipeline to run")
	runCmd.Flags().StringP("workdir", "d", "", "directory where the pipeline should be executed")
	runCmd.Flags().StringP("profile", "p", "", "comma-separated list of profiles to use for the execution")
}
