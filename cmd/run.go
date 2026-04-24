package main

import (
	"context"
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

type Runner interface {
	Run(ctx context.Context, profile string, extraArgs []string) error
	Cleanup() error
}

func NewRunCmd(v *viper.Viper) *cobra.Command {
	var engineArgs []string
	cmd := &cobra.Command{
		Use:   "run PIPELINE [flags]",
		Short: "Run a pipeline",
		Long:  `Run a pipeline with a configuration managed by plumber. Any arguments passed after -- will be passed directly to the workflow engine.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return err
			}
			var plumberArgs []string
			if n := cmd.ArgsLenAtDash(); n != -1 {
				plumberArgs = args[:cmd.ArgsLenAtDash()]
				engineArgs = args[cmd.ArgsLenAtDash():]
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
			ctx := cmd.Context()
			workdir, _ := cmd.Flags().GetString("workdir")
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion := v.GetString("config-version")
			configDir := v.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			stringId, _ := cmd.Flags().GetString("analysis-id")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			if err != nil {
				return fmt.Errorf("error parsing pipeline name: %w", err)
			}

			workdir, err = filepath.Abs(workdir)
			if err != nil {
				return fmt.Errorf("failed to resolve workdir: %w", err)
			}

			var analysisId uuid.UUID
			if stringId == "" {
				analysisId = uuid.New()
			} else {
				analysisId, err = uuid.Parse(stringId)
				if err != nil {
					return fmt.Errorf("failed to parse analysis id: %w", err)
				}
			}

			analysis := plumber.NewAnalysis().
				WithId(analysisId).
				WithUser(os.Getenv("USER")).
				WithPipeline(pipeline).
				WithState(plumber.StatePending).
				WithWorkdir(workdir)

			if a, err := analysis.Read(); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to read analysis file: %w", err)
				}
			} else if a.Id != analysis.Id {
				return fmt.Errorf("existing analysis id does not match current analysis id")
			}

			if err := analysis.Write(); err != nil {
				return fmt.Errorf("failed to write analysis file: %w", err)
			}

			webhookUrl := v.GetString("webhook-url")
			var webhookErr error
			var webhook *plumber.Webhook
			slog.Info("webhook config", "url", webhookUrl, "certs", v.GetString("certs"))
			if webhookUrl == "" {
				slog.Info("no webhook url defined, won't send any information")
			} else {
				webhook = plumber.NewSt2Webhook(webhookUrl, v.GetString("webhook-api-key"))
				webhook.PlumberVersion = v.GetString("plumber-version")
				if v.GetBool("webhook-no-verify") {
					slog.Warn("disabling webhook TLS")
					webhook.DisableTLSVerification()
				} else if v.GetString("certs") != "" {
					slog.Debug("setting certificates for webhook client", "path", v.GetString("certs"))
					webhookErr = webhook.SetCertificates(v.GetString("certs"))
				}
			}

			slog.Debug("webhook", "client", webhook)

			defer func() {
				if ctx.Err() != nil {
					slog.Error("context error", "error", ctx.Err())
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
				}
				if webhook != nil && ctx.Err() != nil {
					msg := plumber.WebhookMessage{
						AnalysisId:      analysis.Id,
						Pipeline:        analysis.Pipeline.Repo,
						PipelineVersion: analysis.Pipeline.Revision,
						Workdir:         analysis.Workdir,
						Message:         "execution failed",
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           plumber.NewMarshableError(ctx.Err()),
					}
					if err := webhook.Send(ctx, msg); err != nil {
						slog.Error("failed to send init message to webhook", "error", err)
					}
				}
			}()

			if webhook != nil {
				msg := plumber.WebhookMessage{
					AnalysisId:      analysis.Id,
					Pipeline:        analysis.Pipeline.Repo,
					PipelineVersion: analysis.Pipeline.Revision,
					Workdir:         analysis.Workdir,
					Message:         "initialising plumber",
					MessageType:     plumber.MessageInit,
					Success:         webhookErr == nil,
					Error:           plumber.NewMarshableError(webhookErr),
				}
				if err := webhook.Send(ctx, msg); err != nil {
					return fmt.Errorf("failed to send init message to webhook: %w", err)
				}
			}

			if webhookErr != nil {
				return fmt.Errorf("failed to initialise webhook: %w", webhookErr)
			}

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
				err = plumber.DownloadConfig(ctx, repo, configVersion, &pf, v.GetString("cache-home"))
				if err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					slog.Error("error downloading config repo", "repo", pf.Source, "path", pf.Path, "error", err)
					return fmt.Errorf("error downloading config: %w", err)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}

			slog.Debug("nextflow config", "path", pf.Path, "version", pf.Pipelines[0].Version)

			var runner Runner
			var profile string
			switch pf.Pipelines[0].Engine {
			case "nextflow":
				nfPipeline := plumber.NewNextflowPipeline(pf)
				nfPipeline.SetEnv("PLUMBER_PIPELINE_ASSETS", filepath.Join(pf.Path, "assets"))
				nfPipeline.SetEnv("NEXTFLOW_CONFIG_HOME", pf.Path)
				nfPipeline.Workdir = workdir
				profile, _ = cmd.Flags().GetString("profile")
				runner = Runner(&nfPipeline)
			case "snakemake":
				env := pyenv.Environment{
					Version: pyenv.VersionFromString(pf.Pipelines[0].Executor.Version),
					Name:    fmt.Sprintf("%s-%s", strings.ToLower(pipeline.Pipeline), pipeline.Revision),
				}
				slog.Info("creating python virtual environment", "name", env.Name, "python_version", env.Version)
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

				if err := smPipeline.Download(); err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("failed to download pipeline: %w", err)
				}

				if err := smPipeline.Install(); err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					return fmt.Errorf("failed to install pipeline: %w", err)
				}
				slog.Debug("pipeline environment", "env", smPipeline.Env)
				profile, _ = cmd.Flags().GetString("profile")
				runner = Runner(&smPipeline)
			default:
				return fmt.Errorf("unsupported workflow engine: %s", pf.Pipelines[0].Engine)
			}

			if webhook != nil {
				msg := plumber.WebhookMessage{
					AnalysisId:      analysis.Id,
					Pipeline:        analysis.Pipeline.Repo,
					PipelineVersion: analysis.Pipeline.Revision,
					Workdir:         analysis.Workdir,
					Message:         "pipeline started",
					MessageType:     plumber.MessageStart,
					Success:         true,
				}
				if err := webhook.Send(ctx, msg); err != nil {
					slog.Error("failed to send message to webhook", "error", err)
				}
			}

			analysis.SetState(plumber.StateRunning)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}

			if err := runner.Run(ctx, profile, engineArgs); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				var loglines []string
				var runErr plumber.PipelineRunError
				if errors.As(err, &runErr) {
					loglines = runErr.Log
				}
				if webhook != nil {
					msg := plumber.WebhookMessage{
						AnalysisId:      analysis.Id,
						Pipeline:        analysis.Pipeline.Repo,
						PipelineVersion: analysis.Pipeline.Revision,
						Workdir:         analysis.Workdir,
						Message:         fmt.Sprintf("pipeline failed, end of log:\n%s", strings.Join(loglines, "\n")),
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           plumber.NewMarshableError(err),
					}
					if err := webhook.Send(ctx, msg); err != nil {
						slog.Error("failed to send message to webhook", "error", err)
					}
				}
				return fmt.Errorf("error running pipeline: %w", err)
			}

			if !noCleanup {
				analysis.SetState(plumber.StateRunning)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				if webhook != nil {
					msg := plumber.WebhookMessage{
						AnalysisId:      analysis.Id,
						Pipeline:        analysis.Pipeline.Repo,
						PipelineVersion: analysis.Pipeline.Revision,
						Workdir:         analysis.Workdir,
						Message:         "cleaning up intermediate files",
						MessageType:     plumber.MessageProgress,
						Success:         true,
					}
					if err := webhook.Send(ctx, msg); err != nil {
						slog.Error("failed to send message to webhook", "error", err)
					}
				}
				if err := runner.Cleanup(); err != nil {
					return fmt.Errorf("failed to clean up pipeline files: %w", err)
				}
			}

			analysis.SetState(plumber.StateSuccess)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}
			if webhook != nil {
				msg := plumber.WebhookMessage{
					AnalysisId:      analysis.Id,
					Pipeline:        analysis.Pipeline.Repo,
					PipelineVersion: analysis.Pipeline.Revision,
					Workdir:         analysis.Workdir,
					Message:         "pipeline finished",
					MessageType:     plumber.MessageEnd,
					Success:         true,
				}
				if err := webhook.Send(ctx, msg); err != nil {
					slog.Error("failed to send message to webhook", "error", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("analysis-id", "", "External UUID of the analyis, if one is not given, an ID will be generated")
	cmd.Flags().Bool("no-cleanup", false, "Do not clean up intermediate files on successful execution")
	cmd.Flags().StringP("profile", "p", "", "Comma-separated list of profiles to use for the execution")
	cmd.Flags().String("version", "main", "Tag/branch/commit of the pipeline to run")
	cmd.Flags().StringP("workdir", "d", ".", "Working directory of the execution")

	return cmd
}
