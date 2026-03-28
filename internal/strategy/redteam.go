package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/avicuna/ai-council-personal/internal/prompt"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/scoring"
)

// RedTeam implements an adversarial red team strategy.
// One provider is selected as the attacker to critique others' proposals.
// The flow is: proposals → critique → defenses → targeted attack → judge synthesis.
type RedTeam struct{}

// NewRedTeam creates a new RedTeam strategy instance.
func NewRedTeam() *RedTeam {
	return &RedTeam{}
}

// Execute runs the red team strategy:
// 1. If <2 providers: fall back to MoA
// 2. Select attacker deterministically using sha256(prompt) % len(proposers)
// 3. Defenders propose initial answers
// 4. Attacker critiques proposals
// 5. Defenders revise and defend
// 6. Attacker targets defenses
// 7. Judge synthesizes hardened answer
// 8. Score agreement on defenses
func (r *RedTeam) Execute(ctx context.Context, opts *Options) (*Result, error) {
	if len(opts.Proposers) == 0 {
		return nil, fmt.Errorf("no proposers configured")
	}

	// Fall back to MoA if fewer than 2 providers (need attacker + at least 1 defender)
	if len(opts.Proposers) < 2 {
		moa := NewMoA()
		return moa.Execute(ctx, opts)
	}

	start := time.Now()

	// Select attacker deterministically based on prompt
	attackerIdx := selectAttacker(opts.Request.UserPrompt, len(opts.Proposers))
	attacker := opts.Proposers[attackerIdx]

	// Separate defenders from attacker
	var defenders []provider.Provider
	for i, p := range opts.Proposers {
		if i != attackerIdx {
			defenders = append(defenders, p)
		}
	}

	// Round 1: Defenders propose initial answers
	proposalResults := provider.QueryAll(ctx, defenders, opts.Request, opts.Progress)
	if len(proposalResults.Responses) == 0 {
		return nil, fmt.Errorf("all defenders failed: %v", proposalResults.Errors)
	}

	initialProposals := proposalResults.Responses

	// Attacker critiques proposals
	attackPrompt := buildAttackPrompt(opts.Request.UserPrompt, initialProposals)
	attackResp, err := attacker.Query(ctx, attackPrompt)
	if err != nil {
		return nil, fmt.Errorf("attacker failed: %w", err)
	}

	// Send progress event for attacker
	if opts.Progress != nil {
		select {
		case opts.Progress <- provider.ProgressEvent{
			Model:   attacker.Name(),
			Status:  "critique",
			Latency: time.Duration(attackResp.LatencyMs) * time.Millisecond,
		}:
		case <-ctx.Done():
		}
	}

	// Round 2: Defenders revise addressing the attack
	var defenseProviders []provider.Provider
	var defenseRequests []*provider.Request

	for _, proposal := range initialProposals {
		// Find the original defender
		var defender provider.Provider
		for _, d := range defenders {
			if d.Name() == proposal.Name {
				defender = d
				break
			}
		}
		if defender == nil {
			continue
		}

		defenseReq := buildDefenseRequest(opts.Request.UserPrompt, proposal.Content, attackResp.Content)
		defenseProviders = append(defenseProviders, defender)
		defenseRequests = append(defenseRequests, defenseReq)
	}

	defenseResults := queryAllWithRequests(ctx, defenseProviders, defenseRequests, opts.Progress)
	if len(defenseResults.Responses) == 0 {
		return nil, fmt.Errorf("all defenders failed to defend: %v", defenseResults.Errors)
	}

	defenses := defenseResults.Responses

	// Attacker targets defenses
	targetedAttackPrompt := buildTargetedAttackPrompt(opts.Request.UserPrompt, initialProposals, attackResp.Content, defenses)
	targetedAttackResp, err := attacker.Query(ctx, targetedAttackPrompt)
	if err != nil {
		// Non-fatal: proceed without targeted attack
		targetedAttackResp = &provider.Response{
			Content: "(targeted attack failed)",
		}
	}

	// Send progress event for targeted attack
	if opts.Progress != nil {
		select {
		case opts.Progress <- provider.ProgressEvent{
			Model:   attacker.Name(),
			Status:  "targeted_critique",
			Latency: time.Duration(targetedAttackResp.LatencyMs) * time.Millisecond,
		}:
		case <-ctx.Done():
		}
	}

	// Judge synthesizes hardened answer
	judgePrompt := buildJudgePrompt(opts.Request.UserPrompt, initialProposals, attackResp.Content, defenses, targetedAttackResp.Content)
	synthesis, err := opts.Aggregator.Query(ctx, judgePrompt)
	if err != nil {
		return nil, fmt.Errorf("judge failed: %w", err)
	}

	// Convert defenses to pointer slice for Result
	proposals := make([]*provider.Response, len(defenses))
	for i := range defenses {
		proposals[i] = &defenses[i]
	}

	// Score agreement on defenses if scorer provided
	var agreementScore *int
	var agreementReason string

	if opts.Scorer != nil {
		score, err := scoring.ScoreAgreement(ctx, opts.Scorer, defenses, opts.Request.UserPrompt)
		if err != nil {
			agreementReason = fmt.Sprintf("scoring failed: %v", err)
		} else if score != nil {
			agreementScore = &score.Score
			agreementReason = score.Reason
		}
	}

	totalMs := time.Since(start).Milliseconds()
	return &Result{
		Proposals:       proposals,
		Synthesis:       synthesis,
		Rounds:          2, // Red team is always 2 rounds (proposals + defenses)
		AgreementScore:  agreementScore,
		AgreementReason: agreementReason,
		TotalMs:         totalMs,
	}, nil
}

