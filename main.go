package main

import (
	"fmt"
	"os"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	tier string
)

var rootCmd = &cobra.Command{
	Use:   "ai-council",
	Short: "AI Council — Personal Edition",
	Long: `AI Council queries multiple flagship AI models in parallel and synthesizes their best answer.

Supports three tiers:
  - fast: Budget models (Haiku, GPT-4o-mini, Gemini Flash)
  - balanced: Mid-tier models (Sonnet, GPT-4.1, Gemini 2.5 Pro)
  - full: Premium models (Claude Opus 4, GPT-4.1, o3, Gemini 2.5 Pro, DeepSeek R1, Grok 3)

Requires API keys: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		// For now, just display tier info.
		tierCfg := config.GetTier(tier)
		fmt.Printf("AI Council — Tier: %s\n\n", tierCfg.Name)

		fmt.Println("Proposers:")
		for _, model := range tierCfg.Proposers {
			available := "✓"
			if !config.Available(model.Model) {
				available = "✗"
			}
			reasoning := ""
			if model.IsReasoning {
				reasoning = " (reasoning)"
			}
			fmt.Printf("  %s %s%s\n", available, model.Name, reasoning)
		}

		fmt.Printf("\nAggregator: %s\n", tierCfg.Aggregator.Name)

		// Validate keys.
		if err := config.ValidateKeys(tier); err != nil {
			fmt.Fprintf(os.Stderr, "\n%v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nAll required API keys are set. Ready to run!")
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&tier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to use (%s)", config.ValidTiers()))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
