package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func config() error {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configHome = filepath.Join(home, ".config")
	}

	viper.SetDefault("config_home", filepath.Join(configHome, "plumber"))
	viper.SetDefault("loglevel", "WARN")

	viper.SetEnvPrefix("plumber")
	viper.MustBindEnv("config_home")
	viper.MustBindEnv("loglevel")
	viper.MustBindEnv("github_token")

	return nil
}

func logger() error {
	logOpts := slog.HandlerOptions{}

	logLevel := viper.GetString("loglevel")
	switch strings.ToLower(logLevel) {
	case "debug":
		logOpts.Level = slog.LevelDebug
	case "info":
		logOpts.Level = slog.LevelInfo
	case "warn":
		logOpts.Level = slog.LevelWarn
	case "error":
		logOpts.Level = slog.LevelError
	default:
		return fmt.Errorf("invalid log level: %s", logLevel)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &logOpts))
	slog.SetDefault(logger)
	return nil
}

var (
	configFetchName string
	configPullName  string
	configRepo      string
	configRev       string

	rootCmd = &cobra.Command{
		Use:     "plumber",
		Short:   "Run pipelines",
		Version: "0.1.0", // x-release-please-version
	}

	configCommand = &cobra.Command{
		Use:   "config",
		Short: "Manage pipeline configuration files",
	}

	configListCommand = &cobra.Command{
		Use:   "list",
		Short: "List existing config directories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			fmt.Printf("# config home directory: %s\n", configDir)
			files, err := os.ReadDir(configDir)
			if err != nil {
				slog.Error("error listing directories", "error", err.Error())
				os.Exit(1)
			}
			didError := false
			for _, f := range files {
				if !f.IsDir() {
					continue
				}
				c, err := plumber.ConfigFromPath(filepath.Join(configDir, f.Name()))
				if err != nil {
					didError = true
					slog.Error("error initialising config", "config", f.Name(), "error", err.Error())
				} else {
					fmt.Printf("%s (%s@%s)\n", f.Name(), c.Repo, c.Revision)
				}
			}
			if didError {
				os.Exit(1)
			}
		},
	}

	configPullCommand = &cobra.Command{
		Use:   "pull",
		Short: "Download config files",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			if configPullName == "" {
				configPullName = fmt.Sprintf("%s-%s", strings.Split(configRepo, "/")[1], configRev)
			}
			path := filepath.Join(configDir, configPullName)
			slog.Debug("flags", "pullRepo", configRepo, "pullRev", configRev)
			config := plumber.NewConfig(configRepo, configRev, path)
			if config.Exists() {
				slog.Warn("config already exists", "path", path)
				return
			}
			if err := config.Clone(); err != nil {
				slog.Error("error cloning config", "error", err.Error())
				os.Exit(1)
			}
		},
	}

	configCheckoutCommand = &cobra.Command{
		Use:   "checkout NAME",
		Short: "Checkout a commitish for an existing config",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			path := filepath.Join(configDir, args[0])
			c, err := plumber.ConfigFromPath(path)
			if err != nil {
				slog.Error("error initialising config", "config", args[0])
				os.Exit(1)
			}
			if err := c.Checkout(configRev); err != nil {
				slog.Error("error checking out revision", "config", args[0], "path", path, "error", err.Error())
				os.Exit(1)
			}
		},
	}

	configMvCommand = &cobra.Command{
		Use:   "mv FROM TO",
		Short: "Move a config file directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir := viper.GetString("config_home")
			from := filepath.Join(configDir, args[0])
			to := filepath.Join(configDir, args[1])

			_, err := plumber.ConfigFromPath(from)
			if err != nil {
				return err
			}
			toConfig, err := plumber.ConfigFromPath(to)
			if err == nil && toConfig.Exists() {
				return fmt.Errorf("destination already exists")
			}

			return os.Rename(from, to)
		},
	}

	configFetchCommand = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch updates from the remote",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			if configFetchName != "all" {
				path := filepath.Join(configDir, configFetchName)
				c, err := plumber.ConfigFromPath(path)
				if err != nil {
					slog.Error(err.Error())
				}
				slog.Info("fetching updates", "name", configFetchName)
				if err := c.Fetch(); err != nil {
					slog.Error(err.Error())
				}
			}
			didError := false
			files, err := os.ReadDir(configDir)
			if err != nil {
				slog.Error(err.Error())
			}
			for _, f := range files {
				if !f.IsDir() {
					continue
				}
				c, err := plumber.ConfigFromPath(filepath.Join(configDir, f.Name()))
				if err != nil {
					didError = true
					slog.Error("error initialising config", "config", f.Name(), "error", err.Error())
					continue
				}
				slog.Info("fetching updates", "name", f.Name())
				if err := c.Fetch(); err != nil {
					didError = true
					slog.Error("error when fetching updates", "error", err.Error())
				}
			}
			if didError {
				os.Exit(1)
			}
		},
	}

	configRmCommand = &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a config",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configDir := viper.GetString("config_home")
			path := filepath.Join(configDir, args[0])
			_, err := plumber.ConfigFromPath(path)
			if err != nil {
				slog.Error("error initialising config", "config", args[0], "error", err.Error())
				os.Exit(1)
			}
			if err := os.RemoveAll(path); err != nil {
				slog.Error("error removing config", "config", args[0], "error", err.Error())
				os.Exit(1)
			}
			slog.Info("removed config", "name", args[0], "path", path)
		},
	}

	nextflowRev        string
	nextflowConfig     string
	nextflowConfigRepo string
	nextflowConfigRev  string
	nextflowWorkdir    string
	nextflowProfile    string

	nextflowCommand = &cobra.Command{
		Use:   "nextflow",
		Short: "Run and manage Nextflow pipelines",
	}

	nextflowRunCommand = &cobra.Command{
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
			configDir := viper.GetString("config_home")
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

func initConfigDir() error {
	configDir := viper.GetString("config_home")
	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(configDir, 0755)
		} else {
			return err
		}
	}
	return nil
}

