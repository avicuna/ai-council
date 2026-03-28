package cmd

import (
	"github.com/spf13/cobra"
)

// RegisterCommands registers all subcommands with the root command.
func RegisterCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(researchCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(costsCmd)
}
