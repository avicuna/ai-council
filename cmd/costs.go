package cmd

import (
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/cost"
	"github.com/avicuna/ai-council-personal/internal/output"
	"github.com/spf13/cobra"
)

var costsCmd = &cobra.Command{
	Use:   "costs",
	Short: "Display cost summary and breakdowns",
	Long: `Display cost tracking summary including:
  - Spending over time (today, week, month, all-time)
  - Total query count
  - Breakdown by tier
  - Breakdown by mode`,
	Example: `  ai-council costs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create tracker
		tracker, err := cost.NewTracker("")
		if err != nil {
			return fmt.Errorf("failed to create tracker: %w", err)
		}

		// Get summary
		summary, err := tracker.GetSummary()
		if err != nil {
			return fmt.Errorf("failed to get summary: %w", err)
		}

		// Get breakdowns
		byTier, err := tracker.GetByTier()
		if err != nil {
			return fmt.Errorf("failed to get tier breakdown: %w", err)
		}

		byMode, err := tracker.GetByMode()
		if err != nil {
			return fmt.Errorf("failed to get mode breakdown: %w", err)
		}

		// Render
		rendered := output.RenderCosts(summary, byTier, byMode)
		fmt.Println(rendered)

		return nil
	},
}

