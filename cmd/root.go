package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gmc-norr/plumber/cmd/config"
	"github.com/gmc-norr/plumber/cmd/nextflow"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func initConfig() {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Error("unable to find user's home directory")
			os.Exit(1)
		}
		configHome = filepath.Join(home, ".config")
	}

	viper.SetDefault("config-home", filepath.Join(configHome, "plumber"))
	viper.SetDefault("loglevel", "WARN")

	viper.MustBindEnv("config-home", "PLUMBER_CONFIG_HOME")
	viper.MustBindEnv("loglevel", "PLUMBER_LOGLEVEL")
	viper.MustBindEnv("github-token", "PLUMBER_GITHUB_TOKEN")

	if err := initConfigDir(); err != nil {
		slog.Error("error initialising config directory", "error", err.Error())
		os.Exit(1)
	}

	if err := logger(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
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
	configRepo string
	configRev  string

	rootCmd = &cobra.Command{
		Use:     "plumber",
		Short:   "Run pipelines",
		Version: "0.1.0", // x-release-please-version
	}
)

func initConfigDir() error {
	configDir := viper.GetString("config-home")
	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(nextflow.NextflowCmd)

	rootCmd.PersistentFlags().StringVar(&configRepo, "config-repo", "gmc-norr/config-files", "URL to config file git repository")
	rootCmd.PersistentFlags().StringVar(&configRev, "config-version", "main", "Commitish representing the version of the config file repository to use")
	rootCmd.PersistentFlags().StringP("log-level", "l", "WARN", "log level")

	if err := viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if err := viper.BindPFlag("config-repo", rootCmd.PersistentFlags().Lookup("config-repo")); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if err := viper.BindPFlag("config-version", rootCmd.PersistentFlags().Lookup("config-version")); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
