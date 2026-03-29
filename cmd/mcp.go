package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/avicuna/ai-council-personal/internal/cost"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/strategy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

const (
	mcpVersion   = "1.0.0"
	mcpChangelog = "Go rewrite v1.0.0 - Complete rewrite from Python to Go"
)

// Package-level variable to store last query usage
var (
	lastUsageMu sync.RWMutex
	lastUsage   *usageStats
)

type usageStats struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Models       []string
	CostUSD      float64
}

// Type definitions for MCP tool arguments
type askArgs struct {
	Prompt  string `json:"prompt" jsonschema:"The query to ask the AI Council"`
	Mode    string `json:"mode,omitempty" jsonschema:"Strategy mode: moa, debate, or redteam (default: moa)"`
	Verbose bool   `json:"verbose,omitempty" jsonschema:"Show per-model responses (default: true)"`
	Rounds  int    `json:"rounds,omitempty" jsonschema:"Number of debate rounds (default: 3)"`
	Tier    string `json:"tier,omitempty" jsonschema:"Model tier: fast, balanced, or full (default: full)"`
}

type reviewArgs struct {
	FilePath    string `json:"file_path,omitempty" jsonschema:"Path to the file to review"`
	FileContent string `json:"file_content,omitempty" jsonschema:"Content of the file to review"`
	Tier        string `json:"tier,omitempty" jsonschema:"Model tier: fast, balanced, or full (default: full)"`
}

type debugArgs struct {
	Error string `json:"error" jsonschema:"The error message or stack trace to debug"`
	Tier  string `json:"tier,omitempty" jsonschema:"Model tier: fast, balanced, or full (default: full)"`
}

type researchArgs struct {
	Topic  string `json:"topic" jsonschema:"The topic to research"`
	Rounds int    `json:"rounds,omitempty" jsonschema:"Number of debate rounds (default: 2)"`
	Tier   string `json:"tier,omitempty" jsonschema:"Model tier: fast, balanced, or full (default: full)"`
}

type modelsArgs struct {
	Tier string `json:"tier,omitempty" jsonschema:"Model tier: fast, balanced, or full (default: full)"`
}

type changelogArgs struct {
	LastN int `json:"last_n,omitempty" jsonschema:"Number of recent changes to show (default: 5)"`
}

type councilArgs struct {
	Action string                 `json:"action" jsonschema:"Action to perform: ask, review, debug, research, costs, models, usage, status, or changelog"`
	Params map[string]interface{} `json:"params,omitempty" jsonschema:"Parameters for the action"`
}

var mcpCmd = &cobra.Command{
	Use:    "mcp",
	Short:  "Run the AI Council MCP server on stdio",
	Hidden: true, // Hidden from main help since it's for MCP clients
	RunE: func(cmd *cobra.Command, args []string) error {
		// Redirect all logging to stderr (MCP protocol uses stdout)
		log.SetOutput(os.Stderr)

		// Create MCP server
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "ai-council",
			Version: mcpVersion,
		}, nil)

		// Register all council tools
		registerCouncilTools(server)

		// Run server on stdio transport
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			return fmt.Errorf("MCP server failed: %w", err)
		}

		return nil
	},
}

func init() {
	// Register the mcp command
	// Note: This is not added to RegisterCommands since it's a special hidden command
}

