package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewHydraCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "hydra",
		Short:      "Run and manage Hydra Genetics pipelines",
		Deprecated: `use "plumber run" instead, this command will be removed in the future`,
	}
	cmd.AddCommand(NewHydraRunCmd(v))
	return cmd
}

func NewHydraRunCmd(v *viper.Viper) *cobra.Command {
	cmd := NewRunCmd(v)
	cmd.Deprecated = `use "plumber run" instead (which this is now an alias of), this command will be removed in the future`
	return cmd
}
