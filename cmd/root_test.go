package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newTestRootCmd(t *testing.T) *cobra.Command {
	configDir := t.TempDir()
	v := viper.New()
	v.SetDefault("config-home", filepath.Join(configDir, "config"))
	v.SetDefault("cache-home", filepath.Join(configDir, "cache"))
	v.SetDefault("log-level", "debug")
	return NewRootCmd(v)
}
