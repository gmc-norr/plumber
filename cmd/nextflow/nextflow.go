package nextflow

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewNextflowCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nextflow",
		Short: "Run and manage Nextflow pipelines",
	}
	cmd.AddCommand(NewRunCmd(v))
	return cmd
}
