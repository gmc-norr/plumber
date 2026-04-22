package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gmc-norr/plumber/cmd/config"
	"github.com/gmc-norr/plumber/cmd/hydra"
	"github.com/gmc-norr/plumber/cmd/nextflow"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func initConfig(v *viper.Viper) error {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to find user's home directory")
		}
		configHome = filepath.Join(home, ".config")
	}

	cacheHome, ok := os.LookupEnv("XDG_CACHE_HOME")
	if !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to find user's home directory")
		}
		cacheHome = filepath.Join(home, ".cache")
	}

	v.SetDefault("config-home", filepath.Join(configHome, "plumber"))
	v.SetDefault("cache-home", filepath.Join(cacheHome, "plumber"))
	v.SetDefault("log-level", "WARN")
	v.Set("plumber-version", version)

	v.MustBindEnv("config-home", "PLUMBER_CONFIG_HOME")
	v.MustBindEnv("cache-home", "PLUMBER_CACHE_HOME")
	v.MustBindEnv("log-level", "PLUMBER_LOGLEVEL")

	v.MustBindEnv("certs", "PLUMBER_CERTS")
	v.MustBindEnv("webhook-url", "PLUMBER_WEBHOOK_URL")
	v.MustBindEnv("webhook-api-key", "PLUMBER_WEBHOOK_API_KEY")
	v.MustBindEnv("webhook-no-verify", "PLUMBER_WEBHOOK_NO_VERIFY")

	return logger(v)
}

func logger(v *viper.Viper) error {
	logOpts := slog.HandlerOptions{}

	logLevel := v.GetString("log-level")
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

func NewRootCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plumber",
		Short:   "Run pipelines",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initErr := initConfig(v)
			configErr := os.MkdirAll(v.GetString("config-home"), 0o777)
			cacheErr := os.MkdirAll(v.GetString("cache-home"), 0o777)
			err := errors.Join(initErr, configErr, cacheErr)
			if err != nil {
				return fmt.Errorf("failed to initialise plumber: %w", err)
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.AddCommand(config.NewConfigCmd(v))
	cmd.AddCommand(nextflow.NewNextflowCmd(v))
	cmd.AddCommand(hydra.NewHydraCmd(v))
	cmd.AddCommand(NewInitCmd(v))
	cmd.AddCommand(NewRunCmd(v))

	cmd.PersistentFlags().String("config-repo", "https://github.com/gmc-norr/config-files", "URL or path to the config file git repository")
	cmd.PersistentFlags().String("config-version", "main", "Commitish representing the version of the config file repository to use")
	cmd.PersistentFlags().String("certs", "", "Path to CA certificates to use for webhook TLS")
	cmd.PersistentFlags().String("webhook-url", "", "Webhook URL where to send status updates")
	cmd.PersistentFlags().String("webhook-api-key", "", "API key for the webhook")
	cmd.PersistentFlags().Bool("webhook-no-verify", false, "Don't verify TLS certificates for webhooks (INSECURE)")
	cmd.PersistentFlags().StringP("log-level", "l", "WARN", "log level")

	_ = v.BindPFlags(cmd.PersistentFlags())

	return cmd
}
