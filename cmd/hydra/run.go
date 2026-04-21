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
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRunCmd(v *viper.Viper) *cobra.Command {
	var snakemakeArgs []string
	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := v.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			stringId, _ := cmd.Flags().GetString("analysis-id")
			workdir, _ := cmd.Flags().GetString("workdir")

			if err != nil {
				return fmt.Errorf("error parsing pipeline name: %w", err)
			}

			workdir, err = filepath.Abs(workdir)
			cobra.CheckErr(err)

			var analysisId uuid.UUID
			if stringId == "" {
				analysisId = uuid.New()
			} else {
				analysisId, err = uuid.Parse(stringId)
				if err != nil {
					return err
				}
			}

			analysis := plumber.NewAnalysis().
				WithId(analysisId).
				WithUser(os.Getenv("USER")).
				WithPipeline(pipeline).
				WithWorkdir(workdir).
				WithState(plumber.StatePending)

			if a, err := analysis.Read(); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to read analysis file: %w", err)
				}
			} else if a.Id != analysis.Id {
				return fmt.Errorf("existing analysis id does not match current analysis id")
			}

			// Only fail on error for the first write, only log future write errors
			err = analysis.Write()
			cobra.CheckErr(err)

			slog.Debug("initialising plumber", "path", workdir, "analysis", analysis)

			h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s", configRepo, configVersion, pipeline.Repo, pipeline.Revision)))
			path := filepath.Join(configDir, fmt.Sprintf("%x", h))
			slog.Debug("attempting to read config", "path", path)
			pf, err := plumber.ReadPlumberFile(filepath.Join(path, plumber.PlumberFileName))
			if err != nil {
				if errors.Is(err, plumber.ErrPlumberFileFormat) {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("plumberfile validation failed: %w", err)
				}
				slog.Info("no existing config found, attempting download")
				if configRepo == "" {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("no config found, and no repo given")
				}
				if configVersion == "" {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("no config found, and no version given")
				}
				pf = plumber.PlumberFile{}
				pf.Path = path
				pf.Pipelines = append(pf.Pipelines, plumber.PipelineMetadata{
					Pipeline: pipeline,
					Version:  pipeline.Revision,
				})
				repo, err := plumber.NewGitRepo(configRepo)
				if err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("error initialising git repo: %w", err)
				}
				err = plumber.DownloadConfig(repo, configVersion, &pf, v.GetString("cache-home"))
				if err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					slog.Error("error downloading config", "repo", repo, "path", pf.Path, "error", err)
					return fmt.Errorf("error downloading config")
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
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				return fmt.Errorf("virtual environment error: %w", err)
			}
			if !exists {
				slog.Info("creating virtual environment", "name", env.Name, "python_version", env.Version)
				if err := env.Create(); err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("failed to set up python environment: %w", err)
				}
			} else {
				slog.Info("virtual environment already exists", "name", env.Name, "python_version", env.Version)
			}

			smPipeline := plumber.NewSnakemakePipeline(pf)
			smPipeline.Workdir = analysis.Workdir
			smPipeline.Path = os.ExpandEnv(fmt.Sprintf("$HOME/.local/share/plumber/%s/%s-%s", pipeline.Organisation, pipeline.Pipeline, pipeline.Revision))

			if home, ok := os.LookupEnv("HOME"); ok {
				smPipeline.SetEnv("HOME", home)
			}
			smPipeline.SetEnv("PLUMBER_PIPELINE_ASSETS", filepath.Join(pf.Path, "assets"))
			smPipeline.SetEnv("PYENV_VERSION", env.Name)
			smPipeline.SetEnv("PLUMBER_PIPELINE_HOME", smPipeline.Path)
			smPipeline.SetEnv("PLUMBER_PIPELINE_CONFIG", smPipeline.PlumberFile.Path)

			// Download the pipeline
			if err := smPipeline.Download(); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				return fmt.Errorf("failed to download pipeline: %w", err)
			}

			// Install the pipeline
			if err := smPipeline.Install(); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				return fmt.Errorf("failed to install pipeline: %w", err)
			}

			slog.Debug("pipeline environment", "env", smPipeline.Env)
			profiles, _ := cmd.Flags().GetString("profile")
			if err := smPipeline.Run(profiles, snakemakeArgs); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				return fmt.Errorf("error running pipeline: %w", err)
			}
			analysis.SetState(plumber.StateSuccess)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}
			return nil
		},
	}

	cmd.Flags().StringP("version", "", "main", "tag/branch/commit of the pipeline to run")
	cmd.Flags().StringP("workdir", "d", ".", "directory where the pipeline should be executed")
	cmd.Flags().StringP("profile", "p", "", "comma-separated list of profiles to use for the execution")
	cmd.Flags().String("analysis-id", "", "external UUID of the analysis. If one is not given, and ID will be generated.")

	return cmd
}
