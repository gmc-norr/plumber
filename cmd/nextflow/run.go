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
			configRepo, _ := cmd.Flags().GetString("config-repo")
			configVersion, _ := cmd.Flags().GetString("config-version")
			configDir := viper.GetString("config-home")
			pipeline, err := plumber.ParsePipelineName(args[0])
			pipeline.Revision, _ = cmd.Flags().GetString("version")
			noCleanup, _ := cmd.Flags().GetBool("no-cleanup")

			webhookUrl := viper.GetString("webhook-url")
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
					err := webhook.SetCertificates(viper.GetString("certs"))
					cobra.CheckErr(err)
				}
			}

			slog.Debug("webhook", "client", webhook)

			if err != nil {
				slog.Error("error parsing pipeline name", "error", err.Error())
			}
			h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%s-%s", configRepo, configVersion, pipeline.Repo, pipeline.Revision)))
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

			slog.Debug("nextflow config", "path", pf.Path, "version", pf.Pipelines[0].Version)

			nfPipeline := plumber.NewNextflowPipeline(pf)
			nfPipeline.SetEnv("PLUMBER_ASSETS_PATH", filepath.Join(pf.Path, "assets"))
			nfPipeline.Workdir, _ = cmd.Flags().GetString("workdir")
			nfPipeline.Workdir, _ = filepath.Abs(nfPipeline.Workdir)
			profiles, _ := cmd.Flags().GetString("profile")
			if webhook != nil {
				msg := plumber.WebhookMessage{
					Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
					PipelineVersion: nfPipeline.Revision,
					Workdir:         nfPipeline.Workdir,
					Message:         "pipeline started",
					MessageType:     plumber.MessageStart,
					Success:         true,
				}
				err := webhook.Send(msg)
				if err != nil {
					slog.Error("failed to send end message to webhook", "error", err)
				}
			}
			if err := nfPipeline.Run(profiles, nextflowArgs, webhook); err != nil {
				if webhook != nil {
					msg := plumber.WebhookMessage{
						Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
						PipelineVersion: nfPipeline.Revision,
						Workdir:         nfPipeline.Workdir,
						Message:         "pipeline failed",
						MessageType:     plumber.MessageEnd,
						Success:         false,
						Error:           err,
					}
					err := webhook.Send(msg)
					if err != nil {
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
						PipelineVersion: nfPipeline.Revision,
						Workdir:         nfPipeline.Workdir,
						Message:         "cleaning up intermediate files",
						MessageType:     plumber.MessageProgress,
						Success:         true,
					}
					err := webhook.Send(msg)
					if err != nil {
						slog.Error("failed to send progress message to webhook", "error", err)
					}
				}
				cobra.CheckErr(nfPipeline.Cleanup())
			}
			if webhook != nil {
				msg := plumber.WebhookMessage{
					Pipeline:        nfPipeline.Pipelines[0].Pipeline.String(),
					PipelineVersion: nfPipeline.Revision,
					Workdir:         nfPipeline.Workdir,
					Message:         "pipeline finished",
					MessageType:     plumber.MessageEnd,
					Success:         true,
				}
				err := webhook.Send(msg)
				if err != nil {
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
