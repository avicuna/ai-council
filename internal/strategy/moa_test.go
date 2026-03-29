package strategy

import (
	"context"
	"errors"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

// mockProvider is a test double for provider.Provider
type mockProvider struct {
	name      string
	response  *provider.Response
	err       error
	available bool
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Query(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProvider) Available() bool {
	return m.available
}

// newMockProvider creates a successful mock provider
func newMockProvider(name, content string) *mockProvider {
	return &mockProvider{
		name: name,
		response: &provider.Response{
			Content:      content,
			Model:        name,
			Name:         name,
			InputTokens:  100,
			OutputTokens: 200,
			LatencyMs:    50,
		},
		available: true,
	}
}

// newFailingMockProvider creates a mock provider that returns an error
func newFailingMockProvider(name string, err error) *mockProvider {
	return &mockProvider{
		name:      name,
		err:       err,
		available: true,
	}
}

func TestMoA_Execute_HappyPath(t *testing.T) {
	// Setup: 3 successful proposers
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response from model 1"),
		newMockProvider("Model2", "Response from model 2"),
		newMockProvider("Model3", "Response from model 3"),
	}

	aggregator := newMockProvider("Aggregator", "Synthesized response")

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     nil, // No scoring for this test
		Request: &provider.Request{
			SystemPrompt: "System",
			UserPrompt:   "What is AI?",
			MaxTokens:    1000,
		},
		Progress:   nil,
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify result
	if len(result.Proposals) != 3 {
		t.Errorf("Expected 3 proposals, got %d", len(result.Proposals))
	}

	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}

	if result.Synthesis.Content != "Synthesized response" {
		t.Errorf("Expected synthesis content 'Synthesized response', got '%s'", result.Synthesis.Content)
	}

	if result.Rounds != 1 {
		t.Errorf("Expected 1 round, got %d", result.Rounds)
	}

	if result.AgreementScore != nil {
		t.Errorf("Expected no agreement score (scorer was nil), got %v", *result.AgreementScore)
	}

	if result.TotalMs < 0 {
		t.Errorf("Expected non-negative total latency, got %d", result.TotalMs)
	}
}

func TestMoA_Execute_PartialFailure(t *testing.T) {
	// Setup: 1 fails, 2 succeed → should still synthesize
	proposers := []provider.Provider{
		newFailingMockProvider("Model1", errors.New("API error")),
		newMockProvider("Model2", "Response from model 2"),
		newMockProvider("Model3", "Response from model 3"),
	}

	aggregator := newMockProvider("Aggregator", "Synthesized response")

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute should succeed with 2 successful proposals, got error: %v", err)
	}

	// Verify only successful proposals are included
	if len(result.Proposals) != 2 {
		t.Errorf("Expected 2 successful proposals, got %d", len(result.Proposals))
	}

	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}
}

func TestMoA_Execute_AllFail(t *testing.T) {
	// Setup: all proposers fail
	proposers := []provider.Provider{
		newFailingMockProvider("Model1", errors.New("error 1")),
		newFailingMockProvider("Model2", errors.New("error 2")),
		newFailingMockProvider("Model3", errors.New("error 3")),
	}

	aggregator := newMockProvider("Aggregator", "Should not be called")

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error when all proposers fail, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result on failure, got %+v", result)
	}

	// Verify error message mentions all failures
	if err.Error() != "all proposers failed: map[Model1:error 1 Model2:error 2 Model3:error 3]" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMoA_Execute_InsufficientSuccesses(t *testing.T) {
	// Setup: only 1 succeeds (need at least 2 for non-fast tier)
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response from model 1"),
		newFailingMockProvider("Model2", errors.New("error 2")),
		newFailingMockProvider("Model3", errors.New("error 3")),
	}

	aggregator := newMockProvider("Aggregator", "Should not be called")

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error with only 1 success (need 2), got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}

	expectedMsg := "insufficient successful proposals: got 1, need at least 2"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%v'", expectedMsg, err)
	}
}

func TestMoA_Execute_FastTierSkipsAggregation(t *testing.T) {
	// Setup: fast tier with 1 proposer
	proposers := []provider.Provider{
		newMockProvider("FastModel", "Fast response"),
	}

	// Aggregator that would fail if called
	aggregator := newFailingMockProvider("Aggregator", errors.New("should not be called"))

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: true, // Fast tier enabled
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Fast tier execution failed: %v", err)
	}

	// Verify synthesis is the first proposal directly (no aggregation)
	if result.Synthesis == nil {
		t.Fatal("Expected synthesis, got nil")
	}

	if result.Synthesis.Content != "Fast response" {
		t.Errorf("Expected synthesis to be first proposal, got '%s'", result.Synthesis.Content)
	}

	// Verify aggregator was NOT called (would have failed)
	if len(result.Proposals) != 1 {
		t.Errorf("Expected 1 proposal, got %d", len(result.Proposals))
	}
}