// registerCouncilTools registers all AI Council tools with the MCP server
func registerCouncilTools(server *mcp.Server) {
	// council_ask
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_ask",
		Description: "Query the AI Council with a prompt. The council runs 6 flagship models in parallel and synthesizes their best answer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args askArgs) (*mcp.CallToolResult, any, error) {
		return handleAsk(ctx, args)
	})

	// council_review
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_review",
		Description: "Get a comprehensive code review from the AI Council. Provide either file_path or file_content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args reviewArgs) (*mcp.CallToolResult, any, error) {
		return handleReview(ctx, args)
	})

	// council_debug
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_debug",
		Description: "Debug an error with help from the AI Council. The council will analyze the error and suggest fixes.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args debugArgs) (*mcp.CallToolResult, any, error) {
		return handleDebug(ctx, args)
	})

	// council_research
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_research",
		Description: "Deep research with the AI Council using debate mode. Models engage in multi-round debate to explore the topic thoroughly.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args researchArgs) (*mcp.CallToolResult, any, error) {
		return handleResearch(ctx, args)
	})

	// council_costs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_costs",
		Description: "Show spending summary and cost breakdowns for AI Council queries.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return handleCosts(ctx)
	})

	// council_models
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_models",
		Description: "List available models for a given tier.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args modelsArgs) (*mcp.CallToolResult, any, error) {
		return handleModels(ctx, args)
	})

	// council_usage
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_usage",
		Description: "Show token usage and cost from the last query.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return handleUsage(ctx)
	})

	// council_status
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_status",
		Description: "Show AI Council server version and status information.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return handleStatus(ctx)
	})

	// council_changelog
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council_changelog",
		Description: "Show recent changes and version history.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args changelogArgs) (*mcp.CallToolResult, any, error) {
		return handleChangelog(ctx, args)
	})

	// council (generic escape-hatch)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "council",
		Description: "Generic AI Council tool. Dispatches to specific council actions based on the action parameter.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args councilArgs) (*mcp.CallToolResult, any, error) {
		return handleGeneric(ctx, args)
	})
}

// handleAsk handles the council_ask tool
func handleAsk(ctx context.Context, args askArgs) (*mcp.CallToolResult, any, error) {
	// Set defaults
	if args.Mode == "" {
		args.Mode = "moa"
	}
	if args.Tier == "" {
		args.Tier = "full"
	}
	if args.Rounds == 0 {
		args.Rounds = 3
	}

	// Run pipeline
	opts := &pipelineOptions{
		tier:         args.Tier,
		mode:         args.Mode,
		prompt:       args.Prompt,
		systemPrompt: "",
		maxTokens:    4000,
		rounds:       args.Rounds,
		verbose:      args.Verbose,
	}

	result, err := runMCPPipeline(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("council query failed: %w", err)
	}

	// Format and return result
	output := formatMCPResult(result, opts)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}

// handleReview handles the council_review tool
func handleReview(ctx context.Context, args reviewArgs) (*mcp.CallToolResult, any, error) {
	// Read code from file or content
	var code string
	if args.FilePath != "" {
		data, err := os.ReadFile(args.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read file: %w", err)
		}
		code = string(data)
	} else if args.FileContent != "" {
		code = args.FileContent
	} else {
		return nil, nil, fmt.Errorf("either file_path or file_content must be provided")
	}

	if args.Tier == "" {
		args.Tier = "full"
	}

	// Construct prompt
	reviewPromptText := "Review this code for bugs, security issues, performance problems, edge cases, and suggest improvements. Be thorough and specific."
	prompt := fmt.Sprintf("%s\n\n```\n%s\n```", reviewPromptText, code)

	// Run pipeline
	opts := &pipelineOptions{
		tier:         args.Tier,
		mode:         "moa",
		prompt:       prompt,
		systemPrompt: "",
		maxTokens:    4000,
		rounds:       1,
		verbose:      true,
	}

	result, err := runMCPPipeline(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("code review failed: %w", err)
	}

	// Format and return result
	output := formatMCPResult(result, opts)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}

// handleDebug handles the council_debug tool
func handleDebug(ctx context.Context, args debugArgs) (*mcp.CallToolResult, any, error) {
	if args.Tier == "" {
		args.Tier = "full"
	}

	// Construct prompt
	debugPromptText := "Debug this error and suggest fixes. Explain what's likely causing it and how to resolve it."
	prompt := fmt.Sprintf("%s\n\n```\n%s\n```", debugPromptText, args.Error)

	// Run pipeline
	opts := &pipelineOptions{
		tier:         args.Tier,
		mode:         "moa",
		prompt:       prompt,
		systemPrompt: "",
		maxTokens:    4000,
		rounds:       1,
		verbose:      true,
	}

	result, err := runMCPPipeline(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("debug failed: %w", err)
	}

	// Format and return result
	output := formatMCPResult(result, opts)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}

