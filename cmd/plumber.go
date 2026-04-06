package main

import (
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

func InitConfigDir(cmd *cobra.Command, args []string) error {
	configDir := viper.GetString("config-home")
	slog.Debug("initialising config dir", "path", configDir)
	return os.MkdirAll(configDir, 0o777)
}

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
	viper.Set("plumber-version", version)

	viper.MustBindEnv("config-home", "PLUMBER_CONFIG_HOME")
	viper.MustBindEnv("log-level", "PLUMBER_LOGLEVEL")

	viper.MustBindEnv("certs", "PLUMBER_CERTS")
	viper.MustBindEnv("webhook-url", "PLUMBER_WEBHOOK_URL")
	viper.MustBindEnv("webhook-api-key", "PLUMBER_WEBHOOK_API_KEY")
	viper.MustBindEnv("webhook-no-verify", "PLUMBER_WEBHOOK_NO_VERIFY")

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
	Use:               "plumber",
	Short:             "Run pipelines",
	Version:           version,
	PersistentPreRunE: InitConfigDir,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(nextflow.NextflowCmd)
	rootCmd.AddCommand(hydra.HydraCmd)
	rootCmd.AddCommand(initCmd)

	rootCmd.PersistentFlags().String("config-repo", "https://github.com/gmc-norr/config-files", "URL or path to the config file git repository")
	rootCmd.PersistentFlags().String("config-version", "main", "Commitish representing the version of the config file repository to use")
	rootCmd.PersistentFlags().String("certs", "", "Path to CA certificates to use for webhook TLS")
	rootCmd.PersistentFlags().String("webhook-url", "", "Webhook URL where to send status updates")
	rootCmd.PersistentFlags().String("webhook-api-key", "", "API key for the webhook")
	rootCmd.PersistentFlags().Bool("webhook-no-verify", false, "Don't verify TLS certificates for webhooks (INSECURE)")
	rootCmd.PersistentFlags().StringP("log-level", "l", "WARN", "log level")

	_ = viper.BindPFlags(rootCmd.PersistentFlags())
}
