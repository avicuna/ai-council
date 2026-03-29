package cmd

import (
	"context"
	"fmt"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/spf13/cobra"
)

var (
	debugVerbose bool
	debugTier    string
)

const debugPrompt = "Analyze this error. Explain: 1) What went wrong (root cause), 2) What to check, 3) How to fix it. Be specific and actionable."

var debugCmd = &cobra.Command{
	Use:   "debug [error]",
	Short: "Debug an error with help from the AI Council",
	Long: `Get debugging help from the AI Council for an error message or stack trace.

The council will analyze the error and provide:
  1. Root cause analysis
  2. Diagnostic steps
  3. Concrete fix recommendations

The error can be provided as:
  - Command line arguments: ai-council debug "error message"
  - Standard input: cat error.log | ai-council debug`,
	Example: `  ai-council debug "NullPointerException at line 42"
  ai-council debug --verbose "connection timeout after 30s"
  cat stack_trace.txt | ai-council debug --tier balanced`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read error message
		errorMsg, err := readPrompt(args, "")
		if err != nil {
			return err
		}

		if errorMsg == "" {
			return fmt.Errorf("no error message provided")
		}

		// Construct prompt
		prompt := fmt.Sprintf("%s\n\nError:\n```\n%s\n```", debugPrompt, errorMsg)

		// Run pipeline with MoA
		opts := &pipelineOptions{
			tier:         debugTier,
			mode:         "moa",
			prompt:       prompt,
			systemPrompt: "",
			maxTokens:    4000,
			rounds:       1,
			verbose:      debugVerbose,
		}

		return runPipeline(context.Background(), opts)
	},
}

func init() {
	debugCmd.Flags().BoolVarP(&debugVerbose, "verbose", "v", false,
		"Show per-model responses")
	debugCmd.Flags().StringVarP(&debugTier, "tier", "t", config.DefaultTier(),
		fmt.Sprintf("Tier to use (%v)", config.ValidTiers()))
}