// handleResearch handles the council_research tool
func handleResearch(ctx context.Context, args researchArgs) (*mcp.CallToolResult, any, error) {
	if args.Tier == "" {
		args.Tier = "full"
	}
	if args.Rounds == 0 {
		args.Rounds = 2
	}

	// Run pipeline with debate mode
	opts := &pipelineOptions{
		tier:         args.Tier,
		mode:         "debate",
		prompt:       args.Topic,
		systemPrompt: "",
		maxTokens:    4000,
		rounds:       args.Rounds,
		verbose:      true,
	}

	result, err := runMCPPipeline(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("research failed: %w", err)
	}

	// Format and return result
	output := formatMCPResult(result, opts)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}

// handleCosts handles the council_costs tool
func handleCosts(ctx context.Context) (*mcp.CallToolResult, any, error) {
	tracker, err := cost.NewTracker("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cost tracker: %w", err)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cost summary: %w", err)
	}

	// Get breakdowns
	byTier, err := tracker.GetByTier()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tier breakdown: %w", err)
	}

	byMode, err := tracker.GetByMode()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mode breakdown: %w", err)
	}

	// Format output
	var output strings.Builder
	output.WriteString("AI Council Cost Summary\n")
	output.WriteString("=======================\n\n")
	output.WriteString(fmt.Sprintf("Today: $%.4f (%d queries)\n", summary.Today, summary.QueriesToday))
	output.WriteString(fmt.Sprintf("Week: $%.4f\n", summary.Week))
	output.WriteString(fmt.Sprintf("Month: $%.4f\n", summary.Month))
	output.WriteString(fmt.Sprintf("All Time: $%.4f (%d queries)\n\n", summary.AllTime, summary.QueryCount))

	if len(byMode) > 0 {
		output.WriteString("By Mode:\n")
		for _, breakdown := range byMode {
			output.WriteString(fmt.Sprintf("  %s: $%.4f (%d queries)\n", breakdown.Mode, breakdown.CostUSD, breakdown.Queries))
		}
		output.WriteString("\n")
	}

	if len(byTier) > 0 {
		output.WriteString("By Tier:\n")
		for _, breakdown := range byTier {
			output.WriteString(fmt.Sprintf("  %s: $%.4f (%d queries)\n", breakdown.Tier, breakdown.CostUSD, breakdown.Queries))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

// handleModels handles the council_models tool
func handleModels(ctx context.Context, args modelsArgs) (*mcp.CallToolResult, any, error) {
	if args.Tier == "" {
		args.Tier = "full"
	}

	proposers := config.GetProposers(args.Tier)
	aggregator := config.GetAggregator(args.Tier)

	var output strings.Builder
	output.WriteString(fmt.Sprintf("AI Council Models - %s tier\n", args.Tier))
	output.WriteString("=============================\n\n")

	output.WriteString("Proposers:\n")
	for _, p := range proposers {
		output.WriteString(fmt.Sprintf("  - %s (%s)\n", p.Name, p.Model))
	}

	output.WriteString(fmt.Sprintf("\nAggregator:\n  - %s (%s)\n", aggregator.Name, aggregator.Model))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

// handleUsage handles the council_usage tool
func handleUsage(ctx context.Context) (*mcp.CallToolResult, any, error) {
	lastUsageMu.RLock()
	defer lastUsageMu.RUnlock()

	if lastUsage == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No recent queries. Run a query first to see usage statistics."},
			},
		}, nil, nil
	}

	var output strings.Builder
	output.WriteString("Last Query Usage\n")
	output.WriteString("================\n\n")
	output.WriteString(fmt.Sprintf("Input Tokens: %d\n", lastUsage.InputTokens))
	output.WriteString(fmt.Sprintf("Output Tokens: %d\n", lastUsage.OutputTokens))
	output.WriteString(fmt.Sprintf("Total Tokens: %d\n", lastUsage.TotalTokens))
	output.WriteString(fmt.Sprintf("Cost: $%.4f\n\n", lastUsage.CostUSD))
	output.WriteString(fmt.Sprintf("Models Used: %s\n", strings.Join(lastUsage.Models, ", ")))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

// handleStatus handles the council_status tool
func handleStatus(ctx context.Context) (*mcp.CallToolResult, any, error) {
	var output strings.Builder
	output.WriteString("AI Council Status\n")
	output.WriteString("=================\n\n")
	output.WriteString(fmt.Sprintf("Version: %s (Go)\n", mcpVersion))
	output.WriteString("\nAvailable Tiers:\n")
	for _, tier := range config.ValidTiers() {
		proposers := config.GetProposers(tier)
		output.WriteString(fmt.Sprintf("  - %s: %d models\n", tier, len(proposers)))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

// handleChangelog handles the council_changelog tool
func handleChangelog(ctx context.Context, args changelogArgs) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: mcpChangelog},
		},
	}, nil, nil
}

