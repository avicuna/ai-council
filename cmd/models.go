package cmd

import (
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/avicuna/ai-council-personal/internal/output"
	"github.com/spf13/cobra"
)

var modelsTier string

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Display available models for a tier",
	Long: `Display the models configured for a tier and their availability status.

Shows proposer models and the aggregator model, along with whether their
required API keys are set.`,
	Example: `  ai-council models
  ai-council models --tier fast
  ai-council models --tier balanced`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate tier
		validTiers := config.ValidTiers()
		valid := false
		for _, t := range validTiers {
			if t == modelsTier {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid tier %q (valid: %v)", modelsTier, validTiers)
		}

		// Get models
		proposers := config.GetAllProposers(modelsTier)
		aggregator := config.GetAggregator(modelsTier)

		// Render
		rendered := output.RenderModels(modelsTier, proposers, aggregator)
		fmt.Println(rendered)

		return nil
	},
}

func init() {
	modelsCmd.Flags().StringVarP(&modelsTier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to display (%v)", config.ValidTiers()))
}
