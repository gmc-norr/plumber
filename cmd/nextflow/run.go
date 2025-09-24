package nextflow

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	nextflowArgs []string

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
			workdir, _ := cmd.Flags().GetString("workdir")
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := viper.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			workdir, _ = filepath.Abs(workdir)
			slog.Debug("initialising plumber", "path", workdir)

			webhookUrl := viper.GetString("webhook-url")
			var webhookErr error
			var webhook *plumber.Webhook
			slog.Info("webhook config", "url", webhookUrl, "certs", viper.GetString("certs"))
			if webhookUrl == "" {
				slog.Info("no webhook url defined, won't send any information")
			} else {
				webhook = plumber.NewSt2Webhook(webhookUrl, viper.GetString("webhook-api-key"))
				webhook.PlumberVersion = viper.GetString("plumber-version")
				if viper.GetBool("webhook-no-verify") {
					slog.Warn("disabling webhook TLS")
					webhook.DisableTLSVerification()
				} else if viper.GetString("certs") != "" {
					slog.Debug("setting certificates for client", "path", viper.GetString("certs"))
					webhookErr = webhook.SetCertificates(viper.GetString("certs"))
				}
			}

			slog.Debug("webhook", "client", webhook)
			slog.Debug("setting up signal handler")
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
				s := <-c
				slog.Error("signal received, cleaning up and exiting", "signal", s)
				if webhook != nil {
					msg := plumber.WebhookMessage{
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
					Pipeline:        pipeline.String(),
					PipelineVersion: pipeline.Revision,
					Workdir:         workdir,
					Message:         "initialising plumber",
					MessageType:     plumber.MessageInit,
					Success:         webhookErr == nil,
					Error:           plumber.NewMarshableError(webhookErr),
				}
				if err := webhook.Send(msg); err != nil {
					slog.Error("failed to send init message to webhook, aborting", "error", err)
					os.Exit(1)
				}
			}

			if webhookErr != nil {
				slog.Error("failed to initialise webhook", "error", webhookErr)
				os.Exit(1)
			}

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
					slog.Error("error downloading config", "repo", pf.Source, "path", pf.Path, "error", err)
					os.Exit(1)
				}
			} else {
				slog.Info("using existing config", "path", pf.Path, "version", pf.Pipelines[0].Version)
			}

			slog.Debug("nextflow config", "path", pf.Path, "version", pf.Pipelines[0].Version)

			nfPipeline := plumber.NewNextflowPipeline(pf)
			nfPipeline.SetEnv("PLUMBER_ASSETS_PATH", filepath.Join(pf.Path, "assets"))
			nfPipeline.Workdir = workdir
			profiles, _ := cmd.Flags().GetString("profile")
			if webhook != nil {
				msg := plumber.WebhookMessage{
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
			if err := nfPipeline.Run(profiles, nextflowArgs, webhook); err != nil {
				if webhook != nil {
					msg := plumber.WebhookMessage{
						Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
						PipelineVersion: nfPipeline.Pipelines[0].Version,
						Workdir:         nfPipeline.Workdir,
						Message:         "pipeline failed",
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           plumber.NewMarshableError(err),
					}
					if err := webhook.Send(msg); err != nil {
						slog.Error("failed to send end message to webhook", "error", err)
					}
				}
				slog.Error("error running pipeline", "error", err.Error())
				os.Exit(1)
			}
			if !noCleanup {
				if webhook != nil {
					msg := plumber.WebhookMessage{
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
				cobra.CheckErr(nfPipeline.Cleanup())
			}
			if webhook != nil {
				msg := plumber.WebhookMessage{
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
		},
	}
)

func init() {
	runCmd.Flags().StringP("version", "", "main", "tag/branch/commit of the pipeline to run")
	runCmd.Flags().StringP("workdir", "d", ".", "directory where the pipeline should be executed")
	runCmd.Flags().StringP("profile", "p", "", "comma-separated list of profiles to use for the execution")
	runCmd.Flags().Bool("no-cleanup", false, "do not clean up intermediate files on successful execution")
}
