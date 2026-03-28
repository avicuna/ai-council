package cmd

import (
	"context"
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/spf13/cobra"
)

var (
	askMode     string
	askVerbose  bool
	askRounds   int
	askFile     string
	askTier     string
	askMaxTokens int
)

var askCmd = &cobra.Command{
	Use:   "ask [prompt]",
	Short: "Query the AI Council with a prompt",
	Long: `Query the AI Council with a prompt. The council will run multiple AI models
in parallel and synthesize their responses.

The prompt can be provided as:
  - Command line arguments: ai-council ask "your question"
  - A file: ai-council ask --file prompt.txt
  - Standard input: echo "your question" | ai-council ask

Modes:
  - moa (default): Mixture of Agents — parallel proposals, then synthesis
  - debate: Multi-round debate with iterative refinement
  - redteam: Adversarial red team with attack and defense`,
	Example: `  ai-council ask "What is the best approach to distributed tracing?"
  ai-council ask --mode debate --rounds 3 "Should we use microservices?"
  ai-council ask --tier fast --verbose "Quick summary of REST vs GraphQL"
  cat research.txt | ai-council ask --mode debate`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read prompt
		prompt, err := readPrompt(args, askFile)
		if err != nil {
			return err
		}

		if prompt == "" {
			return fmt.Errorf("prompt cannot be empty")
		}

		// Validate tier
		validTiers := config.ValidTiers()
		valid := false
		for _, t := range validTiers {
			if t == askTier {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid tier %q (valid: %v)", askTier, validTiers)
		}

		// Validate mode
		validModes := []string{"moa", "debate", "redteam"}
		validMode := false
		for _, m := range validModes {
			if m == askMode {
				validMode = true
				break
			}
		}
		if !validMode {
			return fmt.Errorf("invalid mode %q (valid: %v)", askMode, validModes)
		}

		// Run pipeline
		opts := &pipelineOptions{
			tier:         askTier,
			mode:         askMode,
			prompt:       prompt,
			systemPrompt: "",
			maxTokens:    askMaxTokens,
			rounds:       askRounds,
			verbose:      askVerbose,
		}

		return runPipeline(context.Background(), opts)
	},
}

func init() {
	askCmd.Flags().StringVarP(&askMode, "mode", "m", "moa",
		"Strategy mode: moa, debate, redteam")
	askCmd.Flags().BoolVarP(&askVerbose, "verbose", "v", false,
		"Show per-model responses")
	askCmd.Flags().IntVarP(&askRounds, "rounds", "r", 2,
		"Number of rounds for debate mode")
	askCmd.Flags().StringVarP(&askFile, "file", "f", "",
		"Read prompt from file")
	askCmd.Flags().StringVarP(&askTier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to use (%v)", config.ValidTiers()))
	askCmd.Flags().IntVar(&askMaxTokens, "max-tokens", 4000,
		"Maximum tokens per response")
}
