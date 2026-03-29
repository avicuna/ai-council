package strategy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

func TestRedTeam_Execute_HappyPath(t *testing.T) {
	// Setup: 3 proposers (1 attacker, 2 defenders)
	proposers := []provider.Provider{
		newMockProvider("Model1", "Proposal from model 1"),
		newMockProvider("Model2", "Proposal from model 2"),
		newMockProvider("Model3", "Critique or defense"),
	}

	aggregator := newMockProvider("Judge", "Hardened synthesis")

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     nil,
		Request: &provider.Request{
			SystemPrompt: "System",
			UserPrompt:   "What is AI?",
			MaxTokens:    1000,
		},
	}

	result, err := redteam.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify result
	if result.Rounds != 2 {
		t.Errorf("Expected 2 rounds (proposals + defenses), got %d", result.Rounds)
	}

	// Proposals should be the defenses (final round)
	if len(result.Proposals) < 1 {
		t.Errorf("Expected at least 1 defense proposal, got %d", len(result.Proposals))
	}

	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}

	if result.Synthesis.Content != "Hardened synthesis" {
		t.Errorf("Expected synthesis 'Hardened synthesis', got '%s'", result.Synthesis.Content)
	}
}

func TestRedTeam_Execute_AttackerSelectionDeterministic(t *testing.T) {
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
		newMockProvider("Model3", "Response 3"),
	}

	prompt1 := "What is AI?"
	prompt2 := "Different prompt"

	// Same prompt should always select same attacker
	idx1a := selectAttacker(prompt1, len(proposers))
	idx1b := selectAttacker(prompt1, len(proposers))

	if idx1a != idx1b {
		t.Errorf("Same prompt should select same attacker: %d vs %d", idx1a, idx1b)
	}

	// Different prompts likely select different attackers (not guaranteed but highly likely)
	idx2 := selectAttacker(prompt2, len(proposers))

	// Verify indices are in valid range
	if idx1a < 0 || idx1a >= len(proposers) {
		t.Errorf("Attacker index out of range: %d", idx1a)
	}
	if idx2 < 0 || idx2 >= len(proposers) {
		t.Errorf("Attacker index out of range: %d", idx2)
	}

	// Just log for inspection (can't assert they're different)
	t.Logf("Prompt '%s' selects attacker index %d", prompt1, idx1a)
	t.Logf("Prompt '%s' selects attacker index %d", prompt2, idx2)
}

func TestRedTeam_Execute_AttackerGetsDifferentPrompt(t *testing.T) {
	// Use tracking mocks to verify attacker gets different prompt
	model1 := &trackingMockProvider{
		name: "Model1",
		response: &provider.Response{
			Content: "Proposal 1", Model: "Model1", Name: "Model1",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
	}

	model2 := &trackingMockProvider{
		name: "Model2",
		response: &provider.Response{
			Content: "Proposal 2", Model: "Model2", Name: "Model2",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
	}

	model3 := &trackingMockProvider{
		name: "Model3",
		response: &provider.Response{
			Content: "Critique", Model: "Model3", Name: "Model3",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
	}

	proposers := []provider.Provider{model1, model2, model3}
	aggregator := newMockProvider("Judge", "Final")

	redteam := NewRedTeam()
	ctx := context.Background()

	userPrompt := "What is AI?"
	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: userPrompt,
			MaxTokens:  1000,
		},
	}

	_, err := redteam.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Determine which model is the attacker
	attackerIdx := selectAttacker(userPrompt, len(proposers))
	attackers := []*trackingMockProvider{model1, model2, model3}
	attacker := attackers[attackerIdx]

	// Attacker should have been called (for attack and targeted attack)
	if len(attacker.requests) < 1 {
		t.Fatal("Attacker should have been called at least once")
	}

	// Attacker's first request should be critique, not original prompt
	attackerFirstPrompt := attacker.requests[0].UserPrompt
	if !strings.Contains(attackerFirstPrompt, "Critique") && !strings.Contains(attackerFirstPrompt, "critique") {
		t.Errorf("Attacker's prompt should contain 'critique', got: %s", attackerFirstPrompt)
	}

	// Defenders should get the original prompt in round 1
	defenders := make([]*trackingMockProvider, 0)
	for i, m := range attackers {
		if i != attackerIdx {
			defenders = append(defenders, m)
		}
	}

	for _, defender := range defenders {
		if len(defender.requests) < 1 {
			t.Errorf("Defender %s should have been called", defender.name)
			continue
		}
		// First request should be original prompt or defense prompt
		// After attacker, defenders should see defense prompts
		if len(defender.requests) > 1 {
			defensePrompt := defender.requests[1].UserPrompt
			if !strings.Contains(defensePrompt, "critique") && !strings.Contains(defensePrompt, "Critique") {
				t.Errorf("Defender's defense prompt should mention critique, got: %s", defensePrompt)
			}
		}
	}
}

func TestRedTeam_Execute_FallbackToMoAWithOneProvider(t *testing.T) {
	// Setup: only 1 provider (should fall back to MoA)
	proposers := []provider.Provider{
		newMockProvider("OnlyModel", "Only response"),
	}

	// Aggregator that would fail if used in normal red team flow
	aggregator := newMockProvider("Aggregator", "MoA synthesis")

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := redteam.Execute(ctx, opts)
	// Should fail because MoA needs at least 2 proposers for non-fast tier
	if err == nil {
		t.Fatal("Expected error with 1 provider in non-fast tier, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}
}

func TestRedTeam_Execute_FallbackToMoAWithOneProviderFastTier(t *testing.T) {
	// Setup: only 1 provider with fast tier (should fall back to MoA successfully)
	proposers := []provider.Provider{
		newMockProvider("OnlyModel", "Only response"),
	}

	aggregator := newFailingMockProvider("Aggregator", errors.New("should not be called"))

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		IsFastTier: true, // Fast tier allows 1 provider
	}

	result, err := redteam.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute should succeed via MoA fallback, got error: %v", err)
	}

	// Should have used MoA (1 round, synthesis is first response)
	if result.Rounds != 1 {
		t.Errorf("Expected 1 round (MoA fallback), got %d", result.Rounds)
	}

	if result.Synthesis.Content != "Only response" {
		t.Errorf("Expected MoA fast tier synthesis, got '%s'", result.Synthesis.Content)
	}
}

func TestRedTeam_Execute_AllDefendersFail(t *testing.T) {
	// Setup: attacker succeeds but all defenders fail
	proposers := []provider.Provider{
		newFailingMockProvider("Defender1", errors.New("error 1")),
		newFailingMockProvider("Defender2", errors.New("error 2")),
		newMockProvider("Attacker", "Critique"),
	}

	aggregator := newMockProvider("Judge", "Should not reach here")

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
	}

	result, err := redteam.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error when all defenders fail, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}

	if !strings.Contains(err.Error(), "all defenders failed") {
		t.Errorf("Expected error about defenders failing, got: %v", err)
	}
}

