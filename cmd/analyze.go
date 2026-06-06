package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <audio-file>",
	Short: "Analyze filler words in the given audio file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
