package hydra

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewHydraCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hydra",
		Short: "Run and manage Hydra Genetics pipelines",
	}
	cmd.AddCommand(NewRunCmd(v))
	return cmd
}