func initCobra() {
	configPullCommand.Flags().StringVar(&configRepo, "repository", "gmc-norr/config-files", "repository to pull on the form \"<org>/<repo>\"")
	configPullCommand.Flags().StringVarP(&configRev, "revision", "r", "main", "tag/branch/commit to check out")
	configPullCommand.Flags().StringVarP(&configPullName, "name", "n", "", "local config name to use")
	configCheckoutCommand.Flags().StringVarP(&configRev, "revision", "r", "main", "tag/branch/commit to check out")
	configFetchCommand.Flags().StringVarP(&configFetchName, "name", "n", "all", "config to fetch updates for")

	configCommand.AddCommand(configListCommand)
	configCommand.AddCommand(configPullCommand)
	configCommand.AddCommand(configCheckoutCommand)
	configCommand.AddCommand(configMvCommand)
	configCommand.AddCommand(configFetchCommand)
	configCommand.AddCommand(configRmCommand)

	nextflowRunCommand.Flags().StringVarP(&nextflowRev, "revision", "r", "main", "tag/branch/commit of the pipeline to run")
	nextflowRunCommand.Flags().StringVarP(&nextflowConfig, "config", "c", "", "name of config to use (defult: \"<org>-<pipeline>-<revision>\")")
	nextflowRunCommand.Flags().StringVar(&nextflowConfigRepo, "config-repo", "gmc-norr/config-files", "name of config repo to use")
	nextflowRunCommand.Flags().StringVar(&nextflowConfigRev, "config-revision", "main", "tag/branch/commit of config repo to use")
	nextflowRunCommand.Flags().StringVarP(&nextflowWorkdir, "workdir", "d", ".", "directory where the pipeline should be executed")
	nextflowRunCommand.Flags().StringVarP(&nextflowProfile, "profile", "p", "", "comma-separated list of profiles to use for the execution")

	nextflowCommand.AddCommand(nextflowRunCommand)

	rootCmd.AddCommand(configCommand)
	rootCmd.AddCommand(nextflowCommand)
}

func main() {
	config()
	if err := logger(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if err := initConfigDir(); err != nil {
		slog.Error("error initialising config directory", "error", err.Error())
		os.Exit(1)
	}
	initCobra()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
