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
	ConfigCmd.AddCommand(pullCmd)
	ConfigCmd.AddCommand(checkoutCmd)
	ConfigCmd.AddCommand(mvCmd)
	ConfigCmd.AddCommand(fetchCmd)
	ConfigCmd.AddCommand(rmCmd)
}
