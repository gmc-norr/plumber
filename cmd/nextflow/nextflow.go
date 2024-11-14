package nextflow

import (
	"github.com/spf13/cobra"
)

var NextflowCmd = &cobra.Command{
	Use:   "nextflow",
	Short: "Run and manage Nextflow pipelines",
}

func init() {
	NextflowCmd.AddCommand(runCmd)
}
