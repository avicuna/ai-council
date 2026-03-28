package strategy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

func TestDebate_Execute_HappyPath(t *testing.T) {
	// Setup: 3 proposers, 3 rounds
	proposers := []provider.Provider{
		newMockProvider("Model1", "Round 1 from model 1"),
		newMockProvider("Model2", "Round 1 from model 2"),
		newMockProvider("Model3", "Round 1 from model 3"),
	}

	aggregator := newMockProvider("Judge", "Final synthesis")

	debate := NewDebate()
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
		Progress: nil,
		Rounds:   3,
	}

	result, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify result
	if result.Rounds != 3 {
		t.Errorf("Expected 3 rounds, got %d", result.Rounds)
	}

	if len(result.Proposals) != 3 {
		t.Errorf("Expected 3 proposals, got %d", len(result.Proposals))
	}

	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}

	if result.Synthesis.Content != "Final synthesis" {
		t.Errorf("Expected synthesis 'Final synthesis', got '%s'", result.Synthesis.Content)
	}
}

func TestDebate_Execute_SingleRound(t *testing.T) {
	// Setup: 2 proposers, 1 round (essentially same as MoA but with judge)
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
	}

	aggregator := newMockProvider("Judge", "Judged synthesis")

	debate := NewDebate()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Rounds: 1,
	}

	result, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Rounds != 1 {
		t.Errorf("Expected 1 round, got %d", result.Rounds)
	}

	if len(result.Proposals) != 2 {
		t.Errorf("Expected 2 proposals, got %d", len(result.Proposals))
	}
}

func TestDebate_Execute_RevisionPromptsIncludeOthers(t *testing.T) {
	// Use a mock that tracks prompts to verify revision includes others' responses
	model1 := &trackingMockProvider{
		name: "Model1",
		response: &provider.Response{
			Content:      "Response from model 1",
			Model:        "Model1",
			Name:         "Model1",
			InputTokens:  100,
			OutputTokens: 200,
			LatencyMs:    50,
		},
		available: true,
	}

	model2 := &trackingMockProvider{
		name: "Model2",
		response: &provider.Response{
			Content:      "Response from model 2",
			Model:        "Model2",
			Name:         "Model2",
			InputTokens:  100,
			OutputTokens: 200,
			LatencyMs:    50,
		},
		available: true,
	}

	proposers := []provider.Provider{model1, model2}
	aggregator := newMockProvider("Judge", "Final")

	debate := NewDebate()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "What is AI?",
			MaxTokens:  1000,
		},
		Rounds: 2,
	}

	_, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify that round 2 prompts include other models' responses
	// Each model should have been called twice: round 1 + revision
	if len(model1.requests) < 2 {
		t.Fatalf("Model1 should have been called at least twice, got %d", len(model1.requests))
	}

	if len(model2.requests) < 2 {
		t.Fatalf("Model2 should have been called at least twice, got %d", len(model2.requests))
	}

	// Check that revision prompts include others' responses
	// Model1's revision should include Model2's response
	model1RevisionPrompt := model1.requests[1].UserPrompt
	if !strings.Contains(model1RevisionPrompt, "Model2") {
		t.Errorf("Model1's revision prompt should mention Model2, got: %s", model1RevisionPrompt)
	}

	// Model2's revision should include Model1's response
	model2RevisionPrompt := model2.requests[1].UserPrompt
	if !strings.Contains(model2RevisionPrompt, "Model1") {
		t.Errorf("Model2's revision prompt should mention Model1, got: %s", model2RevisionPrompt)
	}
}

func TestDebate_Execute_EarlyStopOnInsufficientSuccesses(t *testing.T) {
	// Setup: 3 proposers, but 2 fail in round 1
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newFailingMockProvider("Model2", errors.New("error 2")),
		newFailingMockProvider("Model3", errors.New("error 3")),
	}

	aggregator := newMockProvider("Judge", "Final")

	debate := NewDebate()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Rounds: 3,
	}

	result, err := debate.Execute(ctx, opts)
	// Should fail because only 1 success (need at least 2)
	if err == nil {
		t.Fatal("Expected error with only 1 success, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}

	expectedMsg := "insufficient successful proposals: got 1, need at least 2"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%v'", expectedMsg, err)
	}
}

