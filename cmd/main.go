package main

import (
	"os"
)

var version = "0.0.0" // x-release-please-version

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
