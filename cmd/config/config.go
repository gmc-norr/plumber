package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewConfigCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage pipeline configuration files",
	}

	cmd.AddCommand(NewListCmd(v))
	cmd.AddCommand(NewDownloadCmd(v))
	cmd.AddCommand(NewRmCmd(v))
	cmd.AddCommand(NewValidateCmd(v))
	cmd.AddCommand(NewEditCmd(v))

	return cmd
}
