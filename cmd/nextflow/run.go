package nextflow

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gmc-norr/plumber"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRunCmd(v *viper.Viper) *cobra.Command {
	var nextflowArgs []string
	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			workdir, _ := cmd.Flags().GetString("workdir")
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := v.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			stringId, _ := cmd.Flags().GetString("analysis-id")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			if err != nil {
				slog.Error("error parsing pipeline name", "error", err.Error())
			}

			workdir, err = filepath.Abs(workdir)
			cobra.CheckErr(err)

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

			// Only fail on error for the first write, only log future write errors
			if err := analysis.Write(); err != nil {
				return fmt.Errorf("failed to write analysis file: %w", err)
			}

			slog.Debug("initialising plumber", "path", workdir, "analysis", analysis)

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
					slog.Debug("setting certificates for client", "path", v.GetString("certs"))
					webhookErr = webhook.SetCertificates(v.GetString("certs"))
				}
			}

			slog.Debug("webhook", "client", webhook)
			slog.Debug("setting up signal handler")
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
				s := <-c
				slog.Error("signal received, cleaning up and exiting", "signal", s)
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				if webhook != nil {
					msg := plumber.WebhookMessage{
						AnalysisId:      analysis.Id,
						Pipeline:        pipeline.String(),
						PipelineVersion: pipeline.Revision,
						Workdir:         workdir,
						Message:         fmt.Sprintf("process killed by signal: %s", s),
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           plumber.NewMarshableError(fmt.Errorf("killed by signal %s", s)),
					}
					if err := webhook.Send(msg); err != nil {
						slog.Error("failed to send end message to webhook", "error", err)
					}
				}
				os.Exit(1)
			}()

			if webhook != nil {
				msg := plumber.WebhookMessage{
					AnalysisId:      analysis.Id,
					Pipeline:        pipeline.String(),
					PipelineVersion: pipeline.Revision,
					Workdir:         workdir,
					Message:         "initialising plumber",
					MessageType:     plumber.MessageInit,
					Success:         webhookErr == nil,
					Error:           plumber.NewMarshableError(webhookErr),
				}
				if err := webhook.Send(msg); err != nil {
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
				err = plumber.DownloadConfig(repo, configVersion, &pf, v.GetString("cache-home"))
				if err != nil {
					analysis.SetState(plumber.StateFailed)
					if err := analysis.Write(); err != nil {
						slog.Error("failed to write analysis file", "error", err)
					}
					slog.Error("error downloading config", "repo", pf.Source, "path", pf.Path, "error", err)
					return fmt.Errorf("error downloading config: %w", err)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}

			slog.Debug("nextflow config", "path", pf.Path, "version", pf.Pipelines[0].Version)

			nfPipeline := plumber.NewNextflowPipeline(pf)
			nfPipeline.SetEnv("PLUMBER_PIPELINE_ASSETS", filepath.Join(pf.Path, "assets"))
			nfPipeline.SetEnv("NEXTFLOW_CONFIG_HOME", pf.Path)
			nfPipeline.Workdir = workdir
			profiles, _ := cmd.Flags().GetString("profile")
			if webhook != nil {
				msg := plumber.WebhookMessage{
					AnalysisId:      analysis.Id,
					Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
					PipelineVersion: nfPipeline.Pipelines[0].Version,
					Workdir:         nfPipeline.Workdir,
					Message:         "pipeline started",
					MessageType:     plumber.MessageStart,
					Success:         true,
				}
				if err := webhook.Send(msg); err != nil {
					slog.Error("failed to send end message to webhook", "error", err)
				}
			}
			analysis.SetState(plumber.StateRunning)
			if err := analysis.Write(); err != nil {
				slog.Error("failed to write analysis file", "error", err)
			}
			if lastLogLines, err := nfPipeline.Run(profiles, nextflowArgs); err != nil {
				analysis.SetState(plumber.StateFailed)
				if err := analysis.Write(); err != nil {
					slog.Error("failed to write analysis file", "error", err)
				}
				if webhook != nil {
					msg := plumber.WebhookMessage{
						AnalysisId:      analysis.Id,
						Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
						PipelineVersion: nfPipeline.Pipelines[0].Version,
						Workdir:         nfPipeline.Workdir,
						Message:         fmt.Sprintf("pipeline failed, end of log:\n%s", strings.Join(lastLogLines, "\n")),
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           plumber.NewMarshableError(err),
					}
					if err := webhook.Send(msg); err != nil {
						slog.Error("failed to send end message to webhook", "error", err)
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
						Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
						PipelineVersion: nfPipeline.Pipelines[0].Version,
						Workdir:         nfPipeline.Workdir,
						Message:         "cleaning up intermediate files",
						MessageType:     plumber.MessageProgress,
						Success:         true,
					}
					if err := webhook.Send(msg); err != nil {
						slog.Error("failed to send progress message to webhook", "error", err)
					}
				}
				if err := nfPipeline.Cleanup(); err != nil {
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
					Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
					PipelineVersion: nfPipeline.Pipelines[0].Version,
					Workdir:         nfPipeline.Workdir,
					Message:         "pipeline finished",
					MessageType:     plumber.MessageEnd,
					Success:         true,
				}
				if err := webhook.Send(msg); err != nil {
					slog.Error("failed to send end message to webhook", "error", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringP("version", "", "main", "tag/branch/commit of the pipeline to run")
	cmd.Flags().StringP("workdir", "d", ".", "directory where the pipeline should be executed")
	cmd.Flags().StringP("profile", "p", "", "comma-separated list of profiles to use for the execution")
	cmd.Flags().String("analysis-id", "", "external UUID of the analysis. If one is not given, and ID will be generated.")
	cmd.Flags().Bool("no-cleanup", false, "do not clean up intermediate files on successful execution")

	return cmd
}
