package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "filler-cli",
	Short: "Analyze filler words in Japanese speech using AmiVoice API",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
}