// handleGeneric handles the generic council tool
func handleGeneric(ctx context.Context, args councilArgs) (*mcp.CallToolResult, any, error) {
	// Convert params map to appropriate args struct and dispatch
	paramsJSON, err := json.Marshal(args.Params)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid params: %w", err)
	}

	switch args.Action {
	case "ask":
		var askArgs askArgs
		if err := json.Unmarshal(paramsJSON, &askArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid ask params: %w", err)
		}
		return handleAsk(ctx, askArgs)

	case "review":
		var reviewArgs reviewArgs
		if err := json.Unmarshal(paramsJSON, &reviewArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid review params: %w", err)
		}
		return handleReview(ctx, reviewArgs)

	case "debug":
		var debugArgs debugArgs
		if err := json.Unmarshal(paramsJSON, &debugArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid debug params: %w", err)
		}
		return handleDebug(ctx, debugArgs)

	case "research":
		var researchArgs researchArgs
		if err := json.Unmarshal(paramsJSON, &researchArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid research params: %w", err)
		}
		return handleResearch(ctx, researchArgs)

	case "costs":
		return handleCosts(ctx)

	case "models":
		var modelsArgs modelsArgs
		if err := json.Unmarshal(paramsJSON, &modelsArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid models params: %w", err)
		}
		return handleModels(ctx, modelsArgs)

	case "usage":
		return handleUsage(ctx)

	case "status":
		return handleStatus(ctx)

	case "changelog":
		var changelogArgs changelogArgs
		if err := json.Unmarshal(paramsJSON, &changelogArgs); err != nil {
			return nil, nil, fmt.Errorf("invalid changelog params: %w", err)
		}
		return handleChangelog(ctx, changelogArgs)

	default:
		return nil, nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}

// runMCPPipeline is like runPipeline but for MCP context (no progress UI, returns result)
func runMCPPipeline(ctx context.Context, opts *pipelineOptions) (*strategy.Result, error) {
	// Validate API keys for the tier
	if err := config.ValidateKeys(opts.tier); err != nil {
		return nil, err
	}

	// Build providers
	proposers, err := buildProposers(opts.tier)
	if err != nil {
		return nil, err
	}

	aggregator, err := buildAggregator(opts.tier)
	if err != nil {
		return nil, err
	}

	var scorer provider.Provider
	if opts.tier == "full" {
		scorer = aggregator
	}

	// Create strategy
	strat, err := createStrategy(opts.mode)
	if err != nil {
		return nil, err
	}

	// Build request
	req := &provider.Request{
		SystemPrompt: opts.systemPrompt,
		UserPrompt:   opts.prompt,
		Temperature:  nil,
		MaxTokens:    opts.maxTokens,
	}

	// Execute strategy (no progress channel for MCP)
	stratOpts := &strategy.Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer,
		Request:    req,
		Progress:   nil, // No progress UI for MCP
		Rounds:     opts.rounds,
		IsFastTier: opts.tier == "fast",
	}

	result, err := strat.Execute(ctx, stratOpts)
	if err != nil {
		return nil, fmt.Errorf("strategy execution failed: %w", err)
	}

	// Store usage stats
	storeUsageStats(result, opts)

	// Log cost
	if err := logCost(result, opts); err != nil {
		// Don't fail on cost logging error, just log to stderr
		log.Printf("Warning: failed to log cost: %v\n", err)
	}

	return result, nil
}