func TestRedTeam_Execute_AttackerFails(t *testing.T) {
	// Setup: defenders succeed but attacker fails
	// Determine which position will be the attacker for this prompt
	testPrompt := "Test prompt"
	attackerIdx := selectAttacker(testPrompt, 3)

	// Create providers with the attacker at the calculated position
	proposers := make([]provider.Provider, 3)
	for i := 0; i < 3; i++ {
		if i == attackerIdx {
			proposers[i] = newFailingMockProvider("Attacker", errors.New("attacker error"))
		} else {
			proposers[i] = newMockProvider("Defender", "Proposal")
		}
	}

	aggregator := newMockProvider("Judge", "Should not reach here")

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: testPrompt,
			MaxTokens:  1000,
		},
	}

	result, err := redteam.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error when attacker fails, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}

	if !strings.Contains(err.Error(), "attacker failed") {
		t.Errorf("Expected error about attacker failing, got: %v", err)
	}
}

func TestRedTeam_Execute_WithScorer(t *testing.T) {
	// Need 3 providers so we have 2 defenders (scorer needs at least 2 responses)
	proposers := []provider.Provider{
		newMockProvider("Model1", "Proposal 1"),
		newMockProvider("Model2", "Proposal 2"),
		newMockProvider("Model3", "Proposal 3"),
	}

	aggregator := newMockProvider("Judge", "Final")
	scorer := newMockProvider("Scorer", `{"score": 75, "reason": "Good agreement after defense"}`)

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
	}

	result, err := redteam.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.AgreementScore == nil {
		t.Fatal("Expected agreement score, got nil")
	}

	if *result.AgreementScore != 75 {
		t.Errorf("Expected score 75, got %d", *result.AgreementScore)
	}

	if !strings.Contains(result.AgreementReason, "agreement") {
		t.Errorf("Expected reason to contain 'agreement', got '%s'", result.AgreementReason)
	}
}

func TestRedTeam_Execute_TargetedAttackFailsNonFatal(t *testing.T) {
	// Setup: attacker succeeds first time but fails on targeted attack
	attacker := &trackingMockProvider{
		name: "Attacker",
		response: &provider.Response{
			Content: "Initial critique", Model: "Attacker", Name: "Attacker",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available:     true,
		failAfterCall: 1, // Fail on second call (targeted attack)
	}

	defender := newMockProvider("Defender", "Proposal and defense")

	// Position attacker at index 0
	testPrompt := "test"
	for i := 0; i < 100; i++ {
		testPrompt = string(rune('a' + i))
		if selectAttacker(testPrompt, 2) == 0 {
			break
		}
	}

	proposers := []provider.Provider{attacker, defender}
	aggregator := newMockProvider("Judge", "Final synthesis")

	redteam := NewRedTeam()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: testPrompt,
			MaxTokens:  1000,
		},
	}

	result, err := redteam.Execute(ctx, opts)
	// Should succeed despite targeted attack failure (non-fatal)
	if err != nil {
		t.Fatalf("Execute should succeed despite targeted attack failure, got error: %v", err)
	}

	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}

	// Verify we got a result despite targeted attack failing
	if result.Rounds != 2 {
		t.Errorf("Expected 2 rounds, got %d", result.Rounds)
	}
}

func TestRedTeam_Execute_ProgressEvents(t *testing.T) {
	proposers := []provider.Provider{
		newMockProvider("Model1", "Proposal 1"),
		newMockProvider("Model2", "Proposal 2"),
	}

	aggregator := newMockProvider("Judge", "Final")

	redteam := NewRedTeam()
	ctx := context.Background()

	progressCh := make(chan provider.ProgressEvent, 20)

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Progress: progressCh,
	}

	result, err := redteam.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	close(progressCh)

	// Collect events
	var events []provider.ProgressEvent
	for event := range progressCh {
		events = append(events, event)
	}

	// Should have events for:
	// - Defenders' initial proposals
	// - Attacker's critique
	// - Defenders' defenses
	// - Attacker's targeted attack
	if len(events) < 3 {
		t.Errorf("Expected at least 3 progress events, got %d", len(events))
	}

	// Verify result
	if result.Rounds != 2 {
		t.Errorf("Expected 2 rounds, got %d", result.Rounds)
	}
}
