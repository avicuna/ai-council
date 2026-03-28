package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/avicuna/ai-council-personal/internal/prompt"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/scoring"
)

// MoA implements the Mixture of Agents strategy.
// It queries all proposers in parallel, synthesizes their responses, and optionally scores agreement.
type MoA struct{}

// NewMoA creates a new MoA strategy instance.
func NewMoA() *MoA {
	return &MoA{}
}

// Execute runs the MoA strategy:
// 1. Query all proposers in parallel
// 2. Filter to successful responses (need ≥2, or ≥1 for fast tier)
// 3. For fast tier: return first successful response as synthesis (skip aggregation)
// 4. For normal tiers: call aggregator with synthesis prompt
// 5. If scorer provided: score agreement
// 6. Return result with all proposals, synthesis, agreement score, and total latency
func (m *MoA) Execute(ctx context.Context, opts *Options) (*Result, error) {
	if len(opts.Proposers) == 0 {
		return nil, fmt.Errorf("no proposers configured")
	}

	start := time.Now()

	// Step 1: Query all proposers in parallel
	allResults := provider.QueryAll(ctx, opts.Proposers, opts.Request, opts.Progress)

	// Step 2: Filter to successful responses
	succeeded := allResults.Responses
	if len(succeeded) == 0 {
		return nil, fmt.Errorf("all proposers failed: %v", allResults.Errors)
	}

	// Check minimum required successes
	minRequired := 2
	if opts.IsFastTier {
		minRequired = 1
	}

	if len(succeeded) < minRequired {
		return nil, fmt.Errorf("insufficient successful proposals: got %d, need at least %d", len(succeeded), minRequired)
	}

	// Convert to pointer slice for Result
	proposals := make([]*provider.Response, len(succeeded))
	for i := range succeeded {
		proposals[i] = &succeeded[i]
	}

	var synthesis *provider.Response
	var agreementScore *int
	var agreementReason string

	// Step 3: Fast tier optimization - skip aggregation
	if opts.IsFastTier {
		// Return first successful response as synthesis
		synthesis = &succeeded[0]
	} else {
		// Step 4: Call aggregator for synthesis
		aggregatorReq := buildAggregatorRequest(opts.Request.UserPrompt, succeeded)
		resp, err := opts.Aggregator.Query(ctx, aggregatorReq)
		if err != nil {
			return nil, fmt.Errorf("aggregator failed: %w", err)
		}
		synthesis = resp
	}

	// Step 5: Score agreement if scorer provided
	if opts.Scorer != nil {
		score, err := scoring.ScoreAgreement(ctx, opts.Scorer, succeeded, opts.Request.UserPrompt)
		if err != nil {
			// Non-fatal: log but don't fail the entire strategy
			// Return partial result with scoring error in reason
			agreementReason = fmt.Sprintf("scoring failed: %v", err)
		} else if score != nil {
			agreementScore = &score.Score
			agreementReason = score.Reason
		}
	}

	// Step 6: Return result
	totalMs := time.Since(start).Milliseconds()
	return &Result{
		Proposals:       proposals,
		Synthesis:       synthesis,
		Rounds:          1, // MoA is single-round
		AgreementScore:  agreementScore,
		AgreementReason: agreementReason,
		TotalMs:         totalMs,
	}, nil
}

// buildAggregatorRequest constructs the aggregator request with synthesis prompt.
func buildAggregatorRequest(userPrompt string, responses []provider.Response) *provider.Request {
	proposalsText := prompt.FormatProposals(responses)

	// Build user prompt from template
	aggregatorPrompt := strings.ReplaceAll(prompt.MOAAggregatorTemplate, "{prompt}", userPrompt)
	aggregatorPrompt = strings.ReplaceAll(aggregatorPrompt, "{proposals}", proposalsText)

	return &provider.Request{
		SystemPrompt: prompt.MOAAggregatorSystem,
		UserPrompt:   aggregatorPrompt,
		Temperature:  nil, // Let provider use its default
		MaxTokens:    4000,
	}
}
