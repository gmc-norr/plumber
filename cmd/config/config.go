package config

import (
	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage pipeline configuration files",
}

func init() {
	ConfigCmd.AddCommand(listCmd)
	ConfigCmd.AddCommand(downloadCmd)
	ConfigCmd.AddCommand(rmCmd)
	ConfigCmd.AddCommand(validateCmd)
	ConfigCmd.AddCommand(editCmd)
}