// storeUsageStats stores the usage stats from the last query
func storeUsageStats(result *strategy.Result, opts *pipelineOptions) {
	lastUsageMu.Lock()
	defer lastUsageMu.Unlock()

	stats := &usageStats{
		Models: make([]string, 0, len(result.Proposals)+1),
	}

	// Calculate token usage and cost
	tracker, err := cost.NewTracker("")
	if err != nil {
		log.Printf("Warning: failed to create cost tracker: %v\n", err)
		return
	}

	for _, resp := range result.Proposals {
		stats.InputTokens += resp.InputTokens
		stats.OutputTokens += resp.OutputTokens
		stats.Models = append(stats.Models, resp.Name)
		stats.CostUSD += tracker.EstimateCost(resp.Model, resp.InputTokens, resp.OutputTokens)
	}

	if result.Synthesis != nil {
		stats.InputTokens += result.Synthesis.InputTokens
		stats.OutputTokens += result.Synthesis.OutputTokens
		stats.Models = append(stats.Models, result.Synthesis.Name)
		stats.CostUSD += tracker.EstimateCost(result.Synthesis.Model, result.Synthesis.InputTokens, result.Synthesis.OutputTokens)
	}

	stats.TotalTokens = stats.InputTokens + stats.OutputTokens
	lastUsage = stats
}

// formatMCPResult formats a strategy result for MCP output (plain text, no terminal formatting)
func formatMCPResult(result *strategy.Result, opts *pipelineOptions) string {
	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("AI Council Result (%s mode, %s tier)\n", opts.mode, opts.tier))
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Per-model responses if verbose
	if opts.verbose && len(result.Proposals) > 0 {
		output.WriteString("Model Responses:\n")
		output.WriteString("-" + strings.Repeat("-", 50) + "\n\n")
		for i, resp := range result.Proposals {
			output.WriteString(fmt.Sprintf("%d. %s\n", i+1, resp.Name))
			output.WriteString(fmt.Sprintf("   Tokens: %d in / %d out | Latency: %dms\n\n",
				resp.InputTokens, resp.OutputTokens, resp.LatencyMs))
			output.WriteString(resp.Content)
			output.WriteString("\n\n")
		}
	}

	// Synthesis
	if result.Synthesis != nil {
		if opts.verbose {
			output.WriteString("Synthesis:\n")
			output.WriteString("-" + strings.Repeat("-", 50) + "\n\n")
		}
		output.WriteString(result.Synthesis.Content)
		output.WriteString("\n\n")
	}

	// Footer with stats
	output.WriteString("-" + strings.Repeat("-", 50) + "\n")
	modelCount := len(result.Proposals)
	if result.Synthesis != nil {
		modelCount++
	}
	output.WriteString(fmt.Sprintf("Models: %d | Time: %dms", modelCount, result.TotalMs))

	// Add agreement score if available
	if result.AgreementScore != nil {
		output.WriteString(fmt.Sprintf(" | Agreement: %d/10", *result.AgreementScore))
	}

	// Add cost estimate
	tracker, err := cost.NewTracker("")
	if err == nil {
		totalCost := 0.0
		for _, resp := range result.Proposals {
			totalCost += tracker.EstimateCost(resp.Model, resp.InputTokens, resp.OutputTokens)
		}
		if result.Synthesis != nil {
			totalCost += tracker.EstimateCost(result.Synthesis.Model, result.Synthesis.InputTokens, result.Synthesis.OutputTokens)
		}
		output.WriteString(fmt.Sprintf(" | Cost: $%.4f", totalCost))
	}

	output.WriteString("\n")

	return output.String()
}