func TestDebate_Execute_EarlyStopBetweenRounds(t *testing.T) {
	// Setup: 3 proposers start, but one fails in round 2
	model1 := &trackingMockProvider{
		name: "Model1",
		response: &provider.Response{
			Content: "Response 1", Model: "Model1", Name: "Model1",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
	}

	model2 := &trackingMockProvider{
		name: "Model2",
		response: &provider.Response{
			Content: "Response 2", Model: "Model2", Name: "Model2",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
		failAfterCall: 1, // Fail after first call (round 1)
	}

	model3 := &trackingMockProvider{
		name: "Model3",
		response: &provider.Response{
			Content: "Response 3", Model: "Model3", Name: "Model3",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
		failAfterCall: 1, // Fail after first call (round 1)
	}

	proposers := []provider.Provider{model1, model2, model3}
	aggregator := newMockProvider("Judge", "Final")

	debate := NewDebate()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Rounds: 3,
	}

	result, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute should succeed with partial rounds, got error: %v", err)
	}

	// Should stop after round 1 because only 1 provider succeeds in round 2
	if result.Rounds != 1 {
		t.Errorf("Expected debate to stop at round 1, got %d rounds", result.Rounds)
	}
}

func TestDebate_Execute_ContextCancellation(t *testing.T) {
	// Setup: context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())

	model1 := &trackingMockProvider{
		name: "Model1",
		response: &provider.Response{
			Content: "Response 1", Model: "Model1", Name: "Model1",
			InputTokens: 100, OutputTokens: 200, LatencyMs: 50,
		},
		available: true,
		onQuery: func() {
			// Cancel context after first round
			cancel()
		},
	}

	model2 := newMockProvider("Model2", "Response 2")

	proposers := []provider.Provider{model1, model2}
	aggregator := newMockProvider("Judge", "Final")

	debate := NewDebate()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Rounds: 5, // Request many rounds but should stop early
	}

	result, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should have completed at least 1 round before cancellation
	if result.Rounds < 1 {
		t.Errorf("Expected at least 1 round, got %d", result.Rounds)
	}

	// Should not have completed all 5 rounds due to cancellation
	if result.Rounds >= 5 {
		t.Errorf("Expected cancellation to stop debate early, but got %d rounds", result.Rounds)
	}
}

func TestDebate_Execute_WithScorer(t *testing.T) {
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
	}

	aggregator := newMockProvider("Judge", "Final")
	scorer := newMockProvider("Scorer", `{"score": 90, "reason": "Strong agreement"}`)

	debate := NewDebate()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer,
		Request: &provider.Request{
			UserPrompt: "Test",
			MaxTokens:  1000,
		},
		Rounds: 2,
	}

	result, err := debate.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.AgreementScore == nil {
		t.Fatal("Expected agreement score, got nil")
	}

	if *result.AgreementScore != 90 {
		t.Errorf("Expected score 90, got %d", *result.AgreementScore)
	}
}

// trackingMockProvider records all requests for verification.
type trackingMockProvider struct {
	name          string
	response      *provider.Response
	err           error
	available     bool
	requests      []*provider.Request
	failAfterCall int // Fail after N calls (0 = never fail)
	onQuery       func()
}

func (m *trackingMockProvider) Name() string {
	return m.name
}

func (m *trackingMockProvider) Query(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if m.onQuery != nil {
		m.onQuery()
	}

	m.requests = append(m.requests, req)

	if m.failAfterCall > 0 && len(m.requests) > m.failAfterCall {
		return nil, errors.New("failed after call limit")
	}

	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *trackingMockProvider) Available() bool {
	return m.available
}
