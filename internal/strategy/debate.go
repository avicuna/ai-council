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

// Debate implements a multi-round debate strategy.
// Round 1: All proposers respond independently.
// Rounds 2-N: Each proposer revises their answer after seeing others' responses.
// Finally, the aggregator (judge) synthesizes from the full debate history.
type Debate struct{}

// NewDebate creates a new Debate strategy instance.
func NewDebate() *Debate {
	return &Debate{}
}

// Execute runs the debate strategy:
// 1. Round 1: QueryAll proposers with original request
// 2. Rounds 2-N: Each proposer revises seeing others' responses (parallel per round)
// 3. Check ctx.Done() between rounds for early cancellation
// 4. Judge synthesizes from full debate history
// 5. Score agreement on final round if scorer provided
func (d *Debate) Execute(ctx context.Context, opts *Options) (*Result, error) {
	if len(opts.Proposers) == 0 {
		return nil, fmt.Errorf("no proposers configured")
	}

	if opts.Rounds < 1 {
		opts.Rounds = 1 // Default to at least 1 round
	}

	start := time.Now()

	// Track all rounds for history
	var debateRounds []prompt.DebateRound
	var currentResponses []provider.Response

	// Round 1: Independent proposals
	round1Results := provider.QueryAll(ctx, opts.Proposers, opts.Request, opts.Progress)
	if len(round1Results.Responses) == 0 {
		return nil, fmt.Errorf("all proposers failed in round 1: %v", round1Results.Errors)
	}

	// Check minimum required successes (2 for debate to make sense)
	if len(round1Results.Responses) < 2 {
		return nil, fmt.Errorf("insufficient successful proposals: got %d, need at least 2", len(round1Results.Responses))
	}

	currentResponses = round1Results.Responses
	debateRounds = append(debateRounds, prompt.DebateRound{
		RoundNum:  1,
		Responses: currentResponses,
	})

	// Rounds 2 through N: Revision rounds
	for roundNum := 2; roundNum <= opts.Rounds; roundNum++ {
		// Check for cancellation before starting next round
		select {
		case <-ctx.Done():
			// Early cancellation - return what we have so far
			return buildDebateResult(ctx, debateRounds, opts, time.Since(start).Milliseconds())
		default:
		}

		// Build revision requests for each proposer
		var revisionProviders []provider.Provider
		var revisionRequests []*provider.Request

		for _, resp := range currentResponses {
			// Find the original provider for this response
			var originalProvider provider.Provider
			for _, p := range opts.Proposers {
				if p.Name() == resp.Name {
					originalProvider = p
					break
				}
			}
			if originalProvider == nil {
				continue // Skip if provider no longer available
			}

			// Build revision prompt: own response + others' responses
			otherResponses := prompt.FormatOtherResponses(currentResponses, resp.Name)
			revisionPrompt := strings.ReplaceAll(prompt.DebateRevisionTemplate, "{prompt}", opts.Request.UserPrompt)
			revisionPrompt = strings.ReplaceAll(revisionPrompt, "{own_response}", resp.Content)
			revisionPrompt = strings.ReplaceAll(revisionPrompt, "{other_responses}", otherResponses)

			revisionReq := &provider.Request{
				SystemPrompt: prompt.DebateRevisionSystem,
				UserPrompt:   revisionPrompt,
				Temperature:  opts.Request.Temperature,
				MaxTokens:    opts.Request.MaxTokens,
			}

			revisionProviders = append(revisionProviders, originalProvider)
			revisionRequests = append(revisionRequests, revisionReq)
		}

		// Query all revisions in parallel
		revisionResults := queryAllWithRequests(ctx, revisionProviders, revisionRequests, opts.Progress)

		// If we got fewer than 2 successes, stop debate early
		if len(revisionResults.Responses) < 2 {
			break
		}

		currentResponses = revisionResults.Responses
		debateRounds = append(debateRounds, prompt.DebateRound{
			RoundNum:  roundNum,
			Responses: currentResponses,
		})
	}

	return buildDebateResult(ctx, debateRounds, opts, time.Since(start).Milliseconds())
}

