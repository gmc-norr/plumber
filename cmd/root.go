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
	viper.SetDefault("log-level", "WARN")

	viper.MustBindEnv("config-home", "PLUMBER_CONFIG_HOME")
	viper.MustBindEnv("log-level", "PLUMBER_LOGLEVEL")

	if err := logger(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func logger() error {
	logOpts := slog.HandlerOptions{}

	logLevel := viper.GetString("log-level")
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

var rootCmd = &cobra.Command{
	Use:     "plumber",
	Short:   "Run pipelines",
	Version: version,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(nextflow.NextflowCmd)
	rootCmd.AddCommand(initCmd)

	rootCmd.PersistentFlags().String("config-repo", "https://github.com/gmc-norr/config-files", "URL or path to the config file git repository")
	rootCmd.PersistentFlags().String("config-version", "main", "Commitish representing the version of the config file repository to use")
	rootCmd.PersistentFlags().StringP("log-level", "l", "WARN", "log level")

	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
}
