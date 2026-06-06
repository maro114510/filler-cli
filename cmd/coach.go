package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var coachCmd = &cobra.Command{
	Use:   "coach <audio-file>",
	Short: "LLM-enhanced coaching from filler analysis (requires AmiVoice + LLM API key)",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runCoach,
}

func runCoach(_ *cobra.Command, _ []string) error {
	return errors.New("coach is not yet implemented (see Issue #7)")
}

func init() {
	rootCmd.AddCommand(coachCmd)
}