// buildDebateResult constructs the final result from debate rounds.
func buildDebateResult(ctx context.Context, debateRounds []prompt.DebateRound, opts *Options, totalMs int64) (*Result, error) {
	if len(debateRounds) == 0 {
		return nil, fmt.Errorf("no debate rounds completed")
	}

	// Get final round responses for proposals
	finalRound := debateRounds[len(debateRounds)-1]
	proposals := make([]*provider.Response, len(finalRound.Responses))
	for i := range finalRound.Responses {
		proposals[i] = &finalRound.Responses[i]
	}

	// Judge synthesizes from full debate history
	debateHistory := prompt.FormatDebateHistory(debateRounds)
	judgePrompt := strings.ReplaceAll(prompt.DebateJudgeTemplate, "{prompt}", opts.Request.UserPrompt)
	judgePrompt = strings.ReplaceAll(judgePrompt, "{debate_history}", debateHistory)

	judgeReq := &provider.Request{
		SystemPrompt: prompt.DebateJudgeSystem,
		UserPrompt:   judgePrompt,
		Temperature:  nil,
		MaxTokens:    4000,
	}

	synthesis, err := opts.Aggregator.Query(ctx, judgeReq)
	if err != nil {
		return nil, fmt.Errorf("judge failed: %w", err)
	}

	// Score agreement on final round if scorer provided
	var agreementScore *int
	var agreementReason string

	if opts.Scorer != nil {
		score, err := scoring.ScoreAgreement(ctx, opts.Scorer, finalRound.Responses, opts.Request.UserPrompt)
		if err != nil {
			agreementReason = fmt.Sprintf("scoring failed: %v", err)
		} else if score != nil {
			agreementScore = &score.Score
			agreementReason = score.Reason
		}
	}

	return &Result{
		Proposals:       proposals,
		Synthesis:       synthesis,
		Rounds:          len(debateRounds),
		AgreementScore:  agreementScore,
		AgreementReason: agreementReason,
		TotalMs:         totalMs,
	}, nil
}

// queryAllWithRequests queries multiple providers with individual requests in parallel.
func queryAllWithRequests(ctx context.Context, providers []provider.Provider, requests []*provider.Request, progress chan<- provider.ProgressEvent) *provider.QueryAllResult {
	if len(providers) != len(requests) {
		panic("providers and requests length mismatch")
	}

	type result struct {
		resp *provider.Response
		err  error
	}

	results := make([]result, len(providers))
	done := make(chan int, len(providers))

	// Query all in parallel with per-model timeouts
	for i := range providers {
		go func(idx int) {
			// Apply timeout matching QueryAll behavior
			queryCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()

			resp, err := providers[idx].Query(queryCtx, requests[idx])
			results[idx] = result{resp: resp, err: err}

			// Send progress event
			if progress != nil {
				event := provider.ProgressEvent{
					Model:  providers[idx].Name(),
					Status: "revision",
				}
				if err != nil {
					event.Status = "failed"
					event.Error = err
				} else {
					event.Status = "done"
					event.Latency = time.Duration(resp.LatencyMs) * time.Millisecond
				}
				select {
				case progress <- event:
				case <-ctx.Done():
				}
			}

			done <- idx
		}(i)
	}

	// Wait for all to complete
	for range providers {
		<-done
	}

	// Collect successful responses and errors
	var responses []provider.Response
	errors := make(map[string]error)

	for i, res := range results {
		if res.err != nil {
			errors[providers[i].Name()] = res.err
		} else if res.resp != nil {
			responses = append(responses, *res.resp)
		}
	}

	return &provider.QueryAllResult{
		Responses: responses,
		Errors:    errors,
	}
}