// selectAttacker selects an attacker index deterministically based on prompt.
func selectAttacker(prompt string, numProviders int) int {
	hash := sha256.Sum256([]byte(prompt))
	// Use first 8 bytes as uint64
	hashVal := binary.BigEndian.Uint64(hash[:8])
	return int(hashVal % uint64(numProviders))
}

// buildAttackPrompt constructs the attacker's critique prompt.
func buildAttackPrompt(userPrompt string, proposals []provider.Response) *provider.Request {
	proposalsText := prompt.FormatProposals(proposals)
	attackPrompt := strings.ReplaceAll(prompt.RedTeamAttackerTemplate, "{prompt}", userPrompt)
	attackPrompt = strings.ReplaceAll(attackPrompt, "{proposals}", proposalsText)

	return &provider.Request{
		SystemPrompt: prompt.RedTeamAttackerSystem,
		UserPrompt:   attackPrompt,
		Temperature:  nil,
		MaxTokens:    2000,
	}
}

// buildDefenseRequest constructs a defender's defense prompt.
func buildDefenseRequest(userPrompt, ownResponse, attack string) *provider.Request {
	defensePrompt := strings.ReplaceAll(prompt.RedTeamDefenseTemplate, "{prompt}", userPrompt)
	defensePrompt = strings.ReplaceAll(defensePrompt, "{own_response}", ownResponse)
	defensePrompt = strings.ReplaceAll(defensePrompt, "{attack}", attack)

	return &provider.Request{
		SystemPrompt: prompt.RedTeamDefenseSystem,
		UserPrompt:   defensePrompt,
		Temperature:  nil,
		MaxTokens:    3000,
	}
}

// buildTargetedAttackPrompt constructs the attacker's targeted critique of defenses.
func buildTargetedAttackPrompt(userPrompt string, proposals []provider.Response, initialAttack string, defenses []provider.Response) *provider.Request {
	proposalsText := prompt.FormatProposals(proposals)
	defensesText := prompt.FormatProposals(defenses)

	// Use the defense template but adapt it for targeted attack
	targetedPrompt := fmt.Sprintf("Original question: %s\n\nOriginal proposals:\n%s\n\nYour initial critique:\n%s\n\nDefenses:\n%s\n\nProvide a targeted critique addressing the defenses.",
		userPrompt, proposalsText, initialAttack, defensesText)

	return &provider.Request{
		SystemPrompt: prompt.RedTeamAttackerSystem,
		UserPrompt:   targetedPrompt,
		Temperature:  nil,
		MaxTokens:    2000,
	}
}

// buildJudgePrompt constructs the judge's synthesis prompt.
func buildJudgePrompt(userPrompt string, proposals []provider.Response, attack string, defenses []provider.Response, targetedAttack string) *provider.Request {
	proposalsText := prompt.FormatProposals(proposals)
	defensesText := prompt.FormatProposals(defenses)

	judgePrompt := strings.ReplaceAll(prompt.RedTeamJudgeTemplate, "{prompt}", userPrompt)
	judgePrompt = strings.ReplaceAll(judgePrompt, "{proposals}", proposalsText)
	judgePrompt = strings.ReplaceAll(judgePrompt, "{attack}", attack)
	judgePrompt = strings.ReplaceAll(judgePrompt, "{defenses}", defensesText)
	judgePrompt = strings.ReplaceAll(judgePrompt, "{targeted_attack}", targetedAttack)

	return &provider.Request{
		SystemPrompt: prompt.RedTeamJudgeSystem,
		UserPrompt:   judgePrompt,
		Temperature:  nil,
		MaxTokens:    4000,
	}
}
