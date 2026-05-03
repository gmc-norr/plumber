package main

import (
	"os"

	"github.com/spf13/viper"
)

var version = "1.0.0" // x-release-please-version

func main() {
	v := viper.New()
	rootCmd := NewRootCmd(v)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
