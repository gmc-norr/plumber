package config

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gmc-norr/plumber"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewEditCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit HASH",
		Short: "Edit a plumberfile",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			hash := args[0]
			configDir := v.GetString("config-home")
			path := filepath.Join(configDir, hash, plumber.PlumberFileName)
			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "nano"
			}
			command := exec.Command(editor, path)
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			cobra.CheckErr(command.Run())
		},
	}
	return cmd
}
