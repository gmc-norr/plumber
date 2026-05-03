package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewNextflowCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "nextflow",
		Short:      "Run and manage Nextflow pipelines",
		Deprecated: `use "plumber run" instead, this command will be removed in the future`,
	}
	cmd.AddCommand(NewNextflowRunCmd(v))
	return cmd
}

func NewNextflowRunCmd(v *viper.Viper) *cobra.Command {
	cmd := NewRunCmd(v)
	cmd.Deprecated = `use "plumber run" instead (which this is now an alias of), this command will be removed in the future`
	return cmd
}
