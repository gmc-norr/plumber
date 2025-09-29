package hydra

import (
	"github.com/spf13/cobra"
)

var HydraCmd = &cobra.Command{
	Use:   "hydra",
	Short: "Run and manage Hydra Genetics pipelines",
}

func init() {
	HydraCmd.AddCommand(runCmd)
}