func TestMoA_Execute_FastTierWithMultipleProposers(t *testing.T) {
	// Setup: fast tier with multiple proposers (still skips aggregation)
	proposers := []provider.Provider{
		newMockProvider("Fast1", "Response 1"),
		newMockProvider("Fast2", "Response 2"),
		newMockProvider("Fast3", "Response 3"),
	}

	aggregator := newFailingMockProvider("Aggregator", errors.New("should not be called"))

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: true,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Fast tier execution failed: %v", err)
	}

	// Verify all proposals are collected
	if len(result.Proposals) != 3 {
		t.Errorf("Expected 3 proposals, got %d", len(result.Proposals))
	}

	// Verify synthesis is one of the successful responses (order is non-deterministic in parallel execution)
	validContents := map[string]bool{
		"Response 1": true,
		"Response 2": true,
		"Response 3": true,
	}
	if !validContents[result.Synthesis.Content] {
		t.Errorf("Expected synthesis to be one of the proposals, got '%s'", result.Synthesis.Content)
	}
}

func TestMoA_Execute_WithScorer(t *testing.T) {
	// Setup: 3 proposers + scorer
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
		newMockProvider("Model3", "Response 3"),
	}

	aggregator := newMockProvider("Aggregator", "Synthesized")
	scorer := newMockProvider("Scorer", `{"score": 85, "reason": "High agreement"}`)

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer, // Scoring enabled
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify agreement score is set
	if result.AgreementScore == nil {
		t.Fatal("Expected agreement score, got nil")
	}

	if *result.AgreementScore != 85 {
		t.Errorf("Expected score 85, got %d", *result.AgreementScore)
	}

	if result.AgreementReason != "High agreement" {
		t.Errorf("Expected reason 'High agreement', got '%s'", result.AgreementReason)
	}
}

func TestMoA_Execute_ScorerFailure(t *testing.T) {
	// Setup: scorer fails (should not fail entire strategy)
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
	}

	aggregator := newMockProvider("Aggregator", "Synthesized")
	scorer := newFailingMockProvider("Scorer", errors.New("scoring API error"))

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Scorer:     scorer,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute should succeed despite scorer failure, got error: %v", err)
	}

	// Verify scoring failure is captured in reason
	if result.AgreementScore != nil {
		t.Errorf("Expected nil score on scorer failure, got %v", *result.AgreementScore)
	}

	if result.AgreementReason == "" {
		t.Error("Expected agreement reason to contain error message")
	}
}

func TestMoA_Execute_AggregatorFailure(t *testing.T) {
	// Setup: aggregator fails
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
	}

	aggregator := newFailingMockProvider("Aggregator", errors.New("aggregator API error"))

	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error when aggregator fails, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result on aggregator failure, got %+v", result)
	}

	expectedMsg := "aggregator failed: aggregator API error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%v'", expectedMsg, err)
	}
}

func TestMoA_Execute_NoProposers(t *testing.T) {
	moa := NewMoA()
	ctx := context.Background()

	opts := &Options{
		Proposers:  []provider.Provider{}, // Empty
		Aggregator: newMockProvider("Aggregator", "Should not be called"),
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err == nil {
		t.Fatal("Expected error with no proposers, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %+v", result)
	}

	expectedMsg := "no proposers configured"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%v'", expectedMsg, err)
	}
}

func TestMoA_Execute_ProgressEvents(t *testing.T) {
	// Setup: 2 proposers with progress channel
	proposers := []provider.Provider{
		newMockProvider("Model1", "Response 1"),
		newMockProvider("Model2", "Response 2"),
	}

	aggregator := newMockProvider("Aggregator", "Synthesized")

	moa := NewMoA()
	ctx := context.Background()

	// Create progress channel
	progressCh := make(chan provider.ProgressEvent, 10)

	opts := &Options{
		Proposers:  proposers,
		Aggregator: aggregator,
		Request: &provider.Request{
			UserPrompt: "Test prompt",
			MaxTokens:  1000,
		},
		Progress:   progressCh,
		IsFastTier: false,
	}

	result, err := moa.Execute(ctx, opts)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	close(progressCh)

	// Collect progress events
	var events []provider.ProgressEvent
	for event := range progressCh {
		events = append(events, event)
	}

	// Should have events for both proposers (querying + done)
	if len(events) < 2 {
		t.Errorf("Expected at least 2 progress events, got %d", len(events))
	}

	// Verify result is still correct
	if len(result.Proposals) != 2 {
		t.Errorf("Expected 2 proposals, got %d", len(result.Proposals))
	}
}
