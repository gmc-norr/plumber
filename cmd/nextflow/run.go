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
	nextflowRev        string
	nextflowConfig     string
	nextflowConfigRepo string
	nextflowConfigRev  string
	nextflowProfile    string
	nextflowWorkdir    string

	runCmd = &cobra.Command{
		Use:   "run PIPELINE SAMPLESHEET",
		Short: "Run a Nextflow pipeline",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
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
			pipeline.Revision = nextflowRev
			if err != nil {
				slog.Error("error parsing pipeline name", "error", err.Error())
			}
			// Check that a pipeline with this revision exists on Github
			if err := pipeline.Check(); err != nil {
				slog.Error("pipeline not found", "name", args[0], "error", err.Error())
				os.Exit(1)
			}
			if nextflowConfig == "" {
				nextflowConfig = fmt.Sprintf("%s-%s", pipeline.String(), nextflowRev)
			}
			path := filepath.Join(configDir, nextflowConfig)
			c, err := plumber.ConfigFromPath(path)
			if err != nil {
				c = plumber.NewConfig(nextflowConfigRepo, nextflowConfigRev, path)
				if err := c.Clone(); err != nil {
					slog.Error("error cloning config", "repo", c.Repo, "path", c.LocalPath, "error", err.Error())
					os.Exit(1)
				}
			} else {
				slog.Info("using existing config", "path", c.LocalPath, "revision", c.Revision)
			}
			if c.Revision != nextflowConfigRev {
				slog.Info("checking out config revision", "revision", nextflowConfigRev)
				if err := c.Checkout(nextflowConfigRev); err != nil {
					slog.Error("error checking out config files", "repo", c.Repo, "revision", nextflowConfigRev, "path", c.LocalPath, "error", err.Error())
					os.Exit(1)
				}
			}
			slog.Debug("nextflow config", "path", c.LocalPath, "revision", c.Revision)

			nfConfig, err := plumber.NewNextflowConfig(pipeline, c)
			if err != nil {
				slog.Error("error initialising Nextflow config", "error", err.Error())
				os.Exit(1)
			}
			nfConfig.Profile = nextflowProfile

			nfPipeline := plumber.NewNextflowPipeline(nfConfig)
			nfPipeline.SetEnv("NEXTFLOW_CONFIG_HOME", filepath.Join(nfConfig.Config.LocalPath, "nextflow"))
			nfPipeline.Workdir = nextflowWorkdir
			if err := nfPipeline.Run(); err != nil {
				slog.Error("error running pipeline", "error", err.Error())
				os.Exit(1)
			}
		},
	}
)

func init() {
	runCmd.Flags().StringVarP(&nextflowRev, "revision", "r", "main", "tag/branch/commit of the pipeline to run")
	runCmd.Flags().StringVarP(&nextflowConfig, "config", "c", "", "name of config to use (defult: \"<org>-<pipeline>-<revision>\")")
	runCmd.Flags().StringVar(&nextflowConfigRepo, "config-repo", "gmc-norr/config-files", "name of config repo to use")
	runCmd.Flags().StringVar(&nextflowConfigRev, "config-revision", "main", "tag/branch/commit of config repo to use")
	runCmd.Flags().StringVarP(&nextflowWorkdir, "workdir", "d", ".", "directory where the pipeline should be executed")
	runCmd.Flags().StringVarP(&nextflowProfile, "profile", "p", "", "comma-separated list of profiles to use for the execution")
}
