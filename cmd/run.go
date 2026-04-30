package main

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gmc-norr/plumber"
	"github.com/gmc-norr/plumber/pyenv"
	"github.com/google/uuid"
	"github.com/maehler/webhook"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Runner interface {
	Run(ctx context.Context, profile string, extraArgs []string) error
	Cleanup() error
}

func sendMessage(ctx context.Context, client *webhook.Client, payload plumber.WebhookMessage) error {
	if client == nil {
		return nil
	}

	res, err := client.SendContext(ctx, payload)
	if err == nil {
		slog.Info("sent webhook message", "message_type", payload.MessageType, "attempts", res.Attempts, "status", res.Response.Status)
	} else {
		slog.Error("failed to send webhook message", "message_type", payload.MessageType, "attempts", res.Attempts, "error", err)
	}
	return err
}

func NewRunCmd(v *viper.Viper) *cobra.Command {
	var webhookClient *webhook.Client
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			slog.Debug("setting up webhook client")
			webhookURL := v.GetString("webhook-url")
			webhookApiKey := v.GetString("webhook-api-key")
			if webhookURL == "" {
				slog.Info("no webhook url supplied, will not send any messages")
				return nil
			}
			headers := make(http.Header)
			keyParts := strings.Split(webhookApiKey, "=")
			if webhookApiKey != "" && len(keyParts) != 2 {
				return fmt.Errorf("invalid api key format")
			}
			if webhookApiKey != "" {
				headers.Add(keyParts[0], keyParts[1])
			}
			webhookClient = webhook.NewClient(webhookURL, webhook.ClientOpts.WithHeaders(headers))
			slog.Debug("webhook", "url", webhookClient.URL, "method", webhookClient.Method)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctx := cmd.Context()
			workdir, _ := cmd.Flags().GetString("workdir")
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion := v.GetString("config-version")
			configDir := v.GetString("config-home")
			pipeline, _ := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			stringId, _ := cmd.Flags().GetString("analysis-id")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			analysis := plumber.NewAnalysis().
				WithUser(os.Getenv("USER")).
				WithPipeline(pipeline).
				WithState(plumber.StatePending)

			webhookMessage := plumber.WebhookMessage{
				AnalysisId:      analysis.Id,
				Pipeline:        analysis.Pipeline.Repo,
				PipelineVersion: analysis.Pipeline.Revision,
				Workdir:         analysis.Workdir,
			}

			slog.Info("executing as", "user", analysis.User)

			defer func() {
				if ctx.Err() != nil || err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}

					webhookMessage.Message = fmt.Sprintf("execution failed: %s", err)
					if runErr, ok := errors.AsType[plumber.PipelineRunError](err); ok {
						webhookMessage.Message = fmt.Sprintf("%s, end of log: \n%s", webhookMessage.Message, strings.Join(runErr.Log, "\n"))
					}

					webhookMessage.Workdir = analysis.Workdir
					webhookMessage.AnalysisId = analysis.Id
					webhookMessage.Time = time.Now()
					webhookMessage.MessageType = plumber.MessageEnd
					webhookMessage.Success = false
					webhookMessage.Error = plumber.NewMarshableError(errors.Join(ctx.Err(), err))
					_ = sendMessage(context.Background(), webhookClient, webhookMessage)
				}
			}()

			workdir, err = filepath.Abs(workdir)
			if err != nil {
				return fmt.Errorf("failed to resolve workdir: %w", err)
			}
			analysis = analysis.WithWorkdir(workdir)
			webhookMessage.Workdir = analysis.Workdir

			slog.Info("running in", "workdir", analysis.Workdir)

			var analysisId uuid.UUID
			if stringId == "" {
				analysisId = uuid.New()
			} else {
				analysisId, err = uuid.Parse(stringId)
				if err != nil {
					return fmt.Errorf("failed to parse analysis id: %w", err)
				}
			}
			analysis = analysis.WithId(analysisId)
			webhookMessage.AnalysisId = analysis.Id

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

			webhookMessage.Time = time.Now()
			webhookMessage.Message = "initialising plumber"
			webhookMessage.MessageType = plumber.MessageInit
			webhookMessage.Success = true
			webhookMessage.Error = plumber.NewMarshableError(nil)
			if err := sendMessage(ctx, webhookClient, webhookMessage); err != nil {
				// Fail if the first message fails. This is likely indicative of a misconfiguration.
				return fmt.Errorf("failed to send webhook message: %w", err)
			}

			slog.Info("setting up configuration", "pipeline", pipeline.Repo, "version", pipeline.Revision)

			h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s", configRepo, configVersion, pipeline.Repo, pipeline.Revision)))
			path := filepath.Join(configDir, fmt.Sprintf("%x", h))
			slog.Info("config files", "repo", configRepo, "local_path", path)
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
					slog.Error("error downloading config repo", "repo", repo.Url, "path", pf.Path, "error", err)
					return fmt.Errorf("error downloading config: %w", err)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}

			// Check that the config checksums match
			checksums, err := plumber.ReadChecksums(filepath.Join(pf.Path, plumber.ChecksumFile))
			if err != nil {
				return fmt.Errorf(`failed to read config file checksums, download them again with "plumber config download --force %s %s"`, pf.Pipelines[0].Pipeline.Repo, pf.Pipelines[0].Version)
			}
			if err := checksums.Check(pf.Path); err != nil {
				errs := err.(interface{ Unwrap() []error }).Unwrap()
				for _, e := range errs {
					// TODO: this should eventually be upgraded to a fatal error
					slog.Warn("error checking config file checksums", "error", e)
				}
			}

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

			webhookMessage.Time = time.Now()
			webhookMessage.Message = "pipeline started"
			webhookMessage.MessageType = plumber.MessageStart
			webhookMessage.Success = true
			webhookMessage.Error = plumber.NewMarshableError(nil)
			_ = sendMessage(ctx, webhookClient, webhookMessage)

			analysis.SetState(plumber.StateRunning)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}

			if err := runner.Run(ctx, profile, engineArgs); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				return fmt.Errorf("error running pipeline: %w", err)
			}

			if !noCleanup {
				analysis.SetState(plumber.StateRunning)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				webhookMessage.Time = time.Now()
				webhookMessage.Message = "cleaning up intermediate files"
				webhookMessage.MessageType = plumber.MessageProgress
				webhookMessage.Success = true
				webhookMessage.Error = plumber.NewMarshableError(nil)
				_ = sendMessage(ctx, webhookClient, webhookMessage)
				if err := runner.Cleanup(); err != nil {
					return fmt.Errorf("failed to clean up pipeline files: %w", err)
				}
			}

			analysis.SetState(plumber.StateSuccess)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}
			webhookMessage.Time = time.Now()
			webhookMessage.Message = "pipeline finished"
			webhookMessage.MessageType = plumber.MessageEnd
			webhookMessage.Success = true
			webhookMessage.Error = plumber.NewMarshableError(nil)
			_ = sendMessage(ctx, webhookClient, webhookMessage)

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
