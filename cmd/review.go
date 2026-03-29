package cmd

import (
	"context"
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/spf13/cobra"
)

var (
	reviewVerbose bool
	reviewTier    string
)

const reviewPrompt = "Review this code for bugs, security issues, performance problems, edge cases, and suggest improvements. Be thorough and specific."

var reviewCmd = &cobra.Command{
	Use:   "review [file]",
	Short: "Get a code review from the AI Council",
	Long: `Get a comprehensive code review from the AI Council.

The council will review the code for:
  - Bugs and correctness issues
  - Security vulnerabilities
  - Performance problems
  - Edge cases and error handling
  - Code quality and maintainability

The code can be provided as:
  - File path: ai-council review myfile.go
  - Standard input: cat myfile.go | ai-council review`,
	Example: `  ai-council review main.go
  ai-council review --verbose src/handler.py
  cat complex.ts | ai-council review --tier balanced`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read file or stdin
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		}

		code, err := readPrompt([]string{}, filePath)
		if err != nil {
			return err
		}

		if code == "" {
			return fmt.Errorf("no code provided")
		}

		// Construct prompt
		prompt := fmt.Sprintf("%s\n\n```\n%s\n```", reviewPrompt, code)

		// Run pipeline with MoA verbose mode
		opts := &pipelineOptions{
			tier:         reviewTier,
			mode:         "moa",
			prompt:       prompt,
			systemPrompt: "",
			maxTokens:    4000,
			rounds:       1,
			verbose:      reviewVerbose,
		}

		return runPipeline(context.Background(), opts)
	},
}

func init() {
	reviewCmd.Flags().BoolVarP(&reviewVerbose, "verbose", "v", true,
		"Show per-model responses (default: true for review)")
	reviewCmd.Flags().StringVarP(&reviewTier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to use (%v)", config.ValidTiers()))
}
