package main

import (
	"fmt"
	"os"

	"github.com/avicuna/ai-council-personal/cmd"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Version: version,
	Use:   "council-personal",
	Short: "AI Council — Personal Edition",
	Long: `AI Council queries multiple flagship AI models in parallel and synthesizes their best answer.

Supports three tiers:
  - fast: Budget models (Claude Haiku 5.5, GPT-6 Luna)
  - balanced: Mid-tier models (Claude Sonnet 5.5, GPT-6.1 Terra)
  - full: Premium models (Claude Opus 5.5, GPT-6.1 Sol)

Requires API keys: ANTHROPIC_API_KEY, OPENAI_API_KEY (GEMINI_API_KEY with COUNCIL_GEMINI_MODEL)

Commands:
  ask       Query the AI Council with a prompt
  review    Get a code review from the AI Council
  debug     Debug an error with help from the AI Council
  research  Deep research with the AI Council debate mode
  models    Display available models for a tier
  costs     Display cost summary and breakdowns

Use "council-personal [command] --help" for more information about a command.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Register all subcommands
	cmd.RegisterCommands(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
