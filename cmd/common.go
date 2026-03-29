package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/avicuna/ai-council-personal/internal/cost"
	"github.com/avicuna/ai-council-personal/internal/output"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/strategy"
)

// runPipeline executes the standard pipeline: build providers → run strategy → render → log cost.
func runPipeline(ctx context.Context, opts *pipelineOptions) error {
	// Step 1: Validate API keys for the tier
	if err := config.ValidateKeys(opts.tier); err != nil {
		return err
	}

	// Step 2: Build providers
	proposers, err := buildProposers(opts.tier)
	if err != nil {
		return err
	}

	aggregator, err := buildAggregator(opts.tier)
	if err != nil {
		return err
	}

	var scorer provider.Provider
	if opts.tier == "full" {
		// Only full tier has scorer
		scorer = aggregator // Use aggregator as scorer
	}

	// Step 3: Create strategy
	strat, err := createStrategy(opts.mode)
	if err != nil {
		return err
	}

	// Step 4: Create progress channel and tracker
	progressCh := make(chan provider.ProgressEvent, 100)
	tracker := output.NewProgressTracker(progressCh)
	tracker.Start()

	// Step 5: Build request
	req := &provider.Request{
		SystemPrompt: opts.systemPrompt,
		UserPrompt:   opts.prompt,
		Temperature:  nil, // Let provider use defaults
		MaxTokens:    opts.maxTokens,
	}

	// Step 6: Execute strategy
	stratOpts := &strategy.Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer,
		Request:    req,
		Progress:   progressCh,
		Rounds:     opts.rounds,
		IsFastTier: opts.tier == "fast",
	}

	result, err := strat.Execute(ctx, stratOpts)
	close(progressCh)
	tracker.Wait()

	if err != nil {
		return fmt.Errorf("strategy execution failed: %w", err)
	}

	// Step 7: Render output (to stdout for CLI, never called in MCP path)
	renderedOutput := renderResult(result, opts)
	fmt.Fprintln(os.Stdout, renderedOutput)

	// Step 8: Log cost
	if err := logCost(result, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to log cost: %v\n", err)
	}

	return nil
}

// pipelineOptions contains all options for the pipeline.
type pipelineOptions struct {
	tier         string
	mode         string
	prompt       string
	systemPrompt string
	maxTokens    int
	rounds       int
	verbose      bool
}

// buildProposers constructs the proposer providers for the given tier.
func buildProposers(tier string) ([]provider.Provider, error) {
	modelConfigs := config.GetProposers(tier)
	if len(modelConfigs) == 0 {
		return nil, fmt.Errorf("no proposers available for tier %q (check API keys)", tier)
	}

	providers := make([]provider.Provider, 0, len(modelConfigs))
	for _, cfg := range modelConfigs {
		p, err := provider.NewProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider for %s: %w", cfg.Name, err)
		}
		providers = append(providers, p)
	}

	return providers, nil
}

// buildAggregator constructs the aggregator provider for the given tier.
func buildAggregator(tier string) (provider.Provider, error) {
	cfg := config.GetAggregator(tier)
	p, err := provider.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator for %s: %w", cfg.Name, err)
	}
	return p, nil
}

// createStrategy creates the strategy based on the mode.
func createStrategy(mode string) (strategy.Strategy, error) {
	switch mode {
	case "moa":
		return strategy.NewMoA(), nil
	case "debate":
		return strategy.NewDebate(), nil
	case "redteam", "red-team":
		return strategy.NewRedTeam(), nil
	default:
		return nil, fmt.Errorf("unknown mode: %s (valid: moa, debate, redteam)", mode)
	}
}

// renderResult renders the result based on mode and verbosity.
func renderResult(result *strategy.Result, opts *pipelineOptions) string {
	switch opts.mode {
	case "moa":
		return output.RenderMoAResult(result, opts.verbose, opts.tier)
	case "debate":
		return output.RenderDebateResult(result, opts.verbose, opts.tier)
	case "redteam", "red-team":
		return output.RenderRedTeamResult(result, opts.verbose, opts.tier)
	default:
		// Fallback to MoA rendering
		return output.RenderMoAResult(result, opts.verbose, opts.tier)
	}
}

// logCost logs the cost of the query to the tracker.
func logCost(result *strategy.Result, opts *pipelineOptions) error {
	tracker, err := cost.NewTracker("")
	if err != nil {
		return err
	}

	// Calculate total cost
	totalCost := 0.0
	for _, resp := range result.Proposals {
		totalCost += tracker.EstimateCost(resp.Model, resp.InputTokens, resp.OutputTokens)
	}
	if result.Synthesis != nil {
		totalCost += tracker.EstimateCost(result.Synthesis.Model, result.Synthesis.InputTokens, result.Synthesis.OutputTokens)
	}

	// Build model list
	models := make([]string, 0, len(result.Proposals)+1)
	for _, resp := range result.Proposals {
		models = append(models, resp.Name)
	}
	if result.Synthesis != nil {
		models = append(models, result.Synthesis.Name)
	}

	// Truncate prompt for preview (first 100 chars)
	promptPreview := opts.prompt
	if len(promptPreview) > 100 {
		promptPreview = promptPreview[:100] + "..."
	}

	entry := cost.Entry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Mode:          opts.mode,
		Tier:          opts.tier,
		Models:        models,
		SuccessCount:  len(result.Proposals),
		CostUSD:       totalCost,
		LatencyMs:     result.TotalMs,
		PromptPreview: promptPreview,
	}

	return tracker.LogQuery(entry)
}

// readPrompt reads the prompt from args, file, or stdin.
// Returns the prompt string and an error if reading fails.
func readPrompt(args []string, filePath string) (string, error) {
	// Priority: file > args > stdin
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

	// Read from stdin
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	return "", fmt.Errorf("no prompt provided: specify as argument, --file, or pipe to stdin")
}
