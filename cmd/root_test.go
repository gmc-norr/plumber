package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newTestViper(t *testing.T) *viper.Viper {
	configDir := t.TempDir()
	v := viper.New()
	v.Set("config-home", filepath.Join(configDir, "config"))
	v.Set("cache-home", filepath.Join(configDir, "cache"))
	v.Set("log-level", "debug")
	return v
}

func newTestRootCmd(t *testing.T) *cobra.Command {
	v := newTestViper(t)
	return NewRootCmd(v)
}

// findPlumberFile assumes that the supplied viper instance points to a local
// config that contains only a single pipeline config, .i.e. that [newTestViper]
// has been called once and only once for a single pipeline test.
func findPlumberFile(v *viper.Viper) (string, error) {
	if _, err := os.Stat(filepath.Join(v.GetString("config-home"))); err != nil {
		return "", err
	}

	var plumberFilePath string
	err := filepath.WalkDir(v.GetString("config-home"), func(path string, d fs.DirEntry, err error) error {
		if filepath.Base(path) == "plumber.yaml" {
			if plumberFilePath != "" {
				return fmt.Errorf("multiple plumberfiles found")
			}
			plumberFilePath = path
		}
		return nil
	})
	if plumberFilePath == "" && err != nil {
		err = fmt.Errorf("could not find plumberfile")
	}
	return plumberFilePath, err
}
