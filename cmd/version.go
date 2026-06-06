package cmd

import (
	"fmt"

	"github.com/maro114510/filler-cli/internal/cmdinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of filler-cli",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n", cmdinfo.Name, cmdinfo.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
