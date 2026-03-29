package cmd

import (
	"context"
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/spf13/cobra"
)

var (
	researchVerbose bool
	researchTier    string
	researchRounds  int
)

const researchPrompt = "Research this topic thoroughly. Cover: current state of the art, key trade-offs, practical recommendations, and common pitfalls. Be comprehensive but concise."

var researchCmd = &cobra.Command{
	Use:   "research [topic]",
	Short: "Deep research with the AI Council debate mode",
	Long: `Conduct deep research on a topic using the AI Council's debate mode.

Models will debate over multiple rounds to produce thorough analysis covering:
  - Current state of the art
  - Key trade-offs and considerations
  - Practical recommendations
  - Common pitfalls to avoid

The topic can be provided as:
  - Command line arguments: ai-council research "topic"
  - Standard input: echo "topic" | ai-council research`,
	Example: `  ai-council research "Rust vs Go for backend services"
  ai-council research --rounds 3 "distributed consensus algorithms"
  ai-council research --verbose "event sourcing vs CQRS"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read topic
		topic, err := readPrompt(args, "")
		if err != nil {
			return err
		}

		if topic == "" {
			return fmt.Errorf("no research topic provided")
		}

		// Construct prompt
		prompt := fmt.Sprintf("%s\n\nTopic: %s", researchPrompt, topic)

		// Run pipeline with debate mode
		opts := &pipelineOptions{
			tier:         researchTier,
			mode:         "debate",
			prompt:       prompt,
			systemPrompt: "",
			maxTokens:    4000,
			rounds:       researchRounds,
			verbose:      researchVerbose,
		}

		return runPipeline(context.Background(), opts)
	},
}

func init() {
	researchCmd.Flags().BoolVarP(&researchVerbose, "verbose", "v", false,
		"Show per-round responses")
	researchCmd.Flags().StringVarP(&researchTier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to use (%v)", config.ValidTiers()))
	researchCmd.Flags().IntVarP(&researchRounds, "rounds", "r", 2,
		"Number of debate rounds")
}
