package nextflow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configRepo      string
	configVersion   string
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
			plumberArgs := args[:cmd.ArgsLenAtDash()]
			nextflowArgs = args[cmd.ArgsLenAtDash():]
			if err := cobra.ExactArgs(1)(cmd, plumberArgs); err != nil {
				return err
			}
			if !plumber.ValidPipelineName(args[0]) {
				return fmt.Errorf("pipeline name should be on the form <org>/<repo>, spaces not allowed")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
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
			if nextflowConfig == "" {
				nextflowConfig = fmt.Sprintf("%s-%s", pipeline.String(), pipelineVersion)
			}
			path := filepath.Join(configDir, nextflowConfig)
			c, err := plumber.ConfigFromPath(path)
			if err != nil {
				if configRepo == "" {
					slog.Error("no config found, and no repo given")
					os.Exit(1)
				}
				if configVersion == "" {
					slog.Error("no config found, and no version given")
					os.Exit(1)
				}
				c = plumber.NewConfig(configRepo, configVersion, path)
				if err := c.Clone(); err != nil {
					slog.Error("error cloning config", "repo", c.Repo, "path", c.LocalPath, "error", err.Error())
					os.Exit(1)
				}
			} else {
				slog.Info("using existing config", "path", c.LocalPath, "version", c.Version)
			}
			if configRepo != "" && c.Repo != configRepo {
				slog.Warn("--config-repo doesn't match with loaded config, versions might mismatch", "config-repo", configRepo, "config", c.Repo)
			}
			if configVersion != "" && c.Version != configVersion {
				slog.Info("checking out config version", "version", configVersion)
				if err := c.Checkout(configVersion); err != nil {
					slog.Error("error checking out config files", "repo", c.Repo, "version", configVersion, "path", c.LocalPath, "error", err.Error())
					os.Exit(1)
				}
			}
			slog.Debug("nextflow config", "path", c.LocalPath, "version", c.Version)

			nfConfig, err := plumber.NewNextflowConfig(pipeline, c)
			if err != nil {
				slog.Error("error initialising Nextflow config", "error", err.Error())
				os.Exit(1)
			}
			nfConfig.Profile = nextflowProfile

			nfPipeline := plumber.NewNextflowPipeline(nfConfig)
			nfPipeline.SetEnv("NEXTFLOW_CONFIG_HOME", filepath.Join(nfConfig.Config.LocalPath, "nextflow"))
			nfPipeline.Workdir = nextflowWorkdir
			if err := nfPipeline.Run(nextflowArgs); err != nil {
				slog.Error("error running pipeline", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	runCmd.Flags().StringVarP(&configRepo, "config-repo", "", "", "URL or path to the config file git repository")
	runCmd.Flags().StringVarP(&configVersion, "config-version", "", "", "tag/branch/commit of the config files to use")
	runCmd.Flags().StringVarP(&pipelineVersion, "version", "", "main", "tag/branch/commit of the pipeline to run")
	runCmd.Flags().StringVarP(&nextflowConfig, "config", "c", "", "name of config to use (defult: \"<org>-<pipeline>-<revision>\")")
	runCmd.Flags().StringVarP(&nextflowWorkdir, "workdir", "d", ".", "directory where the pipeline should be executed")
	runCmd.Flags().StringVarP(&nextflowProfile, "profile", "p", "", "comma-separated list of profiles to use for the execution")
}
