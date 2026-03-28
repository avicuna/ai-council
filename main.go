package main

import (
	"fmt"
	"os"

	"github.com/avicuna/ai-council-personal/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ai-council",
	Short: "AI Council — Personal Edition",
	Long: `AI Council queries multiple flagship AI models in parallel and synthesizes their best answer.

Supports three tiers:
  - fast: Budget models (Haiku, GPT-4o-mini, Gemini Flash)
  - balanced: Mid-tier models (Sonnet, GPT-4.1, Gemini 2.5 Pro)
  - full: Premium models (Claude Opus 4, GPT-4.1, o3, Gemini 2.5 Pro, DeepSeek R1, Grok 3)

Requires API keys: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, etc.

Commands:
  ask       Query the AI Council with a prompt
  review    Get a code review from the AI Council
  debug     Debug an error with help from the AI Council
  research  Deep research with the AI Council debate mode
  models    Display available models for a tier
  costs     Display cost summary and breakdowns

Use "ai-council [command] --help" for more information about a command.`,
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
